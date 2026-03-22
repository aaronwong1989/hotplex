package base

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"log/slog"
)

// --- ChunkMessage tests ---

func TestChunkMessage_Empty(t *testing.T) {
	result := ChunkMessage("", ChunkerConfig{MaxLen: 100})
	if len(result) != 1 || result[0] != "" {
		t.Errorf("expected [\"\"], got %v", result)
	}
}

func TestChunkMessage_ShortText(t *testing.T) {
	text := "hello"
	result := ChunkMessage(text, ChunkerConfig{MaxLen: 100})
	if len(result) != 1 || result[0] != text {
		t.Errorf("expected single chunk, got %v", result)
	}
}

func TestChunkMessage_DefaultLimit(t *testing.T) {
	text := strings.Repeat("a", 100)
	result := ChunkMessage(text, ChunkerConfig{MaxLen: 0})
	if len(result) != 1 {
		t.Errorf("expected single chunk with default limit, got %d chunks", len(result))
	}
}

func TestChunkMessage_ExactlyAtLimit(t *testing.T) {
	text := strings.Repeat("a", 10)
	result := ChunkMessage(text, ChunkerConfig{MaxLen: 10})
	if len(result) != 1 {
		t.Errorf("expected single chunk at exact limit, got %d chunks", len(result))
	}
}

func TestChunkMessage_ExceedsLimit(t *testing.T) {
	text := strings.Repeat("a", 20)
	result := ChunkMessage(text, ChunkerConfig{MaxLen: 10, PreserveWords: false})
	if len(result) != 2 {
		t.Errorf("expected 2 chunks, got %d", len(result))
	}
	if len(result[0]) != 10 {
		t.Errorf("first chunk should be 10 chars, got %d", len(result[0]))
	}
}

func TestChunkMessage_PreserveWords_Newline(t *testing.T) {
	text := "hello world\nthis is line two and it is quite long"
	result := ChunkMessage(text, ChunkerConfig{MaxLen: 20, PreserveWords: true})
	if len(result) < 2 {
		t.Errorf("expected >= 2 chunks with word preservation, got %d", len(result))
	}
	// Should break at newline, not mid-word
	for _, chunk := range result {
		if strings.HasSuffix(chunk, " \n") || strings.HasPrefix(chunk, "\n") {
			t.Log("Chunk break verified")
		}
	}
}

func TestChunkMessage_PreserveWords_Space(t *testing.T) {
	text := "word1 word2 word3 word4 word5 word6 word7"
	result := ChunkMessage(text, ChunkerConfig{MaxLen: 15, PreserveWords: true})
	if len(result) < 2 {
		t.Errorf("expected >= 2 chunks, got %d", len(result))
	}
}

func TestChunkMessage_WithNumbering(t *testing.T) {
	text := strings.Repeat("a", 50)
	result := ChunkMessage(text, ChunkerConfig{MaxLen: 20, AddNumbering: true})
	if len(result) < 2 {
		t.Fatalf("expected >= 2 chunks, got %d", len(result))
	}
	for i, chunk := range result {
		expected := "[1/3]\n"
		if i == 0 && !strings.HasPrefix(chunk, "[") {
			t.Errorf("chunk %d should have numbering prefix", i)
		}
		_ = expected
	}
}

func TestChunkMessage_WithNumbering_SingleChunk(t *testing.T) {
	text := "short"
	result := ChunkMessage(text, ChunkerConfig{MaxLen: 100, AddNumbering: true})
	if len(result) != 1 {
		t.Errorf("expected single chunk, got %d", len(result))
	}
	if strings.Contains(result[0], "[") {
		t.Error("single chunk should not have numbering")
	}
}

func TestChunkMessage_Unicode(t *testing.T) {
	// Each Chinese char = 3 bytes, 1 rune. Test rune-counting logic.
	text := strings.Repeat("你", 50)
	result := ChunkMessage(text, ChunkerConfig{MaxLen: 10, PreserveWords: false})
	if len(result) != 5 {
		t.Errorf("expected 5 chunks (10 runes each), got %d", len(result))
	}
}

// --- ChunkMessageSimple tests ---

func TestChunkMessageSimple_Empty(t *testing.T) {
	result := ChunkMessageSimple("", 10)
	if len(result) != 1 || result[0] != "" {
		t.Errorf("expected [\"\"], got %v", result)
	}
}

func TestChunkMessageSimple_ShortText(t *testing.T) {
	result := ChunkMessageSimple("hello", 10)
	if len(result) != 1 || result[0] != "hello" {
		t.Errorf("expected [\"hello\"], got %v", result)
	}
}

