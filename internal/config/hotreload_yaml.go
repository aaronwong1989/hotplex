package config

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

// YAMLHotReloader is a hot reload implementation for YAML configuration files.
// It watches for file changes and automatically reloads the configuration.
type YAMLHotReloader struct {
	path     string
	logger   *slog.Logger
	config   any
	mu       sync.RWMutex
	watcher  *fsnotify.Watcher
	onReload func(any)
}

// NewYAMLHotReloader creates a new YAML hot reloader.
// The initialConfig must be a pointer to a struct that will be unmarshaled from YAML.
func NewYAMLHotReloader(path string, initialConfig any, logger *slog.Logger) (*YAMLHotReloader, error) {
	if logger == nil {
		logger = slog.Default()
	}

	loader := &YAMLHotReloader{
		path:   path,
		logger: logger,
		config: initialConfig,
	}

	if err := loader.load(); err != nil {
		return nil, err
	}

	return loader, nil
}

func (h *YAMLHotReloader) load() error {
	data, err := os.ReadFile(h.path)
	if err != nil {
		return fmt.Errorf("read config %s: %w", h.path, err)
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if err := yaml.Unmarshal(data, h.config); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	h.logger.Info("YAML config loaded", "path", h.path)
	return nil
}

// Start begins watching the configuration file for changes.
// It is idempotent - calling Start multiple times has no additional effect.
func (h *YAMLHotReloader) Start(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Idempotency check: prevent multiple watchers
	if h.watcher != nil {
		return nil
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}

	h.watcher = watcher

	if err := watcher.Add(h.path); err != nil {
		_ = watcher.Close()
		return fmt.Errorf("watch path %s: %w", h.path, err)
	}

	go h.watchLoop(ctx)

	h.logger.Info("YAML hot reloader started", "path", h.path)
	return nil
}

func (h *YAMLHotReloader) watchLoop(ctx context.Context) {
	var debounceTimer *time.Timer
	debounceDelay := 100 * time.Millisecond
	// Channel to signal timer cleanup
	timerDone := make(chan struct{})

	defer func() {
		// Clean up timer when watchLoop exits
		if debounceTimer != nil {
			debounceTimer.Stop()
		}
		close(timerDone)
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timerDone:
			return
		case event, ok := <-h.watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) {
				// Stop existing timer before creating a new one
				if debounceTimer != nil {
					if !debounceTimer.Stop() {
						// Timer already fired, wait for it to complete
						<-timerDone
						// Re-create the channel since it was closed
						timerDone = make(chan struct{})
					}
				}

				debounceTimer = time.AfterFunc(debounceDelay, func() {
					if err := h.load(); err != nil {
						h.logger.Error("Failed to reload YAML config", "error", err)
						return
					}
					h.mu.RLock()
					config := h.config
					h.mu.RUnlock()
					if h.onReload != nil {
						h.onReload(config)
					}
				})
			}
		case err, ok := <-h.watcher.Errors:
			if !ok {
				return
			}
			h.logger.Error("Watcher error", "error", err)
		}
	}
}

// OnReload sets the callback function that will be called when the config is reloaded.
func (h *YAMLHotReloader) OnReload(fn func(any)) {
	h.onReload = fn
}

// Get returns the current configuration.
func (h *YAMLHotReloader) Get() any {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.config
}

// Close stops the file watcher and releases resources.
func (h *YAMLHotReloader) Close() error {
	if h.watcher != nil {
		return h.watcher.Close()
	}
	return nil
}
