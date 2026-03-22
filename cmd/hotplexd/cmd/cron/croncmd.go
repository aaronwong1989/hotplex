package croncmd

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/hrygo/hotplex/internal/cron"
	"github.com/spf13/cobra"
)

// SessionCmd is the parent command for cron subcommands.
var SessionCmd = &cobra.Command{
	Use:   "cron",
	Short: "Cron job management commands",
}

func init() {
	SessionCmd.AddCommand(addCronCmd, delCronCmd, listCronCmd, pauseCronCmd, resumeCronCmd, listRunsCmd)
}

// ---------------------------------------------------------------------------
// add_cron
// ---------------------------------------------------------------------------

var addCronCmd = &cobra.Command{
	Use:   "add_cron --cron <expr> --prompt <text>",
	Short: "Add a new cron job",
	Args:  cobra.NoArgs,
	RunE:  runAddCron,
}

func init() {
	addCronCmd.Flags().String("cron", "", "Cron expression (e.g., */5 * * * *)")
	addCronCmd.Flags().String("prompt", "", "Natural language task description")
	addCronCmd.Flags().String("work-dir", "", "Working directory for the job")
	addCronCmd.Flags().String("type", "light", "Job type: light, medium, resource-intensive")
	addCronCmd.Flags().Int("timeout", 30, "Execution timeout in minutes")
	addCronCmd.Flags().Int("retries", 3, "Number of retries on failure")
	addCronCmd.Flags().Bool("disabled", false, "Create job in disabled state")
}

func runAddCron(cmd *cobra.Command, _ []string) error {
	cronExpr, _ := cmd.Flags().GetString("cron")
	prompt, _ := cmd.Flags().GetString("prompt")
	workDir, _ := cmd.Flags().GetString("work-dir")
	jobType, _ := cmd.Flags().GetString("type")
	timeout, _ := cmd.Flags().GetInt("timeout")
	retries, _ := cmd.Flags().GetInt("retries")
	disabled, _ := cmd.Flags().GetBool("disabled")

	if cronExpr == "" || prompt == "" {
		return fmt.Errorf("--cron and --prompt are required")
	}

	store, err := cron.NewCronStore("")
	if err != nil {
		return fmt.Errorf("open cron store: %w", err)
	}

	job := &cron.CronJob{
		CronExpr:    cronExpr,
		Prompt:      prompt,
		WorkDir:     workDir,
		Type:        cron.JobType(jobType),
		TimeoutMins: timeout,
		Retries:     retries,
		Enabled:     !disabled,
	}

	if err := store.Add(job); err != nil {
		return fmt.Errorf("add cron job: %w", err)
	}
	fmt.Printf("Cron job created: %s\n", job.ID)
	return nil
}

// ---------------------------------------------------------------------------
// del_cron
// ---------------------------------------------------------------------------

var delCronCmd = &cobra.Command{
	Use:   "del_cron --id <job-id>",
	Short: "Delete a cron job",
	Args:  cobra.NoArgs,
	RunE:  runDelCron,
}

func init() {
	delCronCmd.Flags().String("id", "", "Cron job ID to delete")
}

func runDelCron(cmd *cobra.Command, _ []string) error {
	id, _ := cmd.Flags().GetString("id")
	if id == "" {
		return fmt.Errorf("--id is required")
	}
	store, err := cron.NewCronStore("")
	if err != nil {
		return fmt.Errorf("open cron store: %w", err)
	}
	if err := store.Delete(id); err != nil {
		return fmt.Errorf("delete cron job: %w", err)
	}
	fmt.Printf("Cron job deleted: %s\n", id)
	return nil
}

// ---------------------------------------------------------------------------
// list_crons
// ---------------------------------------------------------------------------

var listCronCmd = &cobra.Command{
	Use:   "list_crons",
	Short: "List all cron jobs",
	Args:  cobra.NoArgs,
	RunE:  runListCron,
}

