package base

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestWebhookPath tests WebhookPath method
func TestWebhookPath(t *testing.T) {
	// Empty handlers
	adapter := &Adapter{
		httpHandlers: make(map[string]http.HandlerFunc),
	}
	if path := adapter.WebhookPath(); path != "" {
		t.Errorf("WebhookPath(): got %q, want empty", path)
	}

	// With handler
	adapter.httpHandlers["/webhook"] = func(w http.ResponseWriter, r *http.Request) {}
	if path := adapter.WebhookPath(); path != "/webhook" {
		t.Errorf("WebhookPath(): got %q, want /webhook", path)
	}
}

// TestWebhookHandler tests WebhookHandler method
func TestWebhookHandler(t *testing.T) {
	adapter := &Adapter{
		httpHandlers: map[string]http.HandlerFunc{
			"/webhook": func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
		},
	}

	handler := adapter.WebhookHandler()
	if handler == nil {
		t.Fatal("WebhookHandler(): got nil")
	}

	req := httptest.NewRequest("POST", "/webhook", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ServeHTTP: got status %d, want %d", rec.Code, http.StatusOK)
	}
}

// TestHandleHealth tests the health endpoint
func TestHandleHealth(t *testing.T) {
	adapter := &Adapter{}

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()

	adapter.handleHealth(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("handleHealth: got status %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "OK" {
		t.Errorf("handleHealth: got body %q, want OK", rec.Body.String())
	}
}

// TestReadBody tests ReadBody helper
func TestReadBody(t *testing.T) {
	// Normal case
	body := []byte(`{"test": "data"}`)
	req := httptest.NewRequest("POST", "/", bytes.NewReader(body))

	got, err := ReadBody(req)
	if err != nil {
		t.Fatalf("ReadBody: unexpected error: %v", err)
	}
	if string(got) != string(body) {
		t.Errorf("ReadBody: got %q, want %q", got, body)
	}
}

// TestRespondWithJSON tests JSON response helper
func TestRespondWithJSON(t *testing.T) {
	rec := httptest.NewRecorder()

	data := map[string]string{"status": "ok"}
	err := RespondWithJSON(rec, http.StatusOK, data)
	if err != nil {
		t.Fatalf("RespondWithJSON: unexpected error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("RespondWithJSON: got status %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("RespondWithJSON: got Content-Type %q, want application/json", ct)
	}
	expected := `{"status":"ok"}` + "\n"
	if rec.Body.String() != expected {
		t.Errorf("RespondWithJSON: got body %q, want %q", rec.Body.String(), expected)
	}
}

// TestRespondWithText tests text response helper
func TestRespondWithText(t *testing.T) {
	rec := httptest.NewRecorder()

	RespondWithText(rec, http.StatusOK, "hello")

	if rec.Code != http.StatusOK {
		t.Errorf("RespondWithText: got status %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "hello" {
		t.Errorf("RespondWithText: got body %q, want hello", rec.Body.String())
	}
}

// TestRespondWithError tests error response helper
func TestRespondWithError(t *testing.T) {
	rec := httptest.NewRecorder()

	RespondWithError(rec, http.StatusBadRequest, "invalid request")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("RespondWithError: got status %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// TestSendMessage tests SendMessage with nil sender
func TestSendMessage(t *testing.T) {
	adapter := &Adapter{
		messageSender: nil,
	}

	err := adapter.SendMessage(context.Background(), "session-1", &ChatMessage{Content: "test"})
	if err == nil {
		t.Error("SendMessage: got nil error, want error for nil sender")
	}
}

// TestHandleMessage tests HandleMessage (default implementation returns nil)
func TestHandleMessage(t *testing.T) {
	adapter := &Adapter{}

	err := adapter.HandleMessage(context.Background(), &ChatMessage{Content: "test"})
	if err != nil {
		t.Errorf("HandleMessage: got error %v, want nil", err)
	}
}

// TestReadBodyError tests ReadBody with read error
func TestReadBodyError(t *testing.T) {
	// Create request with error reader
	req := httptest.NewRequest("POST", "/", &errorReader{})

	_, err := ReadBody(req)
	if err == nil {
		t.Error("ReadBody: got nil error, want error for failed read")
	}
}

// errorReader always returns an error
type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, io.ErrUnexpectedEOF
}
