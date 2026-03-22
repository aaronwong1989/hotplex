package bridgeclient

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

// Option configures a BridgeClient.
type Option func(*Client) error

// URL sets the BridgeServer WebSocket URL.
// Example: "ws://localhost:8080/bridge" or "wss://hotplex.internal/bridge"
func URL(v string) Option {
	return func(c *Client) error {
		c.url = v
		return nil
	}
}

// Platform sets the platform name this adapter represents.
// This is used as the platform identifier in the Bridge Wire Protocol.
// Example: "dingtalk", "wechat", "lark"
func Platform(v string) Option {
	return func(c *Client) error {
		c.platform = v
		return nil
	}
}

// AuthToken sets the token used to authenticate with BridgeServer.
// This must match the bridge_token configured in HotPlex.
func AuthToken(v string) Option {
	return func(c *Client) error {
		c.token = v
		return nil
	}
}

// Capabilities sets the capabilities this adapter supports.
// If not set, defaults to [CapText].
func Capabilities(caps ...string) Option {
	return func(c *Client) error {
		c.caps = caps
		return nil
	}
}

// Logger sets the logger used by the client.
// If not set, defaults to slog.Default().
func Logger(l *slog.Logger) Option {
	return func(c *Client) error {
		c.logger = l
		return nil
	}
}

// Timeout sets the HTTP client timeout used during connection establishment.
func Timeout(d time.Duration) Option {
	return func(c *Client) error {
		c.httpClient.Timeout = d
		c.dialer.HandshakeTimeout = d
		return nil
	}
}

// TLSConfig sets the TLS configuration for wss:// connections.
func TLSConfig(cfg *tls.Config) Option {
	return func(c *Client) error {
		c.dialer.TLSClientConfig = cfg
		return nil
	}
}

// HTTPClient sets a custom HTTP client for WebSocket dialing.
// This overrides URL, Timeout, and TLSConfig options.
func HTTPClient(hc *http.Client) Option {
	return func(c *Client) error {
		if hc == nil {
			return fmt.Errorf("http client cannot be nil")
		}
		c.httpClient = hc
		return nil
	}
}

// ProxyURL sets a WebSocket proxy URL (ws:// or wss://).
func ProxyURL(raw string) Option {
	return func(c *Client) error {
		u, err := url.Parse(raw)
		if err != nil {
			return fmt.Errorf("invalid proxy URL: %w", err)
		}
		c.dialer.Proxy = http.ProxyURL(u)
		return nil
	}
}