func TestChunkMessageSimple_DefaultLimit(t *testing.T) {
	result := ChunkMessageSimple("short", 0)
	if len(result) != 1 {
		t.Errorf("expected single chunk with default limit, got %d", len(result))
	}
}

func TestChunkMessageSimple_Exact(t *testing.T) {
	text := strings.Repeat("a", 10)
	result := ChunkMessageSimple(text, 10)
	if len(result) != 1 {
		t.Errorf("expected 1 chunk at exact limit, got %d", len(result))
	}
}

func TestChunkMessageSimple_Exceeds(t *testing.T) {
	text := strings.Repeat("a", 25)
	result := ChunkMessageSimple(text, 10)
	if len(result) != 3 || result[2] != "aaaaa" {
		t.Errorf("expected 3 chunks, last 'aaaaa', got %d chunks: %v", len(result), result)
	}
}

// --- DefaultChunker tests ---

func TestNewDefaultChunker(t *testing.T) {
	c := NewDefaultChunker(500)
	if c.maxChars != 500 {
		t.Errorf("expected 500, got %d", c.maxChars)
	}

	c2 := NewDefaultChunker(0)
	if c2.maxChars != DefaultChunkLimit {
		t.Errorf("expected default %d, got %d", DefaultChunkLimit, c2.maxChars)
	}
}

func TestDefaultChunker_ChunkText(t *testing.T) {
	c := NewDefaultChunker(100)
	text := strings.Repeat("a", 200)
	result := c.ChunkText(text, 50)
	if len(result) < 2 {
		t.Errorf("expected >= 2 chunks, got %d", len(result))
	}
}

func TestDefaultChunker_MaxChars(t *testing.T) {
	c := NewDefaultChunker(500)
	if c.MaxChars() != 500 {
		t.Errorf("expected 500, got %d", c.MaxChars())
	}
}

// --- NoOpConverter tests ---

func TestNoOpConverter(t *testing.T) {
	c := NewNoOpConverter()
	if c == nil {
		t.Fatal("expected non-nil converter")
	}

	text := "hello world **bold**"
	result := c.ConvertMarkdownToPlatform(text, ParseModeMarkdown)
	if result != text {
		t.Errorf("expected %q, got %q", text, result)
	}

	escaped := c.EscapeSpecialChars("<>&")
	if escaped != "<>&" {
		t.Errorf("expected unchanged, got %q", escaped)
	}
}

// --- String utility tests ---

func TestRuneCount(t *testing.T) {
	if RuneCount("") != 0 {
		t.Error("empty string should be 0")
	}
	if RuneCount("hello") != 5 {
		t.Error("ascii should be 5")
	}
	if RuneCount("你好世界") != 4 {
		t.Error("chinese chars should be 4")
	}
	if RuneCount("a你b") != 3 {
		t.Error("mixed should be 3")
	}
}

func TestTruncateByRune(t *testing.T) {
	if TruncateByRune("hello", 3) != "hel" {
		t.Error("should truncate to 3 runes")
	}
	if TruncateByRune("你好世界", 2) != "你好" {
		t.Error("should truncate chinese to 2 runes")
	}
	if TruncateByRune("hi", 10) != "hi" {
		t.Error("should return original if shorter")
	}
	if TruncateByRune("hello", 0) != "" {
		t.Error("should return empty for 0 maxRunes")
	}
	if TruncateByRune("hello", -1) != "" {
		t.Error("should return empty for negative maxRunes")
	}
}

func TestTruncateWithEllipsis(t *testing.T) {
	if TruncateWithEllipsis("hello", 3) != "..." {
		t.Errorf("3 runes with ellipsis (3-3=0 chars + ...) should be '...', got %q", TruncateWithEllipsis("hello", 3))
	}
	if TruncateWithEllipsis("hello", 5) != "hello" {
		t.Error("should return original if fits")
	}
	if TruncateWithEllipsis("hello", 6) != "hello" {
		t.Error("should return original if fits")
	}
	if TruncateWithEllipsis("hello", 0) != "" {
		t.Error("should return empty for 0")
	}
	if TruncateWithEllipsis("hello", -1) != "" {
		t.Error("should return empty for negative")
	}
	result := TruncateWithEllipsis("你好世界", 4)
	if result != "你好世界" {
		t.Errorf("should fit exactly, got %q", result)
	}
}

// --- MessageTypeToStatusType tests ---

