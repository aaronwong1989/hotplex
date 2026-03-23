package base

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/hrygo/hotplex/plugins/storage"
	"github.com/hrygo/hotplex/types"
)

// StreamBuffer 流式消息缓冲区 (内存)
type StreamBuffer struct {
	SessionID   string
	Chunks      []string
	IsComplete  bool
	LastUpdated time.Time
	mu          sync.RWMutex
}

// Append 追加 chunk 到缓冲区
func (b *StreamBuffer) Append(chunk string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Chunks = append(b.Chunks, chunk)
	b.LastUpdated = time.Now()
}

// Merge 合并所有 chunk 为完整消息
func (b *StreamBuffer) Merge() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if len(b.Chunks) == 0 {
		return ""
	}
	result := make([]byte, 0)
	for _, chunk := range b.Chunks {
		result = append(result, []byte(chunk)...)
	}
	return string(result)
}

// IsExpired 检查缓冲区是否超时
func (b *StreamBuffer) IsExpired(timeout time.Duration) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return time.Since(b.LastUpdated) > timeout
}

// Clear 清空缓冲区
func (b *StreamBuffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Chunks = nil
	b.IsComplete = false
}

// StreamMessageStore 流式消息存储管理器
type StreamMessageStore struct {
	buffers     map[string]*StreamBuffer
	mu          sync.RWMutex
	store       storage.WriteOnlyStore
	timeout     time.Duration
	maxBuffers  int
	logger      *slog.Logger
	cleanupStop chan struct{}
	cleanupWg   sync.WaitGroup

	// 自动提交配置
	autoCommitEnabled  bool
	autoCommitInterval time.Duration
}

// ErrBufferFull 缓冲区已满错误
var ErrBufferFull = errors.New("stream buffer full, cannot accept new chunks")

// StreamMessageStoreConfig 流式消息存储配置
type StreamMessageStoreConfig struct {
	Timeout            time.Duration // 缓冲区超时时间
	MaxBuffers         int           // 最大缓冲区数量
	AutoCommitEnabled  bool          // 是否启用自动提交
	AutoCommitInterval time.Duration // 自动提交间隔
	SaveOnTermination  bool          // 会话终止前是否保存未提交数据
}

// DefaultStreamMessageStoreConfig 返回默认配置
func DefaultStreamMessageStoreConfig() *StreamMessageStoreConfig {
	return &StreamMessageStoreConfig{
		Timeout:            5 * time.Minute,
		MaxBuffers:         1000,
		AutoCommitEnabled:  true,
		AutoCommitInterval: 30 * time.Second,
	}
}

// NewStreamMessageStore 创建流式消息存储管理器
func NewStreamMessageStore(store storage.WriteOnlyStore, timeout time.Duration, maxBuffers int, logger *slog.Logger) *StreamMessageStore {
	config := DefaultStreamMessageStoreConfig()
	config.Timeout = timeout
	config.MaxBuffers = maxBuffers
	return NewStreamMessageStoreWithConfig(store, config, logger)
}

// NewStreamMessageStoreWithConfig 使用配置创建流式消息存储管理器
func NewStreamMessageStoreWithConfig(store storage.WriteOnlyStore, config *StreamMessageStoreConfig, logger *slog.Logger) *StreamMessageStore {
	if logger == nil {
		logger = slog.Default()
	}
	if config == nil {
		config = DefaultStreamMessageStoreConfig()
	}

	s := &StreamMessageStore{
		buffers:            make(map[string]*StreamBuffer),
		store:              store,
		timeout:            config.Timeout,
		maxBuffers:         config.MaxBuffers,
		logger:             logger,
		cleanupStop:        make(chan struct{}),
		autoCommitEnabled:  config.AutoCommitEnabled,
		autoCommitInterval: config.AutoCommitInterval,
	}
	s.startCleanup()
	s.startAutoCommit()
	return s
}

// startCleanup 启动后台清理 goroutine
func (s *StreamMessageStore) startCleanup() {
	s.cleanupWg.Add(1)
	go func() {
		defer s.cleanupWg.Done()
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-s.cleanupStop:
				return
			case <-ticker.C:
				s.cleanupExpired()
			}
		}
	}()
}

// cleanupExpired 清理超时的缓冲区
func (s *StreamMessageStore) cleanupExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for sessionID, buf := range s.buffers {
		if buf.IsExpired(s.timeout) {
			delete(s.buffers, sessionID)
		}
	}
}

