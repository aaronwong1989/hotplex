package engine

import (
	"sync"
	"time"
)

// SessionStats collects session-level statistics for Geek/Evolution modes.
type SessionStats struct {
	mu                   sync.Mutex      `json:"-"`
	SessionID            string          `json:"session_id"`
	StartTime            time.Time       `json:"-"` // Internal use only, use ToSummary() for JSON
	TotalDurationMs      int64           `json:"total_duration_ms"`
	ThinkingDurationMs   int64           `json:"thinking_duration_ms"`
	ToolDurationMs       int64           `json:"tool_duration_ms"`
	GenerationDurationMs int64           `json:"generation_duration_ms"`
	InputTokens          int32           `json:"input_tokens"`
	OutputTokens         int32           `json:"output_tokens"`
	CacheWriteTokens     int32           `json:"cache_write_tokens"`
	CacheReadTokens      int32           `json:"cache_read_tokens"`
	ToolCallCount        int32           `json:"tool_call_count"`
	ToolsUsed            map[string]bool `json:"-"` // Internal set, use ToSummary() for array
	FilesModified        int32           `json:"files_modified"`
	FilePaths            []string        `json:"file_paths"`
	filePathsSet         map[string]bool `json:"-"` // O(1) deduplication for file paths

	// Last API call token stats (for context window usage calculation)
	// These fields store the token counts from the MOST RECENT provider event,
	// not the accumulated totals. Context window limits apply per API call.
	lastInputTokens      int32 `json:"-"`
	lastCacheReadTokens  int32 `json:"-"`
	lastCacheWriteTokens int32 `json:"-"`

	// Current tool tracking
	currentToolStart time.Time `json:"-"`
	currentToolName  string    `json:"-"`
	currentToolID    string    `json:"-"`

	// Tool ID to name mapping for reliable lookup when tool_result doesn't include name
	// This handles the case where Claude CLI sends tool_result without tool_name
	toolIDMap map[string]string `json:"-"`

	// Phase tracking for duration breakdown
	thinkingStart   time.Time `json:"-"`
	generationStart time.Time `json:"-"`
	hasGeneration   bool      `json:"-"` // Tracks if any content was generated
}

// GetCurrentToolName returns the current tool name being tracked.
// Returns empty string if no tool is currently being tracked.
func (s *SessionStats) GetCurrentToolName() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.currentToolName
}

// GetCurrentToolID returns the current tool ID being tracked.
// Returns empty string if no tool is currently being tracked.
func (s *SessionStats) GetCurrentToolID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.currentToolID
}

// GetToolNameByToolID returns the tool name for a given tool ID.
// This is useful when tool_result events don't include tool_name.
// Returns empty string if tool ID is not found.
func (s *SessionStats) GetToolNameByToolID(toolID string) string {
	if toolID == "" {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.toolIDMap == nil {
		return ""
	}
	return s.toolIDMap[toolID]
}

// RecordToolUse records the start of a tool call.
func (s *SessionStats) RecordToolUse(toolName, toolID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentToolStart = time.Now()
	s.currentToolName = toolName
	s.currentToolID = toolID
	s.ToolCallCount++ // Count here — tool_result may not arrive from all Providers
	// Ensure ToolsUsed map is initialized (concurrency safety)
	if s.ToolsUsed == nil {
		s.ToolsUsed = make(map[string]bool)
	}
	// Ensure toolIDMap is initialized
	if s.toolIDMap == nil {
		s.toolIDMap = make(map[string]string)
	}
	// Record tool ID to name mapping for reliable lookup in RecordToolResult
	// This handles the case where Claude CLI sends tool_result without tool_name
	if toolID != "" && toolName != "" {
		s.toolIDMap[toolID] = toolName
	}
}

// RecordToolResult records the end of a tool call.
// If RecordToolUse was not called (e.g., Claude Code didn't send tool_use event),
// this will record a minimal duration (1ms) to indicate the tool completed.
func (s *SessionStats) RecordToolResult(toolID string) (durationMs int64, toolName string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// ToolCallCount is incremented in RecordToolUse (not here)

	// Try to get tool name from toolID map first (most reliable)
	// This handles the case where Claude CLI sends tool_result without tool_name
	if toolID != "" && s.toolIDMap != nil {
		toolName = s.toolIDMap[toolID]
	}

	// Fallback: use currentToolName if toolID map didn't have it
	if toolName == "" {
		toolName = s.currentToolName
	}

	if !s.currentToolStart.IsZero() {
		// Normal case: tool_use was received, calculate actual duration
		duration := time.Since(s.currentToolStart)
		durationMs = duration.Milliseconds()
		s.ToolDurationMs += durationMs
	} else {
		// Missed tool_use event: record minimal duration (1ms)
		// This happens when Claude Code CLI doesn't send tool_use events
		durationMs = 1
		s.ToolDurationMs += durationMs
	}

	// Record tool name if available
	if toolName != "" {
		if s.ToolsUsed == nil {
			s.ToolsUsed = make(map[string]bool)
		}
		s.ToolsUsed[toolName] = true
	}

	// Reset current tool tracking
	s.currentToolStart = time.Time{}
	s.currentToolName = ""
	s.currentToolID = ""

	return durationMs, toolName
}

// RecordTokens records token usage.
func (s *SessionStats) RecordTokens(input, output, cacheWrite, cacheRead int32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.InputTokens += input
	s.OutputTokens += output
	s.CacheWriteTokens += cacheWrite
	s.CacheReadTokens += cacheRead
}

// SetLastCallTokens records the token counts from the most recent API call.
// This is used for calculating context window usage percentage, which applies
// per API call (not to accumulated session totals).
func (s *SessionStats) SetLastCallTokens(input, cacheRead, cacheWrite int32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastInputTokens = input
	s.lastCacheReadTokens = cacheRead
	s.lastCacheWriteTokens = cacheWrite
}

// GetLastCallTokens returns the token counts from the most recent API call.
// Returns: input, cacheRead, cacheWrite
func (s *SessionStats) GetLastCallTokens() (int32, int32, int32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastInputTokens, s.lastCacheReadTokens, s.lastCacheWriteTokens
}

// StartThinking marks the start of the thinking phase.
func (s *SessionStats) StartThinking() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.thinkingStart.IsZero() {
		s.thinkingStart = time.Now()
	}
}

