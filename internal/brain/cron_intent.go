// Package brain provides intent detection and routing for user messages.
package brain

import (
	"regexp"
	"strings"

	"github.com/hrygo/hotplex/internal/cron"
)

// Cron keyword patterns for detecting intent.
var (
	patternAddCron = regexp.MustCompile(`(?i)(安排|每[天周月分秒]|cron|定时|scheduled?)\s*(.+)`)
	patternDelCron = regexp.MustCompile(`(?i)(删除|取消|delete|remove)\s*(cron|定时|scheduled?)`)
	patternPause   = regexp.MustCompile(`(?i)(暂停|pause)\s*(cron|定时)`)
	patternResume  = regexp.MustCompile(`(?i)(恢复|resume)\s*(cron|定时)`)
)

// AddCronJobIntent represents a detected request to add a new cron job.
type AddCronJobIntent struct {
	CronExpr    string
	Prompt      string
	WorkDir     string
	Type        cron.JobType
	TimeoutMins int
	Enabled     bool
}

// DeleteCronJobIntent represents a detected request to delete a cron job.
type DeleteCronJobIntent struct {
	JobID string
}

// PauseCronJobIntent represents a detected request to pause a cron job.
type PauseCronJobIntent struct {
	JobID string
}

// ResumeCronJobIntent represents a detected request to resume a cron job.
type ResumeCronJobIntent struct {
	JobID string
}

// Intent is the interface implemented by all intent types.
type Intent interface {
	isIntent()
}

func (AddCronJobIntent) isIntent()    {}
func (DeleteCronJobIntent) isIntent() {}
func (PauseCronJobIntent) isIntent()  {}
func (ResumeCronJobIntent) isIntent() {}

// DetectCronIntent returns an intent if the message contains a cron-related request.
func DetectCronIntent(msg string) (Intent, bool) {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return nil, false
	}

	// Delete cron job
	if patternDelCron.MatchString(msg) {
		return DeleteCronJobIntent{}, true
	}
	// Pause cron job
	if patternPause.MatchString(msg) {
		return PauseCronJobIntent{}, true
	}
	// Resume cron job
	if patternResume.MatchString(msg) {
		return ResumeCronJobIntent{}, true
	}
	// Add cron job (look for schedule keywords)
	if patternAddCron.MatchString(msg) {
		return parseAddCronIntent(msg)
	}
	return nil, false
}

// parseAddCronIntent extracts cron schedule and prompt from a message.
func parseAddCronIntent(msg string) (Intent, bool) {
	return AddCronJobIntent{
		Prompt:  msg,
		Type:    cron.JobTypeLight,
		Enabled: true,
	}, true
}