// startAutoCommit 启动定期自动提交 goroutine
func (s *StreamMessageStore) startAutoCommit() {
	if !s.autoCommitEnabled {
		return
	}

	s.cleanupWg.Add(1)
	go func() {
		defer s.cleanupWg.Done()
		ticker := time.NewTicker(s.autoCommitInterval)
		defer ticker.Stop()

		s.logger.Info("stream message auto-commit started",
			"interval", s.autoCommitInterval)

		for {
			select {
			case <-s.cleanupStop:
				s.logger.Info("stream message auto-commit stopped")
				return
			case <-ticker.C:
				s.flushCompletedBuffers()
			}
		}
	}()
}

// flushCompletedBuffers 提交已完成的缓冲区到数据库
func (s *StreamMessageStore) flushCompletedBuffers() {
	s.mu.Lock()
	defer s.mu.Unlock()

	flushedCount := 0
	for sessionID, buf := range s.buffers {
		if buf.IsComplete {
			mergedContent := buf.Merge()
			if mergedContent != "" {
				// 异步存储，不阻塞清理循环
				go func(sid, content string) {
					err := s.store.StoreBotResponse(context.Background(), &storage.ChatAppMessage{
						ChatSessionID: sid,
						Content:       content,
						MessageType:   types.MessageTypeFinalResponse,
						CreatedAt:     time.Now(),
					})
					if err != nil {
						s.logger.Error("failed to flush completed buffer",
							"session_id", sid,
							"error", err)
					} else {
						s.logger.Info("flushed completed buffer",
							"session_id", sid,
							"content_len", len(content))
					}
				}(sessionID, mergedContent)

				delete(s.buffers, sessionID)
				flushedCount++
			}
		}
	}

	if flushedCount > 0 {
		s.logger.Info("auto-commit completed",
			"flushed_count", flushedCount)
	}
}

// Close 停止清理 goroutine
func (s *StreamMessageStore) Close() {
	close(s.cleanupStop)
	s.cleanupWg.Wait()
}

// OnStreamChunk 接收流式消息块 (不存储,仅缓存)
// 如果缓冲区满,降级为直接存储模式,防止数据丢失
func (s *StreamMessageStore) OnStreamChunk(ctx context.Context, sessionID, chunk string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 如果缓冲区已满且无法清理,降级为直接存储
	if len(s.buffers) >= s.maxBuffers {
		// 查找并删除过期的缓冲区
		evicted := false
		for id, buf := range s.buffers {
			if buf.IsExpired(s.timeout) {
				s.logger.Warn("evicting expired stream buffer",
					"session_id", id,
					"chunk_count", len(buf.Chunks),
					"reason", "buffer overflow")
				delete(s.buffers, id)
				evicted = true
				break
			}
		}
		// 如果没有过期的缓冲区可删除,降级为直接存储
		if !evicted {
			s.logger.Warn("stream buffer full, falling back to direct storage",
				"max_buffers", s.maxBuffers,
				"session_id", sessionID,
				"fallback", "direct_store")
			// 降级:直接存储chunk,不缓存(防止数据丢失)
			return s.store.StoreBotResponse(ctx, &storage.ChatAppMessage{
				ChatSessionID: sessionID,
				Content:       chunk,
				MessageType:   types.MessageTypeFinalResponse,
				CreatedAt:     time.Now(),
			})
		}
	}

	buf, exists := s.buffers[sessionID]
	if !exists {
		buf = &StreamBuffer{
			SessionID:   sessionID,
			Chunks:      make([]string, 0),
			LastUpdated: time.Now(),
		}
		s.buffers[sessionID] = buf
	}

	buf.Append(chunk)

	// 流式状态监控日志
	s.logger.Debug("stream chunk received",
		"session_id", sessionID,
		"chunk_len", len(chunk),
		"total_chunks", len(buf.Chunks),
		"buffer_count", len(s.buffers),
		"max_buffers", s.maxBuffers)

	return nil
}

