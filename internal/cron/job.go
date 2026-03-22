// Package cron provides the cron scheduling subsystem for HotPlex.
package cron

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// JobType classifies a cron job by resource intensity.
type JobType string

const (
	JobTypeLight             JobType = "light"
	JobTypeMedium            JobType = "medium"
	JobTypeResourceIntensive JobType = "resource-intensive"
)

// OutputFormat specifies how the job output should be formatted.
type OutputFormat string

const (
	OutputFormatText       OutputFormat = "text"
	OutputFormatJSON       OutputFormat = "json"
	OutputFormatStructured OutputFormat = "structured"
)

// Event represents a lifecycle event of a cron job.
type Event string

const (
	EventCompleted Event = "completed"
	EventFailed    Event = "failed"
	EventCanceled  Event = "canceled"
)

// CronJob represents a scheduled task.
type CronJob struct {
	ID         string `json:"id"`
	CronExpr   string `json:"cron_expr"`
	Prompt     string `json:"prompt"`
	SessionKey string `json:"session_key,omitempty"`
	WorkDir    string `json:"work_dir,omitempty"`

	Type        JobType       `json:"type"`
	TimeoutMins int           `json:"timeout_mins"`
	Retries     int           `json:"retries"`
	RetryDelay  time.Duration `json:"retry_delay"`

	OutputFormat OutputFormat `json:"output_format"`
	OutputSchema string       `json:"output_schema,omitempty"`

	Enabled  bool    `json:"enabled"`
	Silent   bool    `json:"silent"`
	NotifyOn []Event `json:"notify_on"`

	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	LastRun   time.Time `json:"last_run"`
	LastError string    `json:"last_error,omitempty"`
	NextRun   time.Time `json:"next_run"`
	RunCount  int       `json:"run_count"`

	OnComplete string `json:"on_complete,omitempty"`
	OnFail     string `json:"on_fail,omitempty"`
}

// Clone returns a deep copy of the CronJob.
func (j *CronJob) Clone() *CronJob {
	if j == nil {
		return nil
	}
	clone := *j
	if j.NotifyOn != nil {
		clone.NotifyOn = make([]Event, len(j.NotifyOn))
		copy(clone.NotifyOn, j.NotifyOn)
	}
	return &clone
}

// CronRun represents a single execution of a cron job.
type CronRun struct {
	ID         string        `json:"id"`
	JobID      string        `json:"job_id"`
	StartedAt  time.Time     `json:"started_at"`
	FinishedAt time.Time     `json:"finished_at"`
	Duration   time.Duration `json:"duration"`
	Status     string        `json:"status"`
	Error      string        `json:"error,omitempty"`
	RetryCount int           `json:"retry_count"`
	Response   string        `json:"response,omitempty"`
}

// CronCallback handles job lifecycle events.
type CronCallback interface {
	OnComplete(run *CronRun) error
	OnFail(run *CronRun) error
}

// WebhookCallback calls a remote URL on job completion or failure.
type WebhookCallback struct {
	URL     string
	Token   string
	Timeout time.Duration
	Retry   int
}

func (wc *WebhookCallback) OnComplete(run *CronRun) error { return wc.send(run) }
func (wc *WebhookCallback) OnFail(run *CronRun) error     { return wc.send(run) }

// send delivers a POST request to the webhook URL with run data as JSON.
// It retries up to wc.Retry times with exponential backoff.
func (wc *WebhookCallback) send(run *CronRun) error {
	if wc.URL == "" {
		return nil
	}

	timeout := wc.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	retry := wc.Retry
	if retry == 0 {
		retry = 2
	}

	delays := []time.Duration{500 * time.Millisecond, 1 * time.Second, 2 * time.Second}

	body, err := json.Marshal(run)
	if err != nil {
		return fmt.Errorf("marshal run: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= retry; attempt++ {
		if attempt > 0 && attempt-1 < len(delays) {
			time.Sleep(delays[attempt-1] * time.Duration(1<<(attempt-1)))
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, wc.URL, bytes.NewReader(body))
		if err != nil {
			cancel() // Cancel on error before next iteration
			lastErr = fmt.Errorf("create request: %w", err)
			continue
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		if wc.Token != "" {
			req.Header.Set("Authorization", "Bearer "+wc.Token)
		}
		// X-CronRun-ID and X-CronEvent headers allow receivers to correlate hooks.
		req.Header.Set("X-CronRun-ID", run.ID)
		req.Header.Set("X-CronEvent", run.Status)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			cancel() // Cancel on error
			lastErr = fmt.Errorf("do request: %w", err)
			continue
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		cancel() // Cancel after body is drained

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		lastErr = fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return fmt.Errorf("webhook %s: %w", wc.URL, lastErr)
}