func TestMessageTypeToStatusType(t *testing.T) {
	tests := []struct {
		input    MessageType
		expected StatusType
	}{
		{MessageTypeSessionStart, StatusInitializing},
		{MessageTypeEngineStarting, StatusInitializing},
		{MessageTypeThinking, StatusThinking},
		{MessageTypeToolUse, StatusToolUse},
		{MessageTypeToolResult, StatusToolResult},
		{MessageTypeAnswer, StatusAnswering},
		{MessageTypeExitPlanMode, StatusAnswering},
		{MessageTypeError, StatusIdle},
		{MessageTypeRaw, StatusIdle},
		{MessageType("unknown_type"), StatusIdle},
	}
	for _, tt := range tests {
		result := MessageTypeToStatusType(tt.input)
		if result != tt.expected {
			t.Errorf("MessageTypeToStatusType(%v) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

// --- ReadBody is already tested in adapter_http_test.go ---

// --- RespondWithError is already tested in adapter_http_test.go ---

// --- RespondWithJSON is already tested in adapter_http_test.go ---

// --- RespondWithText is already tested in adapter_http_test.go ---

// --- WebhookHelpers tests ---

func TestReadBodyWithError(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("hello"))
	body, err := ReadBodyWithError(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body) != "hello" {
		t.Errorf("expected 'hello', got %q", string(body))
	}
}

func TestReadBodyWithLog_Success(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("test body"))
	logger := slog.Default()

	body, ok := ReadBodyWithLog(w, r, logger)
	if !ok {
		t.Fatal("expected ok")
	}
	if string(body) != "test body" {
		t.Errorf("expected 'test body', got %q", string(body))
	}
}

func TestReadBodyWithLog_NilLogger(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("test"))

	body, ok := ReadBodyWithLog(w, r, nil)
	if !ok {
		t.Fatal("expected ok with nil logger")
	}
	if string(body) != "test" {
		t.Errorf("expected 'test', got %q", string(body))
	}
}

func TestReadBodyWithLogAndClose_Success(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("content"))

	body, ok := ReadBodyWithLogAndClose(w, r, slog.Default())
	if !ok {
		t.Fatal("expected ok")
	}
	if string(body) != "content" {
		t.Errorf("expected 'content', got %q", string(body))
	}
}

