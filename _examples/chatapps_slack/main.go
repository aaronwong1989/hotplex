package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/hrygo/hotplex/chatapps"
	"github.com/hrygo/hotplex/chatapps/slack"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Get configuration from environment or use defaults
	serverAddr := os.Getenv("HOTPLEX_SLACK_SERVER_ADDR")
	if serverAddr == "" {
		serverAddr = ":8080"
	}

	// Initialize Slack Adapter
	// Note: BotToken is required. AppToken is required for Socket Mode.
	config := &slack.Config{
		BotToken:      os.Getenv("HOTPLEX_SLACK_BOT_TOKEN"),
		AppToken:      os.Getenv("HOTPLEX_SLACK_APP_TOKEN"),
		SigningSecret: os.Getenv("HOTPLEX_SLACK_SIGNING_SECRET"),
		Mode:          os.Getenv("HOTPLEX_SLACK_MODE"), // "http" or "socket" (default: http)
		ServerAddr:    serverAddr,
		// Optional: Configure permission policies
		DMPolicy:    "allow", // "allow", "pairing", "block"
		GroupPolicy: "allow", // "allow", "mention", "multibot", "block"
		BotUserID:   os.Getenv("HOTPLEX_SLACK_BOT_USER_ID"),
	}

	adapter := slack.NewAdapter(config, logger)

	// Set message handler - this is where you integrate with HotPlex Engine
	adapter.SetHandler(func(ctx context.Context, msg *chatapps.ChatMessage) error {
		fmt.Printf("\n📥 Received message from %s:\n   %s\n", msg.UserID, msg.Content)
		fmt.Println("   Processing...")

		// Echo back the message (replace with your HotPlex Engine logic)
		response := &chatapps.ChatMessage{
			Platform:  "slack",
			SessionID: msg.SessionID,
			Content:   "Received: " + msg.Content,
			Metadata:  msg.Metadata,
		}

		if err := adapter.SendMessage(ctx, msg.SessionID, response); err != nil {
			fmt.Printf("   ❌ Send failed: %v\n", err)
		} else {
			fmt.Println("   ✅ Response sent")
		}
		return nil
	})

	// Start the adapter
	if err := adapter.Start(context.Background()); err != nil {
		logger.Error("Failed to start adapter", "error", err)
		os.Exit(1)
	}

	fmt.Println("🎉 Slack Chat Adapter started!")
	fmt.Printf("   Listen address: http://localhost%s\n", serverAddr)
	fmt.Println("   Events endpoint: /events")
	fmt.Println("   Interactive endpoint: /interactive")
	fmt.Println("   Slash commands: /slack")
	fmt.Println("   Health check: /health")
	fmt.Println("\nPress Ctrl+C to exit")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\n👋 Shutting down...")
	adapter.Stop()
}
