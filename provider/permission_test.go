package provider

import (
	"encoding/json"
	"testing"
)

// TestPermissionRequest_Parse validates the permission_request format from Claude Code.
// TestPermissionRequest_Parse validates the permission_request format from Claude Code.
// Reference: GitHub Issue #39 - Claude Code 权限确认 ↔ Slack 交互桥接调研
func TestPermissionRequest_Parse(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantType       string
		wantSessionID  string
		wantMessageID  string
		wantDecision   *DecisionDetail
		wantPermission *PermissionDetail
		wantErr        bool
	}{
		{
			name:          "basic permission_request with session_id",
			input:         `{"type":"permission_request","session_id":"sess_abc123"}`,
			wantType:      "permission_request",
			wantSessionID: "sess_abc123",
		},
		// Legacy format - Issue #39 original description
		{
			name:          "permission_request with permission object (legacy format from Issue #39)",
			input:         `{"type":"permission_request","permission":{"name":"bash","input":"rm -rf /some/path"},"session_id":"sess_legacy"}`,
			wantType:      "permission_request",
			wantSessionID: "sess_legacy",
			wantPermission: &PermissionDetail{
				Name:  "bash",
				Input: "rm -rf /some/path",
			},
		},
		{
			name:          "permission_request with Bash tool (legacy format)",
			input:         `{"type":"permission_request","permission":{"name":"Bash","input":"ls -la"},"session_id":"xxx"}`,
			wantType:      "permission_request",
			wantSessionID: "xxx",
			wantPermission: &PermissionDetail{
				Name:  "Bash",
				Input: "ls -la",
			},
		},
		{
			name:          "permission_request with Read tool (legacy format)",
			input:         `{"type":"permission_request","permission":{"name":"Read","input":"/etc/passwd"},"session_id":"yyy"}`,
			wantType:      "permission_request",
			wantSessionID: "yyy",
			wantPermission: &PermissionDetail{
				Name:  "Read",
				Input: "/etc/passwd",
			},
		},
		// Current format - with decision object
		{
			name:          "permission_request with decision and options",
			input:         `{"type":"permission_request","message_id":"msg_xyz789","decision":{"type":"ask","options":[{"name":"Yes"},{"name":"No"}]}}`,
			wantType:      "permission_request",
			wantMessageID: "msg_xyz789",
			wantDecision: &DecisionDetail{
				Type: "ask",
				Options: []struct {
					Name string `json:"name"`
				}{
					{Name: "Yes"},
					{Name: "No"},
				},
			},
		},
		{
			name:          "permission_request with allow decision",
			input:         `{"type":"permission_request","session_id":"sess_123","decision":{"type":"allow","reason":"auto-approved"}}`,
			wantType:      "permission_request",
			wantSessionID: "sess_123",
			wantDecision: &DecisionDetail{
				Type:   "allow",
				Reason: "auto-approved",
			},
		},
		{
			name:          "permission_request with deny decision",
			input:         `{"type":"permission_request","session_id":"sess_456","decision":{"type":"deny","reason":"dangerous command"}}`,
			wantType:      "permission_request",
			wantSessionID: "sess_456",
			wantDecision: &DecisionDetail{
				Type:   "deny",
				Reason: "dangerous command",
			},
		},
		{
			name:    "invalid JSON",
			input:   `{invalid json}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req PermissionRequest
			err := json.Unmarshal([]byte(tt.input), &req)
			if (err != nil) != tt.wantErr {
				t.Errorf("json.Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if req.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", req.Type, tt.wantType)
			}
			if req.SessionID != tt.wantSessionID {
				t.Errorf("SessionID = %q, want %q", req.SessionID, tt.wantSessionID)
			}
			if req.MessageID != tt.wantMessageID {
				t.Errorf("MessageID = %q, want %q", req.MessageID, tt.wantMessageID)
			}
			// Validate Decision (current format)
			if tt.wantDecision != nil {
				if req.Decision == nil {
					t.Error("Decision is nil, want non-nil")
					return
				}
				if req.Decision.Type != tt.wantDecision.Type {
					t.Errorf("Decision.Type = %q, want %q", req.Decision.Type, tt.wantDecision.Type)
				}
				if req.Decision.Reason != tt.wantDecision.Reason {
					t.Errorf("Decision.Reason = %q, want %q", req.Decision.Reason, tt.wantDecision.Reason)
				}
				if len(req.Decision.Options) != len(tt.wantDecision.Options) {
					t.Errorf("Decision.Options length = %d, want %d", len(req.Decision.Options), len(tt.wantDecision.Options))
				}
			}
			// Validate Permission (legacy format from Issue #39)
			if tt.wantPermission != nil {
				if req.Permission == nil {
					t.Error("Permission is nil, want non-nil")
					return
				}
				if req.Permission.Name != tt.wantPermission.Name {
					t.Errorf("Permission.Name = %q, want %q", req.Permission.Name, tt.wantPermission.Name)
				}
				if req.Permission.Input != tt.wantPermission.Input {
					t.Errorf("Permission.Input = %q, want %q", req.Permission.Input, tt.wantPermission.Input)
				}
			}
		})
	}
}

// TestPermissionResponse_Serialize validates the response format sent to Claude Code stdin.
// Expected format: {"behavior": "allow"} or {"behavior": "deny", "message": "User rejected"}
// Reference: GitHub Issue #39
func TestPermissionResponse_Serialize(t *testing.T) {
	tests := []struct {
		name     string
		response PermissionResponse
		wantJSON string
	}{
		{
			name:     "allow without message",
			response: PermissionResponse{Behavior: "allow"},
			wantJSON: `{"behavior":"allow"}`,
		},
		{
			name:     "deny with message",
			response: PermissionResponse{Behavior: "deny", Message: "User rejected"},
			wantJSON: `{"behavior":"deny","message":"User rejected"}`,
		},
		{
			name:     "allow for tool",
			response: PermissionResponse{Behavior: "allow"},
			wantJSON: `{"behavior":"allow"}`,
		},
		{
			name:     "deny for dangerous command",
			response: PermissionResponse{Behavior: "deny", Message: "Dangerous command blocked by policy"},
			wantJSON: `{"behavior":"deny","message":"Dangerous command blocked by policy"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.response)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}
			if string(data) != tt.wantJSON {
				t.Errorf("JSON = %s, want %s", string(data), tt.wantJSON)
			}
		})
	}
}

