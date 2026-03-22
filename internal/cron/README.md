# Cron Scheduler (`internal/cron`)

The `cron` package provides a robust scheduling system for periodic AI agent tasks. It allows users to automate long-running or repetitive workflows using natural language prompts.

## Overview

The scheduler uses `robfig/cron/v3` for precise timing and integrates with the `internal/engine` to execute tasks in isolated sessions.

## Core Components

- **`CronScheduler`**: The main orchestrator that manages job registration, timing, and execution.
- **`CronStore`**: Persistent storage for job definitions (JSON-based).
- **`RunsStore`**: Persistent storage for execution history and logs.
- **`Executor`**: Handles the actual execution of jobs via the `SessionManager`.

## Features

- **Standard Cron Syntax**: Support for standard cron expressions (e.g., `0 0 * * *`).
- **Isolation**: Each job run creates a dedicated session in the engine.
- **Retries**: Automatic exponential backoff retries on failure (1s → 2s → 4s).
- **Concurrency Control**: Global limit on concurrent job executions (default: 4).
- **Webhooks**: Optional `OnComplete` and `OnFail` callback URLs.

## Usage

```go
// Create a new job
job := &cron.CronJob{
    CronExpr: "*/30 * * * *",
    Prompt:   "Audit the logs for security anomalies",
    WorkDir:  "/app/logs",
    Enabled:  true,
}

// Add to scheduler
err := scheduler.AddJob(job)
```

## CLI Interface

Manage cron jobs via `hotplexd cron`:
- `hotplexd cron add_cron`: Create a new scheduled task.
- `hotplexd cron list_crons`: View all registered jobs.
- `hotplexd cron list_runs`: Check execution history and error logs.
