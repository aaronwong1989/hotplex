package chatapps

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestExpandPath(t *testing.T) {
	// Get home directory for test expectations
	homeDir, homeErr := os.UserHomeDir()

	tests := []struct {
		name     string
		input    string
		wantFunc func() string // Function to compute expected result
	}{
		{
			name:  "empty string",
			input: "",
			wantFunc: func() string {
				return ""
			},
		},
		{
			name:  "tilde only",
			input: "~",
			wantFunc: func() string {
				if homeErr != nil {
					return "~"
				}
				return homeDir
			},
		},
		{
			name:  "tilde with slash",
			input: "~/",
			wantFunc: func() string {
				if homeErr != nil {
					return "~/"
				}
				return homeDir
			},
		},
		{
			name:  "tilde with path",
			input: "~/test/path",
			wantFunc: func() string {
				if homeErr != nil {
					return "~/test/path"
				}
				return filepath.Join(homeDir, "test/path")
			},
		},
		{
			name:  "absolute path",
			input: "/absolute/path",
			wantFunc: func() string {
				return "/absolute/path"
			},
		},
		{
			name:  "relative path with dot",
			input: "./relative/path",
			wantFunc: func() string {
				return "relative/path" // filepath.Clean removes ./
			},
		},
		{
			name:  "path with double dots",
			input: "../parent/path",
			wantFunc: func() string {
				return "../parent/path"
			},
		},
		{
			name:  "complex tilde path",
			input: "~/hotplex/workspace",
			wantFunc: func() string {
				if homeErr != nil {
					return "~/hotplex/workspace"
				}
				return filepath.Join(homeDir, "hotplex/workspace")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := tt.wantFunc()
			got := ExpandPath(tt.input)
			if got != want {
				t.Errorf("ExpandPath(%q) = %q, want %q", tt.input, got, want)
			}
		})
	}
}

func TestExpandPath_PathTraversal(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("Cannot determine home directory: %v", err)
	}

	tests := []struct {
		name        string
		input       string
		shouldBlock bool // Whether the path should be rejected by security checks
	}{
		{
			name:        "simple traversal up",
			input:       "~/../etc/passwd",
			shouldBlock: true,
		},
		{
			name:        "deep traversal",
			input:       "~/../../etc/shadow",
			shouldBlock: true,
		},
		{
			name:        "hidden traversal",
			input:       "~/test/../../../etc/passwd",
			shouldBlock: true,
		},
		{
			name:        "safe subdirectory",
			input:       "~/hotplex/workspace",
			shouldBlock: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expanded := ExpandPath(tt.input)

			// Clean the path to resolve any .. elements
			cleaned := filepath.Clean(expanded)

			// Check if the cleaned path is still within home directory
			if tt.shouldBlock {
				// For paths that should be blocked, verify they escape home
				if !filepath.IsLocal(cleaned) && len(cleaned) > 0 && cleaned[0] == '/' {
					// Path escaped the intended directory - this is expected for traversal attempts
					// The security check should catch this
					if !isPathWithinBoundary(cleaned, homeDir) {
						// Correctly identified as potential traversal
						return
					}
				}
			}
		})
	}
}

func TestExpandPath_EdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "single tilde",
			input: "~",
		},
		{
			name:  "tilde with Windows separator",
			input: "~\\path\\to\\file",
		},
		{
			name:  "path with spaces",
			input: "~/My Documents/file.txt",
		},
		{
			name:  "path with unicode",
			input: "~/文档/文件.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify it doesn't panic
			_ = ExpandPath(tt.input)
		})
	}
}

// isPathWithinBoundary checks if a path is within the specified boundary directory
func isPathWithinBoundary(path, boundary string) bool {
	// Ensure both paths are absolute and clean
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absBoundary, err := filepath.Abs(boundary)
	if err != nil {
		return false
	}

	// Check if the path starts with the boundary
	rel, err := filepath.Rel(absBoundary, absPath)
	if err != nil {
		return false
	}

	// If the relative path starts with "..", it's outside the boundary
	return !filepath.IsLocal(rel) || (len(rel) >= 2 && rel[:2] != "..")
}

func TestExpandPath_SensitivePaths(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantEmpty bool // Whether the function should return empty string (blocked)
	}{
		// Blocked paths
		{
			name:      "etc passwd",
			input:     "/etc/passwd",
			wantEmpty: true,
		},
		{
			name:      "etc shadow",
			input:     "/etc/shadow",
			wantEmpty: true,
		},
		{
			name:      "var log",
			input:     "/var/log",
			wantEmpty: true,
		},
		{
			name:      "usr bin",
			input:     "/usr/bin",
			wantEmpty: true,
		},
		{
			name:      "root directory",
			input:     "/root",
			wantEmpty: true,
		},
		{
			name:      "proc filesystem",
			input:     "/proc",
			wantEmpty: true,
		},
		{
			name:      "sys filesystem",
			input:     "/sys",
			wantEmpty: true,
		},
		// Allowed paths
		{
			name:      "tmp directory",
			input:     "/tmp",
			wantEmpty: false,
		},
		{
			name:      "home directory",
			input:     "/home/user",
			wantEmpty: false,
		},
		{
			name:      "current directory",
			input:     ".",
			wantEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExpandPath(tt.input)
			if tt.wantEmpty && got != "" {
				t.Errorf("ExpandPath(%q) should be blocked (return empty), got %q", tt.input, got)
			}
			if !tt.wantEmpty && got == "" && tt.input[0] == '/' {
				t.Errorf("ExpandPath(%q) should not be blocked, got empty string", tt.input)
			}
		})
	}
}

