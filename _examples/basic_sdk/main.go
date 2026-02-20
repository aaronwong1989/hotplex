package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/hrygo/hotplex/pkg/hotplex"
)

/*
This example demonstrates how to integrate the HotPlex SDK directly into a Go application.
It covers the lifecycle of initializing the Engine, configuring a session, sending a prompt,
and processing the real-time stream of events using a callback.
*/
func main() {
	// Configure logging
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// 1. Initialize HotPlex Core Engine
	// EngineOptions allows you to configure global behavior of the HotPlex instance.
	opts := hotplex.EngineOptions{
		Timeout:   5 * time.Minute, // Maximum allowed duration for a single execution
		Logger:    logger,          // Injected slog logger for structured observability
		Namespace: "demo_app",      // Custom string namespace for deterministic UUID isolation

		// Global Security Context
		PermissionMode:     "bypassPermissions",                   // Ensure demo runs seamlessly
		BaseSystemPrompt:   "You are a helpful Go CLI assistant.", // Core persona
		GlobalAllowedPaths: []string{"/tmp", "/var/tmp"},          // Baseline allowed paths
		ForbiddenPaths:     []string{"/etc", "/root"},             // Strict blacklist
	}

	engine, err := hotplex.NewEngine(opts)
	if err != nil {
		log.Fatalf("Failed to initialize HotPlex: %v", err)
	}
	defer engine.Close()

	// 2. Define Execution Configuration
	// This configuration dictates how a specific task is executed within the Engine.
	cfg := &hotplex.Config{
		WorkDir:             "/tmp",                         // The isolated working directory for the agent to operate in
		SessionID:           "local-demo-1",                 // A unique ID for Hot-Multiplexing (process reuse). Same ID = same process.
		UserID:              1,                              // Identifier for the user initiating the request (used for auditing and context)
		TaskSystemPrompt:    "Output directly, no yapping.", // Specific command for this turn
		SessionAllowedPaths: []string{"/tmp/workspace"},     // Extra paths needed for this specific task
	}

	prompt := "Write a one-line bash script to print hello world and execute it."

	fmt.Printf("--- Sending Prompt ---\n%s\n----------------------\n\n", prompt)

	// 3. Define the Callback to consume streaming events
	// HotPlex uses an asynchronous, event-driven model. The callback is invoked
	// repeatedly as the underlying LLM CLI agent emits output.
	cb := func(eventType string, data any) error {
		// In a real application (like a web server), you would marshal this data
		// and push it to a WebSocket or Server-Sent Events (SSE) stream.

		switch eventType {
		case "thinking":
			// Emitted when the agent is formulating a plan or waiting for the model.
			fmt.Println("🤔 Thinking...")

		case "tool_use":
			// Emitted when the agent decides to invoke a local tool (e.g., bash, read_file).
			// We can inspect the data struct directly for detailed tool parameters.
			if msg, ok := data.(hotplex.StreamMessage); ok {
				fmt.Printf("🛠️ Tool Use: %s\n", msg.Name)
			}

		case "assistant":
			// Emitted when the agent streams textual responses back to the user.
			if msg, ok := data.(hotplex.StreamMessage); ok {
				if len(msg.Message.Content) > 0 {
					for _, c := range msg.Message.Content {
						if c.Type == "text" {
							fmt.Print(c.Text) // Print the streamed chunk without newline
						}
					}
				}
			}

		case "session_stats":
			// Emitted at the very end of the execution. Contains rich usage telemetry
			// including duration, token consumption, and cost tracking.
			fmt.Println("\n\n📊 Session Completed!")
			if stats, ok := data.(*hotplex.SessionStatsData); ok {
				fmt.Printf("- Duration: %d ms\n", stats.TotalDurationMs)
				fmt.Printf("- Tokens (In/Out): %d / %d\n", stats.InputTokens, stats.OutputTokens)
				fmt.Printf("- Tools used: %d\n", stats.ToolCallCount)
			}

		case "danger_block":
			// Emitted if the WAF (Web Application Firewall) intercepts a malicious prompt or tool usage.
			fmt.Println("\n🚨 SECURITY ALERT: Operation blocked by HotPlex Firewall!")
		}

		return nil
	}

	// 4. Executing the Task
	// We wrap the execution in a Context to allow for application-level cancellation
	// (e.g., if a user disconnects or an API request times out).
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Engine.Execute blocks until the task completes, errors out, or is cancelled.
	err = engine.Execute(ctx, cfg, prompt, cb)
	if err != nil {
		log.Fatalf("Execution failed: %v", err)
	}
}