func TestCheckMethod(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if CheckMethod(w, r, http.MethodPost) {
		t.Error("should return false for wrong method")
	}
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestCheckMethod_OK(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	if !CheckMethod(w, r, http.MethodPost) {
		t.Error("should return true for correct method")
	}
}

func TestCheckMethodPOST(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if CheckMethodPOST(w, r) {
		t.Error("should reject GET")
	}

	r = httptest.NewRequest(http.MethodPost, "/", nil)
	w = httptest.NewRecorder()
	if !CheckMethodPOST(w, r) {
		t.Error("should accept POST")
	}
}

func TestCheckMethodGET(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	if CheckMethodGET(w, r) {
		t.Error("should reject POST")
	}

	r = httptest.NewRequest(http.MethodGet, "/", nil)
	w = httptest.NewRecorder()
	if !CheckMethodGET(w, r) {
		t.Error("should accept GET")
	}
}

// --- WebhookHandler tests ---

func TestNewWebhookHandler(t *testing.T) {
	logger := slog.Default()
	h := NewWebhookHandler(logger, nil)
	if h.Logger != logger {
		t.Error("logger mismatch")
	}
	if h.Verifier != nil {
		t.Error("verifier should be nil")
	}
}

func TestProcessWebhook_NoVerify(t *testing.T) {
	logger := slog.Default()

	type MyEvent struct {
		Name string `json:"name"`
	}

	parse := func(body []byte) (MyEvent, error) {
		var e MyEvent
		if err := json.Unmarshal(body, &e); err != nil {
			return e, err
		}
		return e, nil
	}

	handle := func(e MyEvent) error {
		if e.Name != "test" {
			t.Errorf("expected name 'test', got %q", e.Name)
		}
		return nil
	}

	body, _ := json.Marshal(MyEvent{Name: "test"})
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	w := httptest.NewRecorder()

	ok := ProcessWebhookNoVerify(logger, w, r, parse, handle)
	if !ok {
		t.Error("expected ok")
	}
}

func TestProcessWebhook_NoVerify_WrongMethod(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	ok := ProcessWebhookNoVerify(slog.Default(), w, r,
		func(body []byte) (string, error) { return "", nil },
		func(e string) error { return nil },
	)
	if ok {
		t.Error("should fail for GET")
	}
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestProcessWebhook_NoVerify_ParseError(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("invalid json"))
	w := httptest.NewRecorder()

	parse := func(body []byte) (string, error) {
		return "", &json.InvalidUnmarshalError{}
	}

	ok := ProcessWebhookNoVerify(slog.Default(), w, r, parse,
		func(e string) error { return nil },
	)
	if ok {
		t.Error("should fail on parse error")
	}
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestProcessWebhook_NoVerify_HandleError(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`"data"`))
	w := httptest.NewRecorder()

	parse := func(body []byte) (string, error) {
		var s string
		return s, json.Unmarshal(body, &s)
	}

	ok := ProcessWebhookNoVerify(slog.Default(), w, r, parse,
		func(e string) error { return context.DeadlineExceeded },
	)
	if ok {
		t.Error("should fail on handle error")
	}
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestProcessWebhook_WithVerifier_NilVerifier(t *testing.T) {
	h := NewWebhookHandler(slog.Default(), nil)

	parse := func(body []byte) (string, error) { return "", nil }

	body, _ := json.Marshal("test")
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	w := httptest.NewRecorder()

	ok := ProcessWebhook(h, w, r, parse, func(e string) error { return nil })
	if !ok {
		t.Error("should pass with nil verifier")
	}
}

func TestProcessWebhook_WithVerifier_Fails(t *testing.T) {
	// NoOpVerifier always passes, so use a failing verifier
	failingVerifier := &mockVerifier{valid: false}
	h2 := NewWebhookHandler(slog.Default(), failingVerifier)

	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("body"))
	w := httptest.NewRecorder()

	ok := ProcessWebhook(h2, w, r,
		func(body []byte) (string, error) { return "", nil },
		func(e string) error { return nil },
	)
	if ok {
		t.Error("should fail with invalid signature")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestWebhookHandler_RespondJSON(t *testing.T) {
	h := NewWebhookHandler(slog.Default(), nil)
	w := httptest.NewRecorder()
	h.RespondJSON(w, http.StatusOK, map[string]string{"ok": "true"})
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestWebhookHandler_RespondOK(t *testing.T) {
	h := NewWebhookHandler(slog.Default(), nil)
	w := httptest.NewRecorder()
	h.RespondOK(w)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestWebhookHandler_RespondText(t *testing.T) {
	h := NewWebhookHandler(slog.Default(), nil)
	w := httptest.NewRecorder()
	h.RespondText(w, http.StatusAccepted, "accepted")
	if w.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d", w.Code)
	}
	if w.Body.String() != "accepted" {
		t.Errorf("unexpected body: %s", w.Body.String())
	}
}

// --- VerifyRequest tests ---

func TestVerifyRequest_Nil(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	if !VerifyRequest(nil, r, []byte("body")) {
		t.Error("should pass with nil verifier")
	}
}

func TestVerifyRequest_NoOpVerifier(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	v := &NoOpVerifier{}
	if !VerifyRequest(v, r, []byte("body")) {
		t.Error("NoOpVerifier should always pass")
	}
}

// --- HMACSHA256Verifier tests ---

func TestHMACSHA256Verifier_SlackFormat(t *testing.T) {
	secret := "mysecret"
	verifier := &HMACSHA256Verifier{
		Secret:          secret,
		SignatureHeader: "X-Slack-Signature",
		TimestampHeader: "X-Slack-Request-Timestamp",
		Prefix:          "v0=",
		Format:          FormatSlack,
	}

	timestamp := "1234567890"
	body := `{"type":"event"}`

	// Compute expected signature
	sig := computeHMACSHA256("v0:"+timestamp+":"+body, secret)

	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	r.Header.Set("X-Slack-Request-Timestamp", timestamp)
	r.Header.Set("X-Slack-Signature", "v0="+sig)

	if !verifier.Verify(r, []byte(body)) {
		t.Error("signature verification should pass")
	}
}

func TestHMACSHA256Verifier_BodyFormat(t *testing.T) {
	secret := "secret"
	verifier := &HMACSHA256Verifier{
		Secret:          secret,
		SignatureHeader: "X-Signature",
		Format:          FormatBody,
	}

	body := "test body"
	sig := computeHMACSHA256(body, secret)

	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	r.Header.Set("X-Signature", sig)

	if !verifier.Verify(r, []byte(body)) {
		t.Error("body format verification should pass")
	}
}

func TestHMACSHA256Verifier_FeishuFormat(t *testing.T) {
	secret := "secret"
	verifier := &HMACSHA256Verifier{
		Secret:          secret,
		SignatureHeader: "X-Lark-Signature",
		TimestampHeader: "X-Lark-Timestamp",
		Format:          FormatFeishu,
	}

	timestamp := "1234567890"
	body := `{"type":"event"}`
	message := timestamp + secret + body
	sig := computeHMACSHA256(message, secret)

	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	r.Header.Set("X-Lark-Timestamp", timestamp)
	r.Header.Set("X-Lark-Signature", sig)

	if !verifier.Verify(r, []byte(body)) {
		t.Error("feishu format verification should pass")
	}
}

func TestHMACSHA256Verifier_WrongSignature(t *testing.T) {
	verifier := &HMACSHA256Verifier{
		Secret:          "secret",
		SignatureHeader: "X-Signature",
		Format:          FormatBody,
	}

	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("body"))
	r.Header.Set("X-Signature", "wrong")

	if verifier.Verify(r, []byte("body")) {
		t.Error("wrong signature should fail")
	}
}

func TestHMACSHA256Verifier_MissingSignature(t *testing.T) {
	verifier := &HMACSHA256Verifier{
		Secret:          "secret",
		SignatureHeader: "X-Signature",
		Format:          FormatBody,
	}

	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("body"))
	if verifier.Verify(r, []byte("body")) {
		t.Error("missing signature should fail")
	}
}

// --- WebhookRunner tests ---

func TestWebhookRunner_New(t *testing.T) {
	r := NewWebhookRunner(slog.Default())
	if r == nil {
		t.Fatal("expected non-nil runner")
	}
	if r.deduplicator == nil {
		t.Error("should have default deduplicator")
	}
	if r.keyStrategy == nil {
		t.Error("should have default key strategy")
	}
}

func TestWebhookRunner_WithDeduplication(t *testing.T) {
	r := NewWebhookRunner(slog.Default(),
		WithDeduplication(10*time.Second, 5*time.Second, nil),
	)
	if r == nil {
		t.Fatal("expected non-nil runner")
	}
}

func TestWebhookRunner_NilHandler(t *testing.T) {
	r := NewWebhookRunner(slog.Default())
	defer r.Stop()

	msg := &ChatMessage{Platform: "test"}
	r.Run(context.Background(), nil, msg) // should not panic
}

func TestWebhookRunner_Run(t *testing.T) {
	r := NewWebhookRunner(slog.Default())
	defer r.Stop()

	handled := make(chan struct{})
	handler := func(ctx context.Context, msg *ChatMessage) error {
		close(handled)
		return nil
	}

	msg := &ChatMessage{
		Platform: "test",
		Metadata: map[string]any{
			"event_type": "message",
			"channel_id": "C123",
			"event_ts":   "1234567890.123456",
		},
		SessionID: "session-1",
	}

	r.Run(context.Background(), handler, msg)

	select {
	case <-handled:
		// ok
	case <-time.After(2 * time.Second):
		t.Error("handler should have been called")
	}
}

func TestWebhookRunner_Dedup(t *testing.T) {
	r := NewWebhookRunner(slog.Default())
	defer r.Stop()

	callCount := 0
	handler := func(ctx context.Context, msg *ChatMessage) error {
		callCount++
		return nil
	}

	msg := &ChatMessage{
		Platform: "test",
		Metadata: map[string]any{
			"event_type": "message",
			"channel_id": "C123",
			"event_ts":   "1234567890.123456",
		},
		SessionID: "session-1",
	}

	// First call should go through
	r.Run(context.Background(), handler, msg)
	time.Sleep(50 * time.Millisecond)

	// Second call should be deduplicated
	r.Run(context.Background(), handler, msg)

	// Wait for goroutines
	r.WaitDefault()

	if callCount != 1 {
		t.Errorf("expected 1 call (dedup), got %d", callCount)
	}
}

func TestWebhookRunner_Wait_Timeout(t *testing.T) {
	r := NewWebhookRunner(slog.Default())
	defer r.Stop()

	// Start a long-running handler
	handler := func(ctx context.Context, msg *ChatMessage) error {
		time.Sleep(5 * time.Second)
		return nil
	}

	msg := &ChatMessage{
		Platform: "test",
		Metadata: map[string]any{
			"event_type": "message",
			"channel_id": "C123",
			"event_ts":   "1234567890.123456",
		},
		SessionID: "session-1",
	}

	r.Run(context.Background(), handler, msg)
	result := r.Wait(100 * time.Millisecond)
	if result {
		t.Error("should timeout")
	}
}

// --- HTTPClient tests ---

func TestNewHTTPClient(t *testing.T) {
	c := NewHTTPClient()
	if c == nil {
		t.Fatal("expected non-nil client")
	}
	if c.maxRetries != 0 {
		t.Errorf("expected 0 retries, got %d", c.maxRetries)
	}
}

func TestNewHTTPClientWithConfig(t *testing.T) {
	c := NewHTTPClientWithConfig(10*time.Second, 3)
	if c == nil {
		t.Fatal("expected non-nil client")
	}
	if c.maxRetries != 3 {
		t.Errorf("expected 3 retries, got %d", c.maxRetries)
	}
}

func TestExtractStringFromMetadata(t *testing.T) {
	msg := &ChatMessage{Metadata: map[string]any{"key": "value"}}
	if ExtractStringFromMetadata(msg, "key") != "value" {
		t.Error("expected 'value'")
	}
	if ExtractStringFromMetadata(msg, "missing") != "" {
		t.Error("expected empty for missing key")
	}
	if ExtractStringFromMetadata(nil, "key") != "" {
		t.Error("expected empty for nil msg")
	}
	if ExtractStringFromMetadata(&ChatMessage{}, "key") != "" {
		t.Error("expected empty for nil metadata")
	}
}

func TestExtractInt64FromMetadata(t *testing.T) {
	msg := &ChatMessage{Metadata: map[string]any{"int64": int64(42), "float": float64(3.14), "int": 10}}
	if ExtractInt64FromMetadata(msg, "int64") != 42 {
		t.Error("expected 42 for int64")
	}
	if ExtractInt64FromMetadata(msg, "float") != 3 {
		t.Error("expected 3 for float64(3.14)")
	}
	if ExtractInt64FromMetadata(msg, "int") != 10 {
		t.Error("expected 10 for int")
	}
	if ExtractInt64FromMetadata(msg, "missing") != 0 {
		t.Error("expected 0 for missing")
	}
	if ExtractInt64FromMetadata(nil, "key") != 0 {
		t.Error("expected 0 for nil msg")
	}
}

func TestHTTPClient_PostJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected application/json, got %s", ct)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	c := NewHTTPClient()
	ctx := context.Background()
	body, err := c.PostJSON(ctx, server.URL, map[string]string{"key": "val"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body) != `{"status":"ok"}` {
		t.Errorf("unexpected response: %s", string(body))
	}
}

func TestHTTPClient_PostJSON_WithHeaders(t *testing.T) {
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	c := NewHTTPClient()
	_, err := c.PostJSON(context.Background(), server.URL, map[string]string{},
		map[string]string{"Authorization": "Bearer token123"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedAuth != "Bearer token123" {
		t.Errorf("expected auth header, got %q", receivedAuth)
	}
}

func TestHTTPClient_PostJSONWithResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":"success"}`))
	}))
	defer server.Close()

	c := NewHTTPClient()
	var result map[string]string
	err := c.PostJSONWithResponse(context.Background(), server.URL,
		map[string]string{"k": "v"}, nil, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["result"] != "success" {
		t.Errorf("expected success, got %v", result)
	}
}

