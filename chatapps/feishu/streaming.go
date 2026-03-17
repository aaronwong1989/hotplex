package feishu

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/hrygo/hotplex/chatapps/base"
)

const (
	// 流式更新节流间隔（推荐 0.5 秒以避免频率限制）
	flushInterval = 500 * time.Millisecond

	// 触发立即 flush 的 rune 数量阈值
	flushSize = 50

	// 最大单次更新内容长度（飞书卡片有内容限制）
	maxUpdateSize = 8000

	// 流式消息最大 TTL（10 分钟，与 Slack 保持一致）
	streamTTL = 10 * time.Minute
)

// StreamingWriter 实现 io.WriteCloser 接口，封装飞书卡片流式消息的生命周期管理
// 流程：首次 Write -> CreateCard -> SendCardMessage
//       后续 Write -> 累积到缓冲区 -> 后台 flushLoop -> UpdateCard (with sequence++)
//       Close -> 最终 UpdateCard
type StreamingWriter struct {
	ctx     context.Context
	adapter *Adapter
	chatID  string
	token   string

	mu         sync.Mutex
	started    bool
	closed     bool
	cardID     string // 卡片实体 ID
	messageID  string // 消息 ID
	sequence   int    // 严格递增的序列号

	// 缓冲流控机制
	buf          bytes.Buffer
	flushTrigger chan struct{}
	closeChan    chan struct{}
	wg           sync.WaitGroup

	// 内容累积（用于完整性校验和 fallback）
	accumulatedContent bytes.Buffer
	bytesWritten       int64
	bytesFlushed       int64

	// TTL 监控
	streamStartTime  time.Time
	streamExpired    bool
	ttlWarningLogged bool

	// 卡片构建器
	cardBuilder *CardBuilder

	// 完成回调
	onComplete func(messageID string)

	// 存储回调（可选）
	storeCallback func(content string)
}

// NewStreamingWriter 创建新的流式写入器
func NewStreamingWriter(
	ctx context.Context,
	adapter *Adapter,
	chatID string,
	onComplete func(messageID string),
) *StreamingWriter {
	w := &StreamingWriter{
		ctx:          ctx,
		adapter:      adapter,
		chatID:       chatID,
		onComplete:   onComplete,
		flushTrigger: make(chan struct{}, 1),
		closeChan:    make(chan struct{}),
		cardBuilder:  NewCardBuilder(""), // sessionID not needed for streaming
	}

	w.wg.Add(1)
	go w.flushLoop()

	return w
}

// SetStoreCallback sets the callback to store the complete message content
func (w *StreamingWriter) SetStoreCallback(callback func(content string)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.storeCallback = callback
}

// flushLoop 后台 goroutine，定期或按需执行卡片更新
func (w *StreamingWriter) flushLoop() {
	defer w.wg.Done()

	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			w.flushBuffer()
			return
		case <-w.closeChan:
			w.flushBuffer()
			return
		case <-w.flushTrigger:
			w.flushBuffer()
		case <-ticker.C:
			w.flushBuffer()
		}
	}
}

// chunkContent splits content into chunks that fit within maxUpdateSize
func (w *StreamingWriter) chunkContent(content string) []string {
	if utf8.RuneCountInString(content) <= maxUpdateSize {
		return []string{content}
	}

	// Use base chunking for large content
	return base.ChunkMessage(content, base.ChunkerConfig{
		MaxLen:        maxUpdateSize,
		PreserveWords: true,
		AddNumbering:  false, // Don't add numbering for streaming updates
	})
}

// flushBuffer 执行实际的卡片更新操作
func (w *StreamingWriter) flushBuffer() {
	w.mu.Lock()
	if w.buf.Len() == 0 {
		w.mu.Unlock()
		return
	}

	content := w.buf.String()
	w.buf.Reset()
	started := w.started
	cardID := w.cardID
	sequence := w.sequence
	streamExpired := w.streamExpired
	streamStartTime := w.streamStartTime
	w.mu.Unlock()

	// 如果流未启动，不应该有内容需要 flush
	if !started {
		return
	}

	// TTL 检测：如果流已超时，不再更新
	if streamExpired || time.Since(streamStartTime) > streamTTL {
		w.mu.Lock()
		if !w.ttlWarningLogged {
			w.adapter.Logger().Warn("Stream TTL exceeded, marking as expired",
				"chat_id", w.chatID,
				"card_id", cardID,
				"stream_age", time.Since(streamStartTime),
				"ttl", streamTTL)
			w.ttlWarningLogged = true
		}
		w.streamExpired = true
		w.mu.Unlock()
		return
	}

	// 执行卡片更新
	chunks := w.chunkContent(content)
	var lastErr error
	var bytesSent int

	for chunkIdx, chunk := range chunks {
		// 为每个块构建新的卡片内容（直接获取 CardTemplate，避免双重 JSON 序列化）
		cardTemplate := w.cardBuilder.BuildAnswerCardTemplate(chunk)

		// 调用 UpdateCard API
		if err := w.adapter.client.UpdateCard(w.ctx, w.token, cardID, cardTemplate, sequence+chunkIdx+1); err != nil {
			lastErr = err
			w.adapter.Logger().Error("UpdateCard failed",
				"card_id", cardID,
				"sequence", sequence+chunkIdx+1,
				"chunk_index", chunkIdx,
				"error", err)
			break
		}

		bytesSent += len(chunk)
	}

	// 如果所有块都成功更新
	if lastErr == nil {
		w.mu.Lock()
		w.sequence += len(chunks)
		w.bytesFlushed += int64(bytesSent)
		w.mu.Unlock()
	}
}

