package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// WebSocket API endpoints
	feishuWebSocketAPI = "/open-apis/v3/bot/websocket/"

	// WebSocket 配置
	wsPingInterval     = 30 * time.Second // 心跳间隔
	wsPongWait         = 60 * time.Second // 等待 pong 响应的超时时间
	wsReconnectDelay   = 5 * time.Second  // 重连延迟
	wsMaxReconnectTries = 10              // 最大重连尝试次数

	// WebSocket 消息类型
	wsMessageTypePing = "ping"
	wsMessageTypePong = "pong"
	wsMessageTypeError = "error"
)

// WebSocketClient 封装飞书 WebSocket 长连接客户端
type WebSocketClient struct {
	appID     string
	appSecret string
	logger    *slog.Logger
	httpClient *http.Client

	// WebSocket 连接
	conn      *websocket.Conn
	connMu    sync.Mutex
	connected bool

	// 上下文和取消
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// 事件处理器
	eventHandler func(event *Event)

	// 连接状态回调
	onConnect    func()
	onDisconnect func(error)
}

// WebSocketResponse 获取 WebSocket 连接地址的响应
type WebSocketResponse struct {
	Code int `json:"code"`
	Msg  string `json:"msg"`
	Data *WebSocketData `json:"data"`
}

// WebSocketData WebSocket 连接数据
type WebSocketData struct {
	URL string `json:"url"`
}

// WebSocketMessage WebSocket 消息格式
type WebSocketMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// NewWebSocketClient 创建新的 WebSocket 客户端
func NewWebSocketClient(appID, appSecret string, logger *slog.Logger) *WebSocketClient {
	if logger == nil {
		logger = slog.Default()
	}

	return &WebSocketClient{
		appID:     appID,
		appSecret: appSecret,
		logger:    logger,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetEventHandler 设置事件处理回调
func (c *WebSocketClient) SetEventHandler(handler func(event *Event)) {
	c.eventHandler = handler
}

// SetOnConnect 设置连接成功回调
func (c *WebSocketClient) SetOnConnect(callback func()) {
	c.onConnect = callback
}

// SetOnDisconnect 设置断开连接回调
func (c *WebSocketClient) SetOnDisconnect(callback func(error)) {
	c.onDisconnect = callback
}

// Connect 建立 WebSocket 连接
func (c *WebSocketClient) Connect(ctx context.Context) error {
	c.ctx, c.cancel = context.WithCancel(ctx)

	// 获取 WebSocket 连接地址
	wsURL, err := c.getWebSocketURL(ctx)
	if err != nil {
		return fmt.Errorf("get websocket URL: %w", err)
	}

	// 建立 WebSocket 连接
	if err := c.connect(wsURL); err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	// 启动消息处理循环
	c.wg.Add(1)
	go c.messageLoop()

	// 启动心跳
	c.wg.Add(1)
	go c.pingLoop()

	c.logger.Info("WebSocket client connected", "url", wsURL)
	return nil
}

// getWebSocketURL 获取 WebSocket 连接地址
func (c *WebSocketClient) getWebSocketURL(ctx context.Context) (string, error) {
	// 获取 app access token
	token, _, err := c.GetAppTokenWithContext(ctx)
	if err != nil {
		return "", err
	}

	url := feishuAPIBase + feishuWebSocketAPI

	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	var wsResp WebSocketResponse
	if err := json.NewDecoder(resp.Body).Decode(&wsResp); err != nil {
		return "", err
	}

	if wsResp.Code != 0 {
		c.logger.Error("Failed to get WebSocket URL", "code", wsResp.Code, "msg", wsResp.Msg)
		return "", &APIError{Code: wsResp.Code, Msg: wsResp.Msg}
	}

	if wsResp.Data == nil || wsResp.Data.URL == "" {
		return "", fmt.Errorf("websocket URL not found in response")
	}

	return wsResp.Data.URL, nil
}

// connect 建立底层的 WebSocket 连接
func (c *WebSocketClient) connect(url string) error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	dialer := websocket.DefaultDialer
	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		return fmt.Errorf("dial websocket: %w", err)
	}

	c.conn = conn
	c.connected = true

	// 设置 pong 处理器
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(wsPongWait))
	})

	// 调用连接成功回调
	if c.onConnect != nil {
		c.onConnect()
	}

	return nil
}

// messageLoop 消息处理循环
func (c *WebSocketClient) messageLoop() {
	defer c.wg.Done()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			if err := c.readMessage(); err != nil {
				c.logger.Error("Failed to read message", "error", err)

				// 处理断开连接
				c.handleDisconnect(err)

				// 尝试重连
				if !c.reconnect() {
					return
				}
			}
		}
	}
}

