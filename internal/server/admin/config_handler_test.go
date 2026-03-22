package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hrygo/hotplex/engine"
)

func TestGetConfig_EngineNil(t *testing.T) {
	handler := NewConfigHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/config", nil)
	rec := httptest.NewRecorder()
	handler.getConfig(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rec.Code)
	}
}

func TestGetAllowedTools_EngineNil(t *testing.T) {
	handler := NewConfigHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/config/allowed_tools", nil)
	rec := httptest.NewRecorder()
	handler.getAllowedTools(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rec.Code)
	}
}

func TestGetDisallowedTools_EngineNil(t *testing.T) {
	handler := NewConfigHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/config/disallowed_tools", nil)
	rec := httptest.NewRecorder()
	handler.getDisallowedTools(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rec.Code)
	}
}

func TestGetConfig_ResponseFormat(t *testing.T) {
	handler := NewConfigHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/config", nil)
	rec := httptest.NewRecorder()
	handler.getConfig(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503 for nil engine, got %d", rec.Code)
	}

	var resp AdminError
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error.Code != "ENGINE_NOT_INITIALIZED" {
		t.Errorf("expected error code ENGINE_NOT_INITIALIZED, got %s", resp.Error.Code)
	}
}

var _ = engine.Engine{}
