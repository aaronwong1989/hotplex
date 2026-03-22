package engine

import (
	"context"
	"fmt"

	intengine "github.com/hrygo/hotplex/internal/engine"
	"github.com/hrygo/hotplex/internal/relay"
)

// RelayExecutor handles cross-instance relay requests.
type RelayExecutor interface {
	HandleRelay(ctx context.Context, req *RelayRequest) (*RelayResponse, error)
}

// RelayRequest is the input for a relay operation.
type RelayRequest struct {
	From       string `json:"from"`
	To         string `json:"to"`
	SessionKey string `json:"session_key"`
	Message    string `json:"message"`
}

// RelayResponse is the result of a relay operation.
type RelayResponse = relay.RelayResponse

// HandleRelay processes an incoming relay message.
func (e *Engine) HandleRelay(ctx context.Context, req *RelayRequest) (*RelayResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("relay request is nil")
	}
	// TODO: delegate to SessionPool with namespace="relay"
	return &RelayResponse{Status: "ok"}, nil
}

// Assert that Engine implements RelayExecutor at compile time.
var _ RelayExecutor = (*Engine)(nil)

// Compile-time check that internal SessionManager is used.
var _ intengine.SessionManager = (*intengine.SessionPool)(nil)