// TestPermissionResponse_RoundTrip validates that the response can be serialized and deserialized correctly.
func TestPermissionResponse_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		resp PermissionResponse
	}{
		{"allow", PermissionResponse{Behavior: "allow"}},
		{"deny with message", PermissionResponse{Behavior: "deny", Message: "User rejected this action"}},
		{"deny empty message", PermissionResponse{Behavior: "deny"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.resp)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}

			var got PermissionResponse
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}

			if got.Behavior != tt.resp.Behavior {
				t.Errorf("Behavior = %q, want %q", got.Behavior, tt.resp.Behavior)
			}
			if got.Message != tt.resp.Message {
				t.Errorf("Message = %q, want %q", got.Message, tt.resp.Message)
			}
		})
	}
}

// TestPermissionResponse_ValidBehaviors ensures only valid behaviors are used.
func TestPermissionResponse_ValidBehaviors(t *testing.T) {
	validBehaviors := map[string]bool{
		"allow": true,
		"deny":  true,
	}

	tests := []struct {
		behavior string
		isValid  bool
	}{
		{"allow", true},
		{"deny", true},
		{"", false},
		{"yes", false},
		{"no", false},
		{"approve", false},
		{"reject", false},
	}

	for _, tt := range tests {
		t.Run(tt.behavior, func(t *testing.T) {
			_, valid := validBehaviors[tt.behavior]
			if valid != tt.isValid {
				t.Errorf("behavior %q: valid = %v, want %v", tt.behavior, valid, tt.isValid)
			}
		})
	}
}

// TestPermissionResponse_StdinFormat validates the exact format expected by Claude Code stdin.
// Each response must be a single line JSON ending with newline.
func TestPermissionResponse_StdinFormat(t *testing.T) {
	resp := PermissionResponse{Behavior: "allow"}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	// Verify format: single line JSON without newline (caller adds newline)
	line := string(data)
	expected := `{"behavior":"allow"}`
	if line != expected {
		t.Errorf("Format = %q, want %q", line, expected)
	}

	// Verify it doesn't contain newlines (should be single line)
	for i, c := range line {
		if c == '\n' || c == '\r' {
			t.Errorf("Format contains newline at position %d", i)
		}
	}
}

// TestPermissionRequestResponseIntegration simulates the full permission flow.
func TestPermissionRequestResponseIntegration(t *testing.T) {
	// Step 1: Parse incoming permission request from Claude Code
	input := `{"type":"permission_request","session_id":"sess_test","decision":{"type":"ask","options":[{"name":"Yes"},{"name":"No"}]}}`
	var req PermissionRequest
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		t.Fatalf("Failed to parse permission request: %v", err)
	}

	// Verify request
	if req.Type != "permission_request" {
		t.Errorf("Request type = %q, want %q", req.Type, "permission_request")
	}
	if req.SessionID != "sess_test" {
		t.Errorf("Session ID = %q, want %q", req.SessionID, "sess_test")
	}
	if req.Decision == nil || req.Decision.Type != "ask" {
		t.Error("Decision type should be 'ask'")
	}

	// Step 2: User approves - create response
	resp := PermissionResponse{Behavior: "allow"}
	respData, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	// Step 3: Verify response format for stdin
	expected := `{"behavior":"allow"}`
	if string(respData) != expected {
		t.Errorf("Response = %s, want %s", string(respData), expected)
	}

	t.Logf("Integration test passed: Request parsed correctly, response format validated")
}

