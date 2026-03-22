package cron

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestJobTypeValues(t *testing.T) {
	tests := []struct {
		name  string
		value JobType
	}{
		{"light", JobTypeLight},
		{"medium", JobTypeMedium},
		{"resource-intensive", JobTypeResourceIntensive},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value == "" {
				t.Error("JobType should not be empty")
			}
		})
	}
}

func TestOutputFormatValues(t *testing.T) {
	tests := []struct {
		name  string
		value OutputFormat
	}{
		{"text", OutputFormatText},
		{"json", OutputFormatJSON},
		{"structured", OutputFormatStructured},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value == "" {
				t.Error("OutputFormat should not be empty")
			}
		})
	}
}

func TestEventValues(t *testing.T) {
	tests := []struct {
		name  string
		value Event
	}{
		{"completed", EventCompleted},
		{"failed", EventFailed},
		{"canceled", EventCanceled},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value == "" {
				t.Error("Event should not be empty")
			}
		})
	}
}

func TestCronJobJSON(t *testing.T) {
	job := &CronJob{
		ID:           "test-123",
		CronExpr:     "*/5 * * * *",
		Prompt:       "Run tests",
		WorkDir:      "/tmp",
		Type:         JobTypeLight,
		TimeoutMins:  30,
		Retries:      3,
		RetryDelay:   5 * time.Second,
		OutputFormat: OutputFormatText,
		Enabled:      true,
		Silent:       false,
		NotifyOn:     []Event{EventCompleted, EventFailed},
		CreatedBy:    "user1",
		CreatedAt:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		LastRun:      time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		NextRun:      time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC),
		RunCount:     10,
	}

	data, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded CronJob
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ID != job.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, job.ID)
	}
	if decoded.CronExpr != job.CronExpr {
		t.Errorf("CronExpr = %q, want %q", decoded.CronExpr, job.CronExpr)
	}
	if decoded.Prompt != job.Prompt {
		t.Errorf("Prompt = %q, want %q", decoded.Prompt, job.Prompt)
	}
	if decoded.Type != job.Type {
		t.Errorf("Type = %q, want %q", decoded.Type, job.Type)
	}
	if decoded.TimeoutMins != job.TimeoutMins {
		t.Errorf("TimeoutMins = %d, want %d", decoded.TimeoutMins, job.TimeoutMins)
	}
	if decoded.Retries != job.Retries {
		t.Errorf("Retries = %d, want %d", decoded.Retries, job.Retries)
	}
	if decoded.Enabled != job.Enabled {
		t.Errorf("Enabled = %v, want %v", decoded.Enabled, job.Enabled)
	}
	if decoded.RunCount != job.RunCount {
		t.Errorf("RunCount = %d, want %d", decoded.RunCount, job.RunCount)
	}
	if len(decoded.NotifyOn) != len(job.NotifyOn) {
		t.Errorf("NotifyOn len = %d, want %d", len(decoded.NotifyOn), len(job.NotifyOn))
	}
}

func TestCronRunJSON(t *testing.T) {
	now := time.Now()
	run := &CronRun{
		ID:         "run-456",
		JobID:      "job-123",
		StartedAt:  now,
		FinishedAt: now.Add(5 * time.Second),
		Duration:   5 * time.Second,
		Status:     string(EventCompleted),
		Error:      "",
		RetryCount: 2,
		Response:   "task completed",
	}

	data, err := json.Marshal(run)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded CronRun
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ID != run.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, run.ID)
	}
	if decoded.JobID != run.JobID {
		t.Errorf("JobID = %q, want %q", decoded.JobID, run.JobID)
	}
	if decoded.Status != run.Status {
		t.Errorf("Status = %q, want %q", decoded.Status, run.Status)
	}
	if decoded.RetryCount != run.RetryCount {
		t.Errorf("RetryCount = %d, want %d", decoded.RetryCount, run.RetryCount)
	}
	if decoded.Response != run.Response {
		t.Errorf("Response = %q, want %q", decoded.Response, run.Response)
	}
}

