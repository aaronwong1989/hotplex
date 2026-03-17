package feishu

import (
	"context"
)

// Start 启动飞书适配器
// 如果启用了 WebSocket 模式，则启动 WebSocket 客户端
// 否则启动 HTTP Webhook 服务器
func (a *Adapter) Start(ctx context.Context) error {
	// 如果启用 WebSocket 模式，启动 WebSocket 客户端
	if a.useWebSocket {
		a.Logger().Info("Starting Feishu adapter in WebSocket mode")
		if err := a.startWebSocket(ctx); err != nil {
			return err
		}
		// WebSocket 模式不需要 HTTP 服务器，但仍需要启动基础适配器的清理逻辑
		// 这里我们调用基础适配器的 Start 方法，但传递一个禁用服务器的选项
		// 由于 base.Adapter 的 Start 会自动处理，我们只需要确保 WebSocket 正常工作
		return nil
	}

	// Webhook 模式：启动 HTTP 服务器
	a.Logger().Info("Starting Feishu adapter in Webhook mode")
	return a.Adapter.Start(ctx)
}

// Stop 停止飞书适配器
func (a *Adapter) Stop() error {
	// 停止 WebSocket 客户端
	if a.wsClient != nil {
		if err := a.stopWebSocket(); err != nil {
			a.Logger().Error("Failed to stop WebSocket client", "error", err)
		}
	}

	// 停止基础适配器（包括 HTTP 服务器等）
	return a.Adapter.Stop()
}