// Write 实现 io.Writer 接口
// 首次调用执行 CreateCard + SendCardMessage；后续调用将内容追加到缓冲区并触发异步 UpdateCard
func (w *StreamingWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return 0, fmt.Errorf("stream already closed")
	}

	if len(p) == 0 {
		return 0, nil
	}

	// 首次调用，启动流
	if !w.started {
		// 获取 token
		token, err := w.adapter.GetAppTokenWithContext(w.ctx)
		if err != nil {
			return 0, fmt.Errorf("get app token: %w", err)
		}
		w.token = token

		// 创建初始卡片实体（空内容或占位符，直接获取 CardTemplate）
		cardTemplate := w.cardBuilder.BuildAnswerCardTemplate("⏳ 正在生成回复...")

		cardID, err := w.adapter.client.CreateCard(w.ctx, token, cardTemplate)
		if err != nil {
			return 0, fmt.Errorf("create card: %w", err)
		}
		w.cardID = cardID

		// 发送卡片消息
		messageID, err := w.adapter.client.SendCardMessage(w.ctx, token, w.chatID, cardID)
		if err != nil {
			return 0, fmt.Errorf("send card message: %w", err)
		}
		w.messageID = messageID

		w.started = true
		w.sequence = 0 // 初始 sequence 为 0，第一次 UpdateCard 使用 1
		w.streamStartTime = time.Now()
	}

	w.buf.Write(p)
	w.accumulatedContent.Write(p)
	w.bytesWritten += int64(len(p))

	// 如果超过 rune 阈值，立即触发一次 flush
	if utf8.RuneCount(w.buf.Bytes()) >= flushSize {
		select {
		case w.flushTrigger <- struct{}{}:
		default:
		}
	}

	return len(p), nil
}

// Close 结束流，更新最终内容并清理资源
func (w *StreamingWriter) Close() error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}

	w.closed = true
	w.mu.Unlock()

	// 停止后台 goroutine
	close(w.closeChan)
	w.wg.Wait()

	// 捕获最终状态
	w.mu.Lock()
	started := w.started
	accumulated := w.accumulatedContent.String()
	bytesWritten := w.bytesWritten
	bytesFlushed := w.bytesFlushed
	cardID := w.cardID
	sequence := w.sequence
	streamExpired := w.streamExpired
	storeCallback := w.storeCallback
	token := w.token
	w.mu.Unlock()

	if !started {
		return nil
	}

	// 完整性校验
	integrityOK := bytesWritten == bytesFlushed && !streamExpired

	if !integrityOK {
		w.adapter.Logger().Warn("Stream integrity check failed",
			"chat_id", w.chatID,
			"card_id", cardID,
			"bytes_written", bytesWritten,
			"bytes_flushed", bytesFlushed,
			"stream_expired", streamExpired)
	}

	// 如果流未过期，发送最终完整内容
	if !streamExpired && len(accumulated) > 0 {
		// 构建最终卡片（完整内容，直接获取 CardTemplate）
		finalCardTemplate := w.cardBuilder.BuildAnswerCardTemplate(accumulated)

		// 最终更新，sequence 严格递增
		if err := w.adapter.client.UpdateCard(w.ctx, token, cardID, finalCardTemplate, sequence+1); err != nil {
			w.adapter.Logger().Error("Final UpdateCard failed",
				"card_id", cardID,
				"sequence", sequence+1,
				"error", err)
		} else {
			w.mu.Lock()
			w.sequence++
			w.bytesFlushed = w.bytesWritten
			w.mu.Unlock()
		}
	}

	// 调用完成回调
	if w.onComplete != nil {
		w.onComplete(w.messageID)
	}

	// 存储完整内容
	if storeCallback != nil && accumulated != "" {
		storeCallback(accumulated)
	}

	return nil
}

// MessageID 返回流式消息的 ID
func (w *StreamingWriter) MessageID() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.messageID
}

// MessageTS 返回消息时间戳（为了兼容 base.StreamWriter 接口，飞书使用 messageID）
func (w *StreamingWriter) MessageTS() string {
	return w.MessageID()
}

// FallbackUsed 返回是否使用了 fallback 机制
// 飞书的卡片更新没有编辑次数限制，因此不需要 fallback
func (w *StreamingWriter) FallbackUsed() bool {
	return false
}

// CardID 返回卡片实体 ID
func (w *StreamingWriter) CardID() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.cardID
}

// IsStarted 返回流是否已启动
func (w *StreamingWriter) IsStarted() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.started
}

// IsClosed 返回流是否已关闭
func (w *StreamingWriter) IsClosed() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.closed
}

// StreamWriterStats 流式消息统计信息
type StreamWriterStats struct {
	BytesWritten int64 // 成功写入的总字节数
	BytesFlushed int64 // 成功 flush 的总字节数
	Sequence     int   // 当前序列号
	IntegrityOK  bool  // 完整性检查是否通过
	ContentLength int  // 累积内容长度
}

// GetStats 返回流统计信息
func (w *StreamingWriter) GetStats() StreamWriterStats {
	w.mu.Lock()
	defer w.mu.Unlock()

	return StreamWriterStats{
		BytesWritten:  w.bytesWritten,
		BytesFlushed:  w.bytesFlushed,
		Sequence:      w.sequence,
		IntegrityOK:   w.bytesWritten == w.bytesFlushed && !w.streamExpired,
		ContentLength: w.accumulatedContent.Len(),
	}
}

// Ensure StreamingWriter implements io.WriteCloser at compile time
var _ io.WriteCloser = (*StreamingWriter)(nil)

// Ensure StreamingWriter implements base.StreamWriter at compile time
var _ base.StreamWriter = (*StreamingWriter)(nil)