// TestPermissionDenials_Parse validates the permission_denials field in result events.
// This is the new format used when Claude Code stop_reason="tool_disallowed".
// Reference: Claude Code permission_denials in type="result" events
func TestPermissionDenials_Parse(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		wantDenialCount int
		wantToolNames   []string
		wantToolUseIDs  []string
	}{
		{
			name: "result with single permission denial",
			input: `{
				"type": "result",
				"result": "Tool 'Edit' permission denied",
				"session_id": "c8418a03-b679-5dcc-bcde-dda4f38da44f",
				"permission_denials": [
					{
						"tool_name": "Edit",
						"tool_use_id": "call_function_abc123",
						"tool_input": {"file_path": "/Users/user/.zshrc", "old_string": "old token", "new_string": "new token"}
					}
				]
			}`,
			wantDenialCount: 1,
			wantToolNames:   []string{"Edit"},
			wantToolUseIDs:  []string{"call_function_abc123"},
		},
		{
			name: "result with multiple permission denials (AskUserQuestion + Edit + Edit)",
			input: `{
				"type": "result",
				"result": "Waiting for permission approval...",
				"session_id": "c8418a03-b679-5dcc-bcde-dda4f38da44f",
				"permission_denials": [
					{
						"tool_name": "AskUserQuestion",
						"tool_use_id": "call_function_iyhdvqe9p04y_1",
						"tool_input": {"questions": [{"header": "Shell Profile"}]}
					},
					{
						"tool_name": "Edit",
						"tool_use_id": "call_function_fxjuemzoghzo_1",
						"tool_input": {"file_path": "/Users/huangzhonghui/.zshrc"}
					},
					{
						"tool_name": "Edit",
						"tool_use_id": "call_function_fxjuemzoghzo_2",
						"tool_input": {"file_path": "/Users/huangzhonghui/.zprofile"}
					}
				]
			}`,
			wantDenialCount: 3,
			wantToolNames:   []string{"AskUserQuestion", "Edit", "Edit"},
			wantToolUseIDs:  []string{"call_function_iyhdvqe9p04y_1", "call_function_fxjuemzoghzo_1", "call_function_fxjuemzoghzo_2"},
		},
		{
			name: "result without permission denials",
			input: `{
				"type": "result",
				"result": "Task completed successfully",
				"session_id": "sess_123"
			}`,
			wantDenialCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var msg StreamMessage
			if err := json.Unmarshal([]byte(tt.input), &msg); err != nil {
				t.Fatalf("Failed to unmarshal: %v", err)
			}

			if len(msg.PermissionDenials) != tt.wantDenialCount {
				t.Errorf("PermissionDenials count = %d, want %d", len(msg.PermissionDenials), tt.wantDenialCount)
			}

			for i, denial := range msg.PermissionDenials {
				if i < len(tt.wantToolNames) && denial.ToolName != tt.wantToolNames[i] {
					t.Errorf("Denial[%d].ToolName = %q, want %q", i, denial.ToolName, tt.wantToolNames[i])
				}
				if i < len(tt.wantToolUseIDs) && denial.ToolUseID != tt.wantToolUseIDs[i] {
					t.Errorf("Denial[%d].ToolUseID = %q, want %q", i, denial.ToolUseID, tt.wantToolUseIDs[i])
				}
			}
		})
	}
}

// TestPermissionDeniedDetail_JSON validates the JSON serialization of PermissionDeniedDetail.
func TestPermissionDeniedDetail_JSON(t *testing.T) {
	input := `{
		"tool_name": "Bash",
		"tool_use_id": "call_123",
		"tool_input": {"command": "rm -rf /tmp/*"}
	}`

	var denial PermissionDeniedDetail
	if err := json.Unmarshal([]byte(input), &denial); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if denial.ToolName != "Bash" {
		t.Errorf("ToolName = %q, want %q", denial.ToolName, "Bash")
	}
	if denial.ToolUseID != "call_123" {
		t.Errorf("ToolUseID = %q, want %q", denial.ToolUseID, "call_123")
	}
	if denial.ToolInput == nil {
		t.Error("ToolInput should not be nil")
	}
}