func TestCronJobOptionalFields(t *testing.T) {
	// Test that omitempty fields behave correctly
	job := &CronJob{
		ID:       "minimal",
		CronExpr: "* * * * *",
		Prompt:   "test",
	}

	data, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}

	// omitempty fields should be absent when zero-valued
	if _, ok := raw["session_key"]; ok {
		t.Error("session_key should be omitted when empty")
	}
	if _, ok := raw["work_dir"]; ok {
		t.Error("work_dir should be omitted when empty")
	}
	if _, ok := raw["output_schema"]; ok {
		t.Error("output_schema should be omitted when empty")
	}
	if _, ok := raw["last_error"]; ok {
		t.Error("last_error should be omitted when empty")
	}
	if _, ok := raw["on_complete"]; ok {
		t.Error("on_complete should be omitted when empty")
	}
	if _, ok := raw["on_fail"]; ok {
		t.Error("on_fail should be omitted when empty")
	}
}

func TestCronRunOptionalFields(t *testing.T) {
	run := &CronRun{
		ID:     "run-minimal",
		JobID:  "job-1",
		Status: "completed",
	}

	data, err := json.Marshal(run)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}

	if _, ok := raw["error"]; ok {
		t.Error("error should be omitted when empty")
	}
	if _, ok := raw["response"]; ok {
		t.Error("response should be omitted when empty")
	}
}

// ---------------------------------------------------------------------------
// WebhookCallback tests
// ---------------------------------------------------------------------------

func TestWebhookCallback_EmptyURL(t *testing.T) {
	wc := &WebhookCallback{}
	run := &CronRun{ID: "run-1", Status: "completed"}

	if err := wc.OnComplete(run); err != nil {
		t.Errorf("OnComplete with empty URL should return nil, got: %v", err)
	}
	if err := wc.OnFail(run); err != nil {
		t.Errorf("OnFail with empty URL should return nil, got: %v", err)
	}
}

func TestWebhookCallback_Success(t *testing.T) {
	var receivedBody string
	var receivedHeaders http.Header
	var receivedMethod string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedHeaders = r.Header
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		receivedBody = string(buf[:n])
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	wc := &WebhookCallback{
		URL:     server.URL,
		Token:   "test-token",
		Timeout: 5 * time.Second,
		Retry:   0,
	}

	run := &CronRun{
		ID:     "run-abc",
		JobID:  "job-123",
		Status: "completed",
	}

	if err := wc.OnComplete(run); err != nil {
		t.Fatalf("OnComplete: %v", err)
	}

	if receivedMethod != http.MethodPost {
		t.Errorf("method = %q, want %q", receivedMethod, http.MethodPost)
	}
	if ct := receivedHeaders.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
	if auth := receivedHeaders.Get("Authorization"); auth != "Bearer test-token" {
		t.Errorf("Authorization = %q, want %q", auth, "Bearer test-token")
	}
	if runID := receivedHeaders.Get("X-CronRun-ID"); runID != "run-abc" {
		t.Errorf("X-CronRun-ID = %q, want %q", runID, "run-abc")
	}
	if event := receivedHeaders.Get("X-CronEvent"); event != "completed" {
		t.Errorf("X-CronEvent = %q, want %q", event, "completed")
	}

	// Verify body is valid JSON containing run data
	var parsed map[string]any
	if err := json.Unmarshal([]byte(receivedBody), &parsed); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	if parsed["id"] != "run-abc" {
		t.Errorf("body id = %v, want %q", parsed["id"], "run-abc")
	}
}

func TestWebhookCallback_ServerError_Retries(t *testing.T) {
	attempts := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	wc := &WebhookCallback{
		URL:     server.URL,
		Timeout: 2 * time.Second,
		Retry:   1,
	}

	run := &CronRun{ID: "run-retry", Status: "failed"}

	err := wc.send(run)
	if err == nil {
		t.Fatal("expected error after retries, got nil")
	}

	// First attempt + 1 retry = 2 total
	if attempts != 2 {
		t.Errorf("attempts = %d, want 2", attempts)
	}
}

func TestWebhookCallback_EventBasedRouting(t *testing.T) {
	completeCalled := false
	failCalled := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-CronEvent") == "completed" {
			completeCalled = true
		}
		if r.Header.Get("X-CronEvent") == "failed" {
			failCalled = true
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	wc := &WebhookCallback{URL: server.URL, Retry: 0}

	// Test OnComplete
	if err := wc.OnComplete(&CronRun{ID: "r1", Status: "completed"}); err != nil {
		t.Fatalf("OnComplete: %v", err)
	}
	if !completeCalled {
		t.Error("OnComplete should have sent completed event")
	}

	// Test OnFail
	if err := wc.OnFail(&CronRun{ID: "r2", Status: "failed"}); err != nil {
		t.Fatalf("OnFail: %v", err)
	}
	if !failCalled {
		t.Error("OnFail should have sent failed event")
	}
}