func runListCron(_ *cobra.Command, _ []string) error {
	store, err := cron.NewCronStore("")
	if err != nil {
		return fmt.Errorf("open cron store: %w", err)
	}
	jobs := store.List()
	if len(jobs) == 0 {
		fmt.Println("No cron jobs found.")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "ID\tCRON\tPROMPT\tENABLED\tTYPE\tLAST RUN\tNEXT RUN"); err != nil {
		return err
	}
	for _, j := range jobs {
		enabled := "true"
		if !j.Enabled {
			enabled = "false"
		}
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			j.ID, j.CronExpr, j.Prompt, enabled, j.Type,
			j.LastRun.Format("2006-01-02T15:04"),
			j.NextRun.Format("2006-01-02T15:04")); err != nil {
			return err
		}
	}
	return w.Flush()
}

// ---------------------------------------------------------------------------
// pause_cron
// ---------------------------------------------------------------------------

var pauseCronCmd = &cobra.Command{
	Use:   "pause_cron --id <job-id>",
	Short: "Pause a cron job",
	Args:  cobra.NoArgs,
	RunE:  runPauseCron,
}

func init() {
	pauseCronCmd.Flags().String("id", "", "Cron job ID to pause")
}

func runPauseCron(cmd *cobra.Command, _ []string) error {
	return setJobEnabled(cmd, false)
}

// ---------------------------------------------------------------------------
// resume_cron
// ---------------------------------------------------------------------------

var resumeCronCmd = &cobra.Command{
	Use:   "resume_cron --id <job-id>",
	Short: "Resume a paused cron job",
	Args:  cobra.NoArgs,
	RunE:  runResumeCron,
}

func init() {
	resumeCronCmd.Flags().String("id", "", "Cron job ID to resume")
}

func runResumeCron(cmd *cobra.Command, _ []string) error {
	return setJobEnabled(cmd, true)
}

// setJobEnabled is a helper that handles both pause and resume operations.
func setJobEnabled(cmd *cobra.Command, enabled bool) error {
	id, _ := cmd.Flags().GetString("id")
	if id == "" {
		return fmt.Errorf("--id is required")
	}
	store, err := cron.NewCronStore("")
	if err != nil {
		return fmt.Errorf("open cron store: %w", err)
	}
	job := store.Get(id)
	if job == nil {
		return fmt.Errorf("job %q not found", id)
	}
	job.Enabled = enabled
	action := "pause"
	if enabled {
		action = "resume"
	}
	if err := store.Update(job); err != nil {
		return fmt.Errorf("%s cron job: %w", action, err)
	}
	fmt.Printf("Cron job %sd: %s\n", action, id)
	return nil
}

// ---------------------------------------------------------------------------
// list_runs
// ---------------------------------------------------------------------------

var listRunsCmd = &cobra.Command{
	Use:   "list_runs --job-id <id> [--last N]",
	Short: "List execution history for a cron job",
	Args:  cobra.NoArgs,
	RunE:  runListRuns,
}

func init() {
	listRunsCmd.Flags().String("job-id", "", "Cron job ID")
	listRunsCmd.Flags().Int("last", 0, "Show only the last N runs (0 = all)")
}

func runListRuns(cmd *cobra.Command, _ []string) error {
	jobID, _ := cmd.Flags().GetString("job-id")
	if jobID == "" {
		return fmt.Errorf("--job-id is required")
	}
	lastN, _ := cmd.Flags().GetInt("last")

	runsStore, err := cron.NewRunsStore("")
	if err != nil {
		return fmt.Errorf("open runs store: %w", err)
	}

	runs := runsStore.GetRuns(jobID)
	if len(runs) == 0 {
		fmt.Println("No runs found.")
		return nil
	}

	if lastN > 0 && lastN < len(runs) {
		runs = runs[:lastN]
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "RUN ID\tSTATUS\tSTARTED AT\tDURATION\tERROR\tRETRIES"); err != nil {
		return err
	}
	for _, r := range runs {
		errMsg := r.Error
		if errMsg == "" && r.Status == string(cron.EventFailed) {
			errMsg = "(none)"
		}
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d\n",
			r.ID, r.Status,
			r.StartedAt.Format("2006-01-02T15:04:05"),
			r.Duration.Round(time.Millisecond).String(),
			errMsg,
			r.RetryCount,
		); err != nil {
			return err
		}
	}
	return w.Flush()
}
