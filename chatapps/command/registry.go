package command

import (
	"context"
	"fmt"
	"sync"

	"github.com/hrygo/hotplex/event"
)

// Registry manages all registered command executors
type Registry struct {
	mu   sync.RWMutex
	cmds map[string]Executor
}

// NewRegistry creates a new command registry
func NewRegistry() *Registry {
	return &Registry{
		cmds: make(map[string]Executor),
	}
}

// Register adds a command executor to the registry
func (r *Registry) Register(exec Executor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cmds[exec.Command()] = exec
}

// Get retrieves a command executor by name
func (r *Registry) Get(command string) (Executor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	exec, ok := r.cmds[command]
	return exec, ok
}

// Execute runs a command by name
func (r *Registry) Execute(ctx context.Context, req *Request, callback event.Callback) (*Result, error) {
	exec, ok := r.Get(req.Command)
	if !ok {
		return nil, fmt.Errorf("unknown command: %s", req.Command)
	}

	return exec.Execute(ctx, req, callback)
}

// List returns all registered commands
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	commands := make([]string, 0, len(r.cmds))
	for cmd := range r.cmds {
		commands = append(commands, cmd)
	}
	return commands
}