func TestWebhookCallback_NoToken_NoAuthHeader(t *testing.T) {
	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	wc := &WebhookCallback{URL: server.URL, Retry: 0}
	if err := wc.send(&CronRun{ID: "r", Status: "completed"}); err != nil {
		t.Fatalf("send: %v", err)
	}

	if receivedHeaders.Get("Authorization") != "" {
		t.Error("Authorization header should not be set when Token is empty")
	}
}

// ---------------------------------------------------------------------------
// Clone
// ---------------------------------------------------------------------------

func TestCronJob_Clone_Nil(t *testing.T) {
	var job *CronJob
	clone := job.Clone()
	if clone != nil {
		t.Error("Clone(nil) should return nil")
	}
}

func TestCronJob_Clone_WithNotifyOn(t *testing.T) {
	job := &CronJob{
		ID:       "clone-test",
		CronExpr: "* * * * *",
		Prompt:   "test",
		NotifyOn: []Event{EventCompleted, EventFailed},
	}

	clone := job.Clone()
	if clone == nil {
		t.Fatal("Clone should not return nil")
	}
	if clone.ID != job.ID {
		t.Errorf("ID = %q, want %q", clone.ID, job.ID)
	}
	if len(clone.NotifyOn) != len(job.NotifyOn) {
		t.Errorf("NotifyOn len = %d, want %d", len(clone.NotifyOn), len(job.NotifyOn))
	}

	// Verify deep copy: modifying clone should not affect original
	clone.NotifyOn[0] = EventCanceled
	if job.NotifyOn[0] != EventCompleted {
		t.Error("modifying clone.NotifyOn should not affect original")
	}
}

func TestCronJob_Clone_EmptyNotifyOn(t *testing.T) {
	job := &CronJob{ID: "empty-notify", CronExpr: "* * * * *"}

	clone := job.Clone()
	if clone == nil {
		t.Fatal("Clone should not return nil")
	}
	if len(clone.NotifyOn) != 0 {
		t.Errorf("NotifyOn len = %d, want 0", len(clone.NotifyOn))
	}
}

// ---------------------------------------------------------------------------
// WebhookCallback - additional coverage
// ---------------------------------------------------------------------------

func TestWebhookCallback_Send_ClientError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	wc := &WebhookCallback{
		URL:     server.URL,
		Timeout: 2 * time.Second,
		Retry:   0,
	}

	err := wc.send(&CronRun{ID: "r-client-err", Status: "failed"})
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
}

func TestWebhookCallback_Send_AllRetriesExhausted(t *testing.T) {
	attempts := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	wc := &WebhookCallback{
		URL:     server.URL,
		Timeout: 2 * time.Second,
		Retry:   2,
	}

	err := wc.send(&CronRun{ID: "r-all-retry", Status: "failed"})
	if err == nil {
		t.Fatal("expected error after all retries")
	}
	// First attempt + 2 retries = 3 total
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}

func TestWebhookCallback_DefaultTimeout(t *testing.T) {
	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// No timeout set - should default to 10s
	wc := &WebhookCallback{URL: server.URL}
	if err := wc.send(&CronRun{ID: "r-default-timeout", Status: "completed"}); err != nil {
		t.Fatalf("send: %v", err)
	}

	if ct := receivedHeaders.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
	if accept := receivedHeaders.Get("Accept"); accept != "application/json" {
		t.Errorf("Accept = %q, want %q", accept, "application/json")
	}
}

func TestWebhookCallback_RequestHeaders(t *testing.T) {
	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	run := &CronRun{ID: "run-headers", Status: "completed"}
	wc := &WebhookCallback{URL: server.URL}
	if err := wc.send(run); err != nil {
		t.Fatalf("send: %v", err)
	}

	if runID := receivedHeaders.Get("X-CronRun-ID"); runID != "run-headers" {
		t.Errorf("X-CronRun-ID = %q, want %q", runID, "run-headers")
	}
	if event := receivedHeaders.Get("X-CronEvent"); event != "completed" {
		t.Errorf("X-CronEvent = %q, want %q", event, "completed")
	}
}