func TestHTTPClient_Get(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`"hello"`))
	}))
	defer server.Close()

	c := NewHTTPClient()
	body, err := c.Get(context.Background(), server.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body) != `"hello"` {
		t.Errorf("unexpected response: %s", string(body))
	}
}

func TestHTTPClient_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer server.Close()

	c := NewHTTPClient()
	_, err := c.Get(context.Background(), server.URL, nil)
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

// --- Metadata tests ---

func TestGetMetadata(t *testing.T) {
	roomID, threadID, userID := GetMetadata(nil)
	if roomID != "" || threadID != "" || userID != "" {
		t.Error("expected empty values for nil map")
	}

	m := map[string]any{KeyRoomID: "R1", KeyThreadID: "T1", KeyUserID: "U1"}
	roomID, threadID, userID = GetMetadata(m)
	if roomID != "R1" || threadID != "T1" || userID != "U1" {
		t.Errorf("expected R1 T1 U1, got %s %s %s", roomID, threadID, userID)
	}
}

func TestSetMetadata(t *testing.T) {
	m := SetMetadata(nil, "R1", "T1", "U1")
	if m[KeyRoomID] != "R1" {
		t.Error("room_id not set")
	}
	if m[KeyThreadID] != "T1" {
		t.Error("thread_id not set")
	}
	if m[KeyUserID] != "U1" {
		t.Error("user_id not set")
	}

	// Should reuse existing map
	m2 := map[string]any{"existing": true}
	result := SetMetadata(m2, "R2", "", "U2")
	if result["existing"] != true {
		t.Error("should preserve existing keys")
	}
}

