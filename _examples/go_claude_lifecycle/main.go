package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hrygo/hotplex"
)

func main() {
	// 1. Configure logging with Debug level for internal visibility
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	fmt.Println("=== HotPlex Full SDK Demo: Lifecycle & Persistence ===")

	// 2. Create a custom Provider (optional - demonstrates Provider abstraction)
	// If Provider is nil, Engine will use ClaudeCodeProvider by default.
	provider, err := hotplex.NewClaudeCodeProvider(hotplex.ProviderConfig{
		DefaultPermissionMode: "bypass-permissions",
		AllowedTools:          []string{"Bash", "Read", "Edit", "Write"},
	}, logger)
	if err != nil {
		panic(fmt.Errorf("create provider: %w", err))
	}

	// 3. Initialize Engine with Provider and short IdleTimeout to demonstrate GC
	// Note: Internal cleanup loop runs every 1 minute.
	//
	// =============================================================================
	// SYSTEM PROMPT INJECTION EXAMPLES
	// =============================================================================
	// HotPlex 支持三种系统提示词注入方式：
	//
	// A) BaseSystemPrompt (Engine 级别) - 会话全程生效
	//    用途：定义 AI 的核心身份、行为规范、输出风格
	//
	// B) TaskInstructions (Session 级别) - 每个 turn 追加
	//    用途：任务特定指令、约束条件
	//
	// C) InitialPrompt (Session 级别) - 会话建立时自动执行的任务
	//    用途：无需用户发送消息，AI 在会话启动时自动执行指定任务
	//    例如："Show me git status" → 用户加入会话，AI 自动显示状态
	//
	// 示例 A: BaseSystemPrompt - 定义 AI 身份和输出风格
	// =============================================================================
	baseSystemPrompt := `You are HotPlex, a concise and practical AI coding assistant.

## Core Principles
- Think step by step before taking action
- Provide working code with minimal explanation
- Prefer Go, Python, or JavaScript for examples

## Output Style
- Use bullet points for lists (not paragraphs)
- Code blocks must have language tags: [code]go
- Never explain what you're about to do - just do it
- If unsure, state your assumption clearly`

	opts := hotplex.EngineOptions{
		Namespace:        "demo_lifecycle",
		Timeout:          5 * time.Minute,
		IdleTimeout:      10 * time.Second, // We set it short for demonstration, but it cleans every 1m
		Logger:           logger,
		Provider:         provider, // Custom provider (or nil for default ClaudeCodeProvider)
		BaseSystemPrompt: baseSystemPrompt,
	}

	engine, err := hotplex.NewEngine(opts)
	if err != nil {
		panic(err)
	}

	// Handle signals for graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		fmt.Println("\nInterrupted! Cleaning up...")
		_ = engine.Close()
		os.Exit(0)
	}()

	defer func() { _ = engine.Close() }()

	ctx := context.Background()

	// ---------------------------------------------------------
	// CASE 1: Cold Start & Multiplexing
	// ---------------------------------------------------------
	// =============================================================================
	// 示例 B: TaskInstructions - 每个 turn 追加的指令
	// =============================================================================
	// 注意：BaseSystemPrompt 定义身份，TaskInstructions 定义行为约束
	// =============================================================================
	taskInstructions := `## Task Rules
- Always respond in UPPERCASE for secret words
- Keep responses under 3 sentences
- End with an emoji`

	sessionID := "persistent-task-42"
	workDir, _ := os.Getwd()

	cfg := &hotplex.Config{
		SessionID:        sessionID,
		WorkDir:          workDir,
		TaskInstructions: taskInstructions,
	}

	fmt.Printf("\n[1] Running Turn 1 (Cold Start for session: %s)...\n", sessionID)
	err = engine.Execute(ctx, cfg, "Remember the secret word is 'ALBATROSS'. Just say 'OK'.", silentCallback)
	if err != nil {
		panic(err)
	}

	fmt.Printf("\n[2] Running Turn 2 (Hot-Multiplexing - same session)...\n")
	fmt.Println("Observe logs: it will reuse the existing process PID.")
	err = engine.Execute(ctx, cfg, "What was the secret word? Just say the word.", printingCallback)
	if err != nil {
		panic(err)
	}

	// ---------------------------------------------------------
	// CASE 2: Process Recovery (Persistence via Markers)
	// ---------------------------------------------------------
	fmt.Printf("\n[3] Simulating Engine Restart (Crash Recovery)...\n")
	_ = engine.Close() // Forcefully shut down everything

	// Re-initialize a new engine instance
	engine, _ = hotplex.NewEngine(opts)
	defer func() { _ = engine.Close() }()

	fmt.Println("New engine instance created. The underlying Claude CLI will use '--resume' because of marker files.")
	err = engine.Execute(ctx, cfg, "I just restarted my logic. What was the secret word again?", printingCallback)
	if err != nil {
		panic(err)
	}

	// ---------------------------------------------------------
	// CASE 3: Manual Termination
	// ---------------------------------------------------------
	fmt.Printf("\n[4] Explicitly Stopping Session...\n")
	err = engine.StopSession(sessionID, "Done with task")
	if err != nil {
		fmt.Printf("Stop failed: %v\n", err)
	} else {
		fmt.Println("Session terminated successfully.")
	}

	fmt.Println("\n[5] Fetching Final Session Stats Manually...")
	// Demonstrates the new v0.8.0 GetSessionStats(sessionID) interface
	stats := engine.GetSessionStats(sessionID)
	if stats != nil {
		fmt.Printf("Manually Fetched Stats -> Duration: %dms, Input Tokens: %d, Output Tokens: %d\n",
			stats.TotalDurationMs, stats.InputTokens, stats.OutputTokens)
	}

	fmt.Println("\n=== Demo Complete ===")
}

// printingCallback prints answers and final stats
func printingCallback(eventType string, data any) error {
	switch eventType {
	case "answer":
		if ev, ok := data.(*hotplex.EventWithMeta); ok {
			fmt.Printf("AI: %s", ev.EventData)
		}
	case "session_stats":
		if stats, ok := data.(*hotplex.SessionStatsData); ok {
			fmt.Printf("\n[STATS] Duration: %dms, Cost: $%f\n", stats.TotalDurationMs, stats.TotalCostUSD)
		}
	}
	return nil
}

// silent callback that ignores output
func silentCallback(eventType string, data any) error {
	return nil
}