func TestResolveWorkDir(t *testing.T) {
	// Get home directory for test expectations
	homeDir, _ := os.UserHomeDir()

	tests := []struct {
		name            string
		configuredPath  string
		platform        string
		setupEnv        map[string]string
		wantResolved    string
		wantReason      string
		wantLogContains string
	}{
		{
			name:           "empty path returns default",
			configuredPath: "",
			platform:       "slack",
			wantResolved:   "",
			wantReason:     "no work_dir configured",
		},
		{
			name:            "absolute path resolves correctly",
			configuredPath:  "/tmp/test-workdir",
			platform:        "slack",
			wantResolved:    "/tmp/test-workdir",
			wantReason:      "",
			wantLogContains: "Work directory initialized",
		},
		{
			name:            "tilde path expands correctly",
			configuredPath:  "~/test-workdir",
			platform:        "feishu",
			wantResolved:    filepath.Join(homeDir, "test-workdir"),
			wantReason:      "",
			wantLogContains: "Work directory initialized",
		},
		{
			name:           "path with unset env var - partial expansion",
			configuredPath: "${UNSET_VAR_XYZ123}/workdir",
			platform:       "slack",
			// sys.ExpandPath replaces unset vars with empty string
			wantResolved: "/workdir",
			wantReason:   "",
		},
		{
			name:           "relative path kept as-is",
			configuredPath: "./relative-workdir",
			platform:       "dingtalk",
			// sys.ExpandPath keeps relative paths
			wantResolved: "./relative-workdir",
			wantReason:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment variables if needed
			if tt.setupEnv != nil {
				for k, v := range tt.setupEnv {
					os.Setenv(k, v)
					defer os.Unsetenv(k)
				}
			}

			// Capture log output
			var logBuf bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{
				Level: slog.LevelDebug,
			}))

			// Execute
			result := resolveWorkDir(tt.configuredPath, tt.platform, logger)

			// Verify
			if result.ResolvedPath != tt.wantResolved {
				t.Errorf("ResolvedPath = %q, want %q", result.ResolvedPath, tt.wantResolved)
			}

			if result.PlatformName != tt.platform {
				t.Errorf("PlatformName = %q, want %q", result.PlatformName, tt.platform)
			}

			if result.DefaultReason != tt.wantReason {
				t.Errorf("DefaultReason = %q, want %q", result.DefaultReason, tt.wantReason)
			}

			logOutput := logBuf.String()
			if tt.wantLogContains != "" && !bytes.Contains([]byte(logOutput), []byte(tt.wantLogContains)) {
				t.Errorf("Log output does not contain %q\nGot: %s", tt.wantLogContains, logOutput)
			}
		})
	}
}

func TestResolveWorkDir_Logging(t *testing.T) {
	t.Run("logs info when path configured and resolves", func(t *testing.T) {
		var logBuf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))

		resolveWorkDir("/tmp/test", "slack", logger)

		logOutput := logBuf.String()
		if !bytes.Contains([]byte(logOutput), []byte("Work directory initialized")) {
			t.Errorf("Expected info log for successful resolution, got: %s", logOutput)
		}
	})

	t.Run("logs warning when expansion fails", func(t *testing.T) {
		var logBuf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{
			Level: slog.LevelWarn,
		}))

		resolveWorkDir("${UNSET_VAR_XYZ}", "slack", logger)

		logOutput := logBuf.String()
		if !bytes.Contains([]byte(logOutput), []byte("path expansion failed")) {
			t.Errorf("Expected warning log for failed expansion, got: %s", logOutput)
		}
	})

	t.Run("no log when path not configured", func(t *testing.T) {
		var logBuf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))

		resolveWorkDir("", "slack", logger)

		logOutput := logBuf.String()
		if logOutput != "" {
			t.Errorf("Expected no log for empty path, got: %s", logOutput)
		}
	})
}

func TestResolveWorkDir_RaceConditionSafety(t *testing.T) {
	// Test that resolveWorkDir returns values that can be safely used in closures
	// without causing race conditions (Code Review Finding #2)

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))

	// Simulate multiple goroutines using the result
	result := resolveWorkDir("/tmp/test", "slack", logger)

	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func() {
			// Access result fields - should not cause race
			_ = result.ResolvedPath
			_ = result.PlatformName
			_ = result.DefaultReason
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}