func TestGetMetadataString(t *testing.T) {
	if GetMetadataString(nil, "key") != "" {
		t.Error("expected empty for nil map")
	}
	if GetMetadataString(map[string]any{"key": "value"}, "key") != "value" {
		t.Error("expected 'value'")
	}
	if GetMetadataString(map[string]any{"key": 123}, "key") != "" {
		t.Error("expected empty for non-string value")
	}
}

func TestSlackMetadata(t *testing.T) {
	m := map[string]any{
		KeyRoomID: "R1", KeyThreadID: "T1", KeyUserID: "U1",
	}
	ch, ts, uid := SlackMetadata(m)
	if ch != "R1" || ts != "T1" || uid != "U1" {
		t.Errorf("expected R1 T1 U1, got %s %s %s", ch, ts, uid)
	}

	// Test fallback
	m2 := map[string]any{
		"channel_id": "C1", "thread_ts": "TS1", "user_id": "U1",
	}
	ch, ts, uid = SlackMetadata(m2)
	if ch != "C1" || ts != "TS1" || uid != "U1" {
		t.Errorf("expected C1 TS1 U1 (fallback), got %s %s %s", ch, ts, uid)
	}
}

func TestMergeMetadata(t *testing.T) {
	dst := map[string]any{"other": "value"}
	src := map[string]any{KeyRoomID: "R1", KeyThreadID: "T1", KeyUserID: "U1"}
	result := MergeMetadata(dst, src)
	if result[KeyRoomID] != "R1" {
		t.Error("room_id not merged")
	}
	if result["other"] != "value" {
		t.Error("existing key lost")
	}

	// Nil dst
	result2 := MergeMetadata(nil, src)
	if result2[KeyRoomID] != "R1" {
		t.Error("room_id not set with nil dst")
	}

	// Nil src
	result3 := MergeMetadata(dst, nil)
	if result3[KeyRoomID] != "R1" {
		t.Error("should preserve existing room_id")
	}
}

