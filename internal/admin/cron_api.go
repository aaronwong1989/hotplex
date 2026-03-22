package admin

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hrygo/hotplex/internal/adminapi"
	"github.com/hrygo/hotplex/internal/cron"
)

// listCronJobs handles GET /admin/cron/jobs.
func (h *Handler) listCronJobs(w http.ResponseWriter, r *http.Request) {
	if h.cronScheduler == nil {
		adminapi.WriteError(w, http.StatusServiceUnavailable, ErrCodeServerError, "Cron scheduler not initialized")
		return
	}

	jobs := h.cronScheduler.ListJobs()
	adminapi.WriteJSON(w, http.StatusOK, CronJobListResponse{
		Jobs:  jobs,
		Total: len(jobs),
	})
}

// createCronJob handles POST /admin/cron/jobs.
func (h *Handler) createCronJob(w http.ResponseWriter, r *http.Request) {
	if h.cronScheduler == nil {
		adminapi.WriteError(w, http.StatusServiceUnavailable, ErrCodeServerError, "Cron scheduler not initialized")
		return
	}

	var req CronJobCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		adminapi.WriteError(w, http.StatusBadRequest, ErrCodeInvalidRequest, "Invalid JSON body")
		return
	}

	job := &cron.CronJob{
		CronExpr:     req.CronExpr,
		Prompt:       req.Prompt,
		SessionKey:   req.SessionKey,
		WorkDir:      req.WorkDir,
		Type:         cron.JobType(req.Type),
		TimeoutMins:  req.TimeoutMins,
		Retries:      req.Retries,
		RetryDelay:   req.RetryDelay,
		OutputFormat: cron.OutputFormat(req.OutputFormat),
		OutputSchema: req.OutputSchema,
		Enabled:      req.Enabled,
		Silent:       req.Silent,
		NotifyOn:     req.NotifyOn,
		CreatedBy:    req.CreatedBy,
	}

	if err := h.cronScheduler.AddJob(job); err != nil {
		adminapi.WriteError(w, http.StatusInternalServerError, ErrCodeServerError, "Failed to create job: "+err.Error())
		return
	}

	adminapi.WriteJSON(w, http.StatusCreated, CronJobResponse{Job: job})
}

// getCronJob handles GET /admin/cron/jobs/:id.
func (h *Handler) getCronJob(w http.ResponseWriter, r *http.Request) {
	if h.cronScheduler == nil {
		adminapi.WriteError(w, http.StatusServiceUnavailable, ErrCodeServerError, "Cron scheduler not initialized")
		return
	}

	vars := mux.Vars(r)
	job := h.cronScheduler.GetJob(vars["id"])
	if job == nil {
		adminapi.WriteError(w, http.StatusNotFound, ErrCodeNotFound, "Job not found: "+vars["id"])
		return
	}
	adminapi.WriteJSON(w, http.StatusOK, CronJobResponse{Job: job})
}

// deleteCronJob handles DELETE /admin/cron/jobs/:id.
func (h *Handler) deleteCronJob(w http.ResponseWriter, r *http.Request) {
	if h.cronScheduler == nil {
		adminapi.WriteError(w, http.StatusServiceUnavailable, ErrCodeServerError, "Cron scheduler not initialized")
		return
	}

	vars := mux.Vars(r)
	if err := h.cronScheduler.RemoveJob(vars["id"]); err != nil {
		adminapi.WriteError(w, http.StatusNotFound, ErrCodeNotFound, "Failed to delete job: "+err.Error())
		return
	}
	adminapi.WriteJSON(w, http.StatusOK, CronJobDeleteResponse{Success: true, Message: "Job " + vars["id"] + " deleted"})
}

// pauseCronJob handles POST /admin/cron/jobs/:id/pause.
func (h *Handler) pauseCronJob(w http.ResponseWriter, r *http.Request) {
	if h.cronScheduler == nil {
		adminapi.WriteError(w, http.StatusServiceUnavailable, ErrCodeServerError, "Cron scheduler not initialized")
		return
	}

	vars := mux.Vars(r)
	if err := h.cronScheduler.PauseJob(vars["id"]); err != nil {
		adminapi.WriteError(w, http.StatusNotFound, ErrCodeNotFound, "Failed to pause job: "+err.Error())
		return
	}
	adminapi.WriteJSON(w, http.StatusOK, CronJobResponse{Job: h.cronScheduler.GetJob(vars["id"])})
}

// resumeCronJob handles POST /admin/cron/jobs/:id/resume.
func (h *Handler) resumeCronJob(w http.ResponseWriter, r *http.Request) {
	if h.cronScheduler == nil {
		adminapi.WriteError(w, http.StatusServiceUnavailable, ErrCodeServerError, "Cron scheduler not initialized")
		return
	}

	vars := mux.Vars(r)
	if err := h.cronScheduler.ResumeJob(vars["id"]); err != nil {
		adminapi.WriteError(w, http.StatusNotFound, ErrCodeNotFound, "Failed to resume job: "+err.Error())
		return
	}
	adminapi.WriteJSON(w, http.StatusOK, CronJobResponse{Job: h.cronScheduler.GetJob(vars["id"])})
}

// listCronJobRuns handles GET /admin/cron/jobs/:id/runs.
func (h *Handler) listCronJobRuns(w http.ResponseWriter, r *http.Request) {
	if h.cronScheduler == nil {
		adminapi.WriteError(w, http.StatusServiceUnavailable, ErrCodeServerError, "Cron scheduler not initialized")
		return
	}

	vars := mux.Vars(r)
	runs := h.cronScheduler.ListRuns(vars["id"])
	adminapi.WriteJSON(w, http.StatusOK, CronRunListResponse{Runs: runs, Total: len(runs)})
}