// EndThinking marks the end of the thinking phase and records its duration.
func (s *SessionStats) EndThinking() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.thinkingStart.IsZero() {
		s.ThinkingDurationMs += time.Since(s.thinkingStart).Milliseconds()
		s.thinkingStart = time.Time{} // Reset for next thinking phase
	}
}

// StartGeneration marks the start of the generation phase.
func (s *SessionStats) StartGeneration() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.generationStart.IsZero() {
		s.generationStart = time.Now()
		s.hasGeneration = true
	}
}

// EndGeneration marks the end of the generation phase and records its duration.
func (s *SessionStats) EndGeneration() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.generationStart.IsZero() {
		s.GenerationDurationMs += time.Since(s.generationStart).Milliseconds()
		s.generationStart = time.Time{} // Reset for next generation phase
	}
}

// RecordFileModification records that a file was modified.
// Uses O(1) map lookup for deduplication instead of O(n) linear scan.
func (s *SessionStats) RecordFileModification(filePath string) {
	if filePath == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	// Initialize map if needed
	if s.filePathsSet == nil {
		s.filePathsSet = make(map[string]bool)
	}

	// O(1) deduplication check
	if s.filePathsSet[filePath] {
		return // Already recorded
	}

	s.filePathsSet[filePath] = true
	s.FilePaths = append(s.FilePaths, filePath)
	s.FilesModified++
}

// ToSummary converts stats to a summary map for JSON serialization.
func (s *SessionStats) ToSummary() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	tools := make([]string, 0, len(s.ToolsUsed))
	for tool := range s.ToolsUsed {
		tools = append(tools, tool)
	}

	return map[string]any{
		"session_id":             s.SessionID,
		"total_duration_ms":      s.TotalDurationMs,
		"thinking_duration_ms":   s.ThinkingDurationMs,
		"tool_duration_ms":       s.ToolDurationMs,
		"generation_duration_ms": s.GenerationDurationMs,
		"input_tokens":           s.InputTokens,
		"output_tokens":          s.OutputTokens,
		"cache_write_tokens":     s.CacheWriteTokens,
		"cache_read_tokens":      s.CacheReadTokens,
		"tool_call_count":        s.ToolCallCount,
		"tools_used":             tools,
		"files_modified":         s.FilesModified,
		"file_paths":             s.FilePaths,
		"status":                 "success",
	}
}

// FinalizeDuration finalizes any ongoing phase tracking and returns the final stats.
func (s *SessionStats) FinalizeDuration() *SessionStats {
	s.mu.Lock()
	defer s.mu.Unlock()

	totalThinking := s.ThinkingDurationMs
	totalGeneration := s.GenerationDurationMs

	if !s.thinkingStart.IsZero() {
		totalThinking += time.Since(s.thinkingStart).Milliseconds()
	}
	if !s.generationStart.IsZero() {
		totalGeneration += time.Since(s.generationStart).Milliseconds()
	}

	return &SessionStats{
		SessionID:            s.SessionID,
		StartTime:            s.StartTime,
		TotalDurationMs:      s.TotalDurationMs,
		ThinkingDurationMs:   totalThinking,
		ToolDurationMs:       s.ToolDurationMs,
		GenerationDurationMs: totalGeneration,
		InputTokens:          s.InputTokens,
		OutputTokens:         s.OutputTokens,
		CacheWriteTokens:     s.CacheWriteTokens,
		CacheReadTokens:      s.CacheReadTokens,
		ToolCallCount:        s.ToolCallCount,
		ToolsUsed:            s.ToolsUsed,
		FilesModified:        s.FilesModified,
		FilePaths:            s.FilePaths,
	}
}