// OnStreamComplete 流式消息完成 (合并后存储)
func (s *StreamMessageStore) OnStreamComplete(ctx context.Context, sessionID string, msg *storage.ChatAppMessage) error {
	s.mu.Lock()
	buf, exists := s.buffers[sessionID]
	if exists {
		buf.IsComplete = true
	}
	s.mu.Unlock()

	if !exists {
		// 没有缓冲区,直接存储 (非流式消息)
		s.logger.Warn("stream complete but no buffer found",
			"session_id", sessionID,
			"fallback", "direct_store")
		return s.store.StoreBotResponse(ctx, msg)
	}

	// 合并 chunk
	mergedContent := buf.Merge()
	if mergedContent == "" {
		s.logger.Debug("stream complete, buffer is empty",
			"session_id", sessionID,
			"chunk_count", len(buf.Chunks))
		return nil
	}

	s.logger.Info("stream complete, saving message",
		"session_id", sessionID,
		"chunk_count", len(buf.Chunks),
		"merged_len", len(mergedContent))

	// 更新消息内容为合并后的完整内容
	msg.Content = mergedContent

	// 存储最终结果
	err := s.store.StoreBotResponse(ctx, msg)

	// 清理缓冲区
	s.mu.Lock()
	delete(s.buffers, sessionID)
	s.mu.Unlock()

	return err
}

// GetBuffer 获取指定 session 的缓冲区 (用于调试/监控)
// 返回 any 类型以匹配 engine.StreamDataSaver 接口
func (s *StreamMessageStore) GetBuffer(sessionID string) any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if buf, exists := s.buffers[sessionID]; exists {
		return buf
	}
	return nil
}

// SaveIncompleteStream 保存未完成的流式数据 (实现 engine.StreamDataSaver)
// 在会话终止前同步调用，确保未提交的数据不会丢失
func (s *StreamMessageStore) SaveIncompleteStream(ctx context.Context, sessionID string, buffer any) error {
	buf, ok := buffer.(*StreamBuffer)
	if !ok || buf == nil {
		return nil
	}

	// 标记为完成
	buf.mu.Lock()
	buf.IsComplete = true
	buf.mu.Unlock()

	// 合并内容
	mergedContent := buf.Merge()
	if mergedContent == "" {
		s.logger.Debug("incomplete stream buffer is empty, skipping save",
			"session_id", sessionID)
		return nil
	}

	s.logger.Info("saving incomplete stream buffer before termination",
		"session_id", sessionID,
		"chunk_count", len(buf.Chunks),
		"content_len", len(mergedContent))

	// 同步保存（阻塞以确保数据不丢失）
	err := s.store.StoreBotResponse(ctx, &storage.ChatAppMessage{
		ChatSessionID: sessionID,
		Content:       mergedContent,
		MessageType:   types.MessageTypeFinalResponse,
		CreatedAt:     time.Now(),
	})

	if err != nil {
		s.logger.Error("failed to save incomplete stream",
			"session_id", sessionID,
			"error", err)
		return err
	}

	// 清理缓冲区
	s.mu.Lock()
	delete(s.buffers, sessionID)
	s.mu.Unlock()

	s.logger.Info("incomplete stream saved successfully",
		"session_id", sessionID)

	return nil
}

// GetBufferCount 获取当前活跃的缓冲区数量
func (s *StreamMessageStore) GetBufferCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.buffers)
}

// StreamMetrics 流式存储指标
type StreamMetrics struct {
	ActiveBuffers    int     `json:"active_buffers"`     // 活跃缓冲区数量
	CompletedBuffers int     `json:"completed_buffers"`  // 已完成但未提交的缓冲区数量
	TotalChunks      int     `json:"total_chunks"`       // 所有缓冲区的总 chunk 数
	MaxBuffers       int     `json:"max_buffers"`        // 最大缓冲区数量
	TimeoutSeconds   float64 `json:"timeout_seconds"`    // 超时时间（秒）
}

// GetMetrics 导出流式存储指标 (用于监控/Admin API)
func (s *StreamMessageStore) GetMetrics() *StreamMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	activeBuffers := len(s.buffers)
	totalChunks := 0
	completedBuffers := 0

	for _, buf := range s.buffers {
		buf.mu.RLock()
		totalChunks += len(buf.Chunks)
		if buf.IsComplete {
			completedBuffers++
		}
		buf.mu.RUnlock()
	}

	return &StreamMetrics{
		ActiveBuffers:    activeBuffers,
		CompletedBuffers: completedBuffers,
		TotalChunks:      totalChunks,
		MaxBuffers:       s.maxBuffers,
		TimeoutSeconds:   s.timeout.Seconds(),
	}
}