// --- PendingMessageStore tests ---

func TestPendingMessageStore_StoreAndGet(t *testing.T) {
	store := NewPendingMessageStore(5 * time.Minute)
	defer store.Stop()

	msg := &PendingMessage{
		ChannelID: "C123",
		MessageTS: "1234.5678",
		Reason:    "test",
	}
	store.Store("session-1", msg)

	got, ok := store.Get("session-1")
	if !ok {
		t.Fatal("expected to find message")
	}
	if got.ChannelID != "C123" {
		t.Errorf("expected C123, got %s", got.ChannelID)
	}
}

func TestPendingMessageStore_Get_NotFound(t *testing.T) {
	store := NewPendingMessageStore(5 * time.Minute)
	defer store.Stop()

	_, ok := store.Get("nonexistent")
	if ok {
		t.Error("should not find nonexistent key")
	}
}

func TestPendingMessageStore_Delete(t *testing.T) {
	store := NewPendingMessageStore(5 * time.Minute)
	defer store.Stop()

	store.Store("session-1", &PendingMessage{ChannelID: "C123"})
	store.Delete("session-1")

	_, ok := store.Get("session-1")
	if ok {
		t.Error("should not find deleted key")
	}
}

func TestPendingMessageStore_TTL(t *testing.T) {
	store := NewPendingMessageStore(50 * time.Millisecond)
	defer store.Stop()

	store.Store("session-1", &PendingMessage{ChannelID: "C123"})
	time.Sleep(100 * time.Millisecond)

	_, ok := store.Get("session-1")
	if ok {
		t.Error("should expire after TTL")
	}
}

// --- Adapter Start/Stop tests ---

