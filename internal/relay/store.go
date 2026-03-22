package relay

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// BindingStore persists RelayBindings to disk using atomic writes.
type BindingStore struct {
	path     string
	mu       sync.Mutex
	bindings map[string]*RelayBinding // keyed by ChatID
}

// bindingsFile is the on-disk JSON format.
type bindingsFile struct {
	Version  int             `json:"version"`
	Bindings []*RelayBinding `json:"bindings"`
}

const bindingsSchemaVersion = 1

// NewBindingStore loads or creates a BindingStore at the default path.
func NewBindingStore(dataDir string) *BindingStore {
	if dataDir == "" {
		dataDir = filepath.Join(os.Getenv("HOME"), ".hotplex", "relay")
	}
	_ = os.MkdirAll(dataDir, 0o755)
	path := filepath.Join(dataDir, "bindings.json")
	bs := &BindingStore{path: path, bindings: make(map[string]*RelayBinding)}
	_ = bs.load()
	return bs
}

// List returns a snapshot of all bindings.
func (bs *BindingStore) List() []*RelayBinding {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	result := make([]*RelayBinding, 0, len(bs.bindings))
	for _, b := range bs.bindings {
		result = append(result, b)
	}
	return result
}

// Add creates or updates a binding, keyed by ChatID.
func (bs *BindingStore) Add(binding *RelayBinding) error {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	bs.bindings[binding.ChatID] = binding
	return bs.atomicWriteLocked()
}

// Delete removes a binding by ChatID.
func (bs *BindingStore) Delete(chatID string) error {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	if _, ok := bs.bindings[chatID]; !ok {
		return fmt.Errorf("binding %q not found", chatID)
	}
	delete(bs.bindings, chatID)
	return bs.atomicWriteLocked()
}

func (bs *BindingStore) atomicWriteLocked() error {
	bindings := make([]*RelayBinding, 0, len(bs.bindings))
	for _, b := range bs.bindings {
		bindings = append(bindings, b)
	}
	data := bindingsFile{Version: bindingsSchemaVersion, Bindings: bindings}

	tmp := bs.path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open tmp file: %w", err)
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("encode json: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("close tmp file: %w", err)
	}
	if err := os.Rename(tmp, bs.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("atomic rename: %w", err)
	}
	return nil
}

func (bs *BindingStore) load() error {
	bs.bindings = make(map[string]*RelayBinding)
	f, err := os.Open(bs.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open bindings.json: %w", err)
	}
	defer func() { _ = f.Close() }()
	var bf bindingsFile
	if err := json.NewDecoder(f).Decode(&bf); err != nil {
		return fmt.Errorf("decode bindings.json: %w", err)
	}
	for _, b := range bf.Bindings {
		bs.bindings[b.ChatID] = b
	}
	return nil
}