// readMessage 读取并处理一条 WebSocket 消息
func (c *WebSocketClient) readMessage() error {
	c.connMu.Lock()
	conn := c.conn
	c.connMu.Unlock()

	if conn == nil {
		return fmt.Errorf("connection not established")
	}

	// 设置读取超时
	if err := conn.SetReadDeadline(time.Now().Add(wsPongWait)); err != nil {
		return err
	}

	_, message, err := conn.ReadMessage()
	if err != nil {
		return err
	}

	// 解析消息
	var wsMsg WebSocketMessage
	if err := json.Unmarshal(message, &wsMsg); err != nil {
		c.logger.Warn("Failed to parse WebSocket message", "error", err, "message", string(message))
		return nil
	}

	// 处理不同类型的消息
	switch wsMsg.Type {
	case wsMessageTypePing:
		// 响应 ping
		c.handlePing(wsMsg.Data)
	case wsMessageTypeError:
		// 处理错误
		c.handleError(wsMsg.Data)
	default:
		// 处理事件
		c.handleEvent(wsMsg)
	}

	return nil
}

// pingLoop 心跳循环
func (c *WebSocketClient) pingLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(wsPingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			if err := c.sendPing(); err != nil {
				c.logger.Error("Failed to send ping", "error", err)
			}
		}
	}
}

// sendPing 发送心跳消息
func (c *WebSocketClient) sendPing() error {
	c.connMu.Lock()
	conn := c.conn
	c.connMu.Unlock()

	if conn == nil {
		return fmt.Errorf("connection not established")
	}

	pingMsg := WebSocketMessage{
		Type: wsMessageTypePing,
		Data: json.RawMessage(fmt.Sprintf(`{"timestamp":%d}`, time.Now().Unix())),
	}

	return conn.WriteJSON(pingMsg)
}

// handlePing 处理服务器发来的 ping 消息
func (c *WebSocketClient) handlePing(data json.RawMessage) {
	c.connMu.Lock()
	conn := c.conn
	c.connMu.Unlock()

	if conn == nil {
		return
	}

	pongMsg := WebSocketMessage{
		Type: wsMessageTypePong,
		Data: data,
	}

	if err := conn.WriteJSON(pongMsg); err != nil {
		c.logger.Error("Failed to send pong", "error", err)
	}
}

// handleError 处理错误消息
func (c *WebSocketClient) handleError(data json.RawMessage) {
	c.logger.Error("WebSocket error message", "data", string(data))
}

// handleEvent 处理事件消息
func (c *WebSocketClient) handleEvent(wsMsg WebSocketMessage) {
	// 解析事件
	var event Event
	if err := json.Unmarshal(wsMsg.Data, &event); err != nil {
		c.logger.Warn("Failed to parse event", "error", err, "data", string(wsMsg.Data))
		return
	}

	// 调用事件处理器
	if c.eventHandler != nil {
		c.eventHandler(&event)
	}
}

// handleDisconnect 处理断开连接
func (c *WebSocketClient) handleDisconnect(err error) {
	c.connMu.Lock()
	c.connected = false
	conn := c.conn
	c.conn = nil
	c.connMu.Unlock()

	// 关闭连接
	if conn != nil {
		_ = conn.Close()
	}

	// 调用断开连接回调
	if c.onDisconnect != nil {
		c.onDisconnect(err)
	}

	c.logger.Warn("WebSocket disconnected", "error", err)
}

// reconnect 尝试重新连接
func (c *WebSocketClient) reconnect() bool {
	for i := 1; i <= wsMaxReconnectTries; i++ {
		select {
		case <-c.ctx.Done():
			return false
		default:
		}

		c.logger.Info("Attempting to reconnect", "attempt", i, "max_tries", wsMaxReconnectTries)

		time.Sleep(wsReconnectDelay * time.Duration(i))

		// 获取新的 WebSocket URL
		wsURL, err := c.getWebSocketURL(c.ctx)
		if err != nil {
			c.logger.Error("Failed to get WebSocket URL for reconnection", "attempt", i, "error", err)
			continue
		}

		// 尝试连接
		if err := c.connect(wsURL); err != nil {
			c.logger.Error("Failed to reconnect", "attempt", i, "error", err)
			continue
		}

		c.logger.Info("WebSocket reconnected successfully", "attempt", i)
		return true
	}

	c.logger.Error("Max reconnection attempts reached")
	return false
}

// Close 关闭 WebSocket 连接
func (c *WebSocketClient) Close() error {
	// 取消上下文
	if c.cancel != nil {
		c.cancel()
	}

	// 等待所有 goroutine 完成
	c.wg.Wait()

	// 关闭连接
	c.connMu.Lock()
	conn := c.conn
	c.conn = nil
	c.connected = false
	c.connMu.Unlock()

	if conn != nil {
		_ = conn.Close()
	}

	c.logger.Info("WebSocket client closed")
	return nil
}

// IsConnected 返回当前连接状态
func (c *WebSocketClient) IsConnected() bool {
	c.connMu.Lock()
	defer c.connMu.Unlock()
	return c.connected
}

// GetAppTokenWithContext 获取 app access token（实现 FeishuAPIClient 接口）
func (c *WebSocketClient) GetAppTokenWithContext(ctx context.Context) (string, int, error) {
	// 委托给实际的 API 客户端
	// 这里需要创建一个临时的 Client 实例
	client := NewClient(c.appID, c.appSecret, c.logger)
	return client.GetAppTokenWithContext(ctx)
}