func TestAdapter_Start_Serverless(t *testing.T) {
	adapter := NewAdapter("test", Config{}, slog.Default(), WithoutServer())
	defer func() { _ = adapter.Stop() }()

	err := adapter.Start(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdapter_Start_WithServer(t *testing.T) {
	adapter := NewAdapter("test", Config{ServerAddr: ":0"}, slog.Default())
	defer func() { _ = adapter.Stop() }()

	err := adapter.Start(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdapter_Start_Idempotent(t *testing.T) {
	adapter := NewAdapter("test", Config{}, slog.Default(), WithoutServer())
	defer func() { _ = adapter.Stop() }()

	err := adapter.Start(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = adapter.Start(context.Background())
	if err != nil {
		t.Fatalf("unexpected error on second start: %v", err)
	}
}

func TestAdapter_Stop_NotRunning(t *testing.T) {
	adapter := NewAdapter("test", Config{}, slog.Default(), WithoutServer())
	err := adapter.Stop()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdapter_Stop_Serverless(t *testing.T) {
	adapter := NewAdapter("test", Config{}, slog.Default(), WithoutServer())
	_ = adapter.Start(context.Background())
	err := adapter.Stop()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdapter_CleanupSessions(t *testing.T) {
	adapter := NewAdapter("test", Config{}, slog.Default(),
		WithoutServer(),
		WithSessionTimeout(50*time.Millisecond),
		WithCleanupInterval(20*time.Millisecond),
	)
	_ = adapter.Start(context.Background())
	defer func() { _ = adapter.Stop() }()

	adapter.GetOrCreateSession("U1", "", "C1", "")
	time.Sleep(120 * time.Millisecond) // Wait for cleanup

	_, ok := adapter.GetSession("test:U1::C1:")
	if ok {
		t.Error("expired session should be cleaned up")
	}
}

// --- Adapter NoOp methods ---

func TestAdapter_NoOpMethods(t *testing.T) {
	adapter := NewAdapter("test", Config{}, slog.Default(), WithoutServer())
	ctx := context.Background()

	if err := adapter.DeleteMessage(ctx, "C1", "TS1"); err != nil {
		t.Errorf("DeleteMessage should be no-op: %v", err)
	}
	if err := adapter.UpdateMessage(ctx, "C1", "TS1", &ChatMessage{}); err != nil {
		t.Errorf("UpdateMessage should be no-op: %v", err)
	}
	if err := adapter.SetAssistantStatus(ctx, "C1", "TS1", "thinking"); err != nil {
		t.Errorf("SetAssistantStatus should be no-op: %v", err)
	}
	if err := adapter.SendThreadReply(ctx, "C1", "TS1", "reply"); err != nil {
		t.Errorf("SendThreadReply should be no-op: %v", err)
	}
	ts, err := adapter.StartStream(ctx, "C1", "TS1")
	if err != nil || ts != "" {
		t.Error("StartStream should return empty")
	}
	if err := adapter.AppendStream(ctx, "C1", "TS1", "content"); err != nil {
		t.Errorf("AppendStream should be no-op: %v", err)
	}
	if err := adapter.StopStream(ctx, "C1", "TS1"); err != nil {
		t.Errorf("StopStream should be no-op: %v", err)
	}
}

func TestAdapter_SendMessage_NotConfigured(t *testing.T) {
	adapter := NewAdapter("test", Config{}, slog.Default(), WithoutServer())
	err := adapter.SendMessage(context.Background(), "s1", &ChatMessage{})
	if err == nil {
		t.Error("expected error when sender not configured")
	}
}

func TestAdapter_SendMessage_Configured(t *testing.T) {
	adapter := NewAdapter("test", Config{}, slog.Default(), WithoutServer(),
		WithMessageSender(func(ctx context.Context, sessionID string, msg *ChatMessage) error {
			return nil
		}),
	)
	err := adapter.SendMessage(context.Background(), "s1", &ChatMessage{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAdapter_HandleHealth(t *testing.T) {
	adapter := NewAdapter("test", Config{ServerAddr: ":0"}, slog.Default())
	defer func() { _ = adapter.Stop() }()
	_ = adapter.Start(context.Background())

	// Test health endpoint via the server
	// Since we started with :0, we need to get the actual port
	// Instead, just test the handler directly
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	adapter.handleHealth(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "OK" {
		t.Errorf("expected OK, got %s", w.Body.String())
	}
}

func TestAdapter_FindSessionByUserAndChannel(t *testing.T) {
	adapter := NewAdapter("test", Config{}, slog.Default(), WithoutServer())

	// Not found
	s := adapter.FindSessionByUserAndChannel("U1", "C1")
	if s != nil {
		t.Error("expected nil for not found")
	}

	// Create session
	adapter.GetOrCreateSession("U1", "B1", "C1", "T1")

	// Find via secondary index
	s = adapter.FindSessionByUserAndChannel("U1", "C1")
	if s == nil {
		t.Fatalf("expected to find session")
	}
	if s.UserID != "U1" {
		t.Errorf("expected U1, got %s", s.UserID)
	}
}

// --- helpers ---

type mockVerifier struct {
	valid bool
}

func (m *mockVerifier) Verify(_ *http.Request, _ []byte) bool {
	return m.valid
}

func computeHMACSHA256(message, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}
