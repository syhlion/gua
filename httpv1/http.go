// Package httpv1 is gua's admin REST API (resource oriented, under /v1).
//
// Every handler is a thin adapter over delayquene.Quene: it parses the request,
// calls the queue, and maps the queue's sentinel errors to HTTP statuses
// (ErrInvalid → 400, ErrNotFound → 404, ErrDuplicate → 409, anything else →
// 500). Responses are JSON: {"success": ...} on 200, {"error": ...} otherwise.
package httpv1

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/syhlion/gua/delayquene"
	guaproto "github.com/syhlion/gua/proto"
)

// maxBodyBytes bounds a request body (payloads are stored in Postgres).
const maxBodyBytes = 1 << 20 // 1 MiB

var logger = slog.Default()

// SetLogger replaces the package logger (defaults to slog.Default()).
func SetLogger(l *slog.Logger) {
	if l != nil {
		logger = l
	}
}

// WriteJSON writes the API's JSON envelope: {"success": data} for 200,
// {"error": data} for any other status.
func WriteJSON(w http.ResponseWriter, data any, status int) {
	w.Header().Set("Content-Type", "application/json;charset=UTF-8")
	w.WriteHeader(status)
	var body any
	if status == http.StatusOK {
		body = struct {
			Success any `json:"success"`
		}{data}
	} else {
		body = struct {
			Error any `json:"error"`
		}{data}
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		logger.Warn("write response", "error", err)
	}
}

// writeErr maps a Quene error to a status and writes it. Internal errors are
// logged and hidden behind a generic message.
func writeErr(w http.ResponseWriter, op string, err error) {
	switch {
	case errors.Is(err, delayquene.ErrInvalid):
		WriteJSON(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, delayquene.ErrNotFound):
		WriteJSON(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, delayquene.ErrDuplicate):
		WriteJSON(w, err.Error(), http.StatusConflict)
	default:
		logger.Error(op, "error", err)
		WriteJSON(w, "internal error", http.StatusInternalServerError)
	}
}

// readJSON decodes a bounded JSON body into dst. An empty body leaves dst
// untouched. It writes the error response and returns false on failure.
func readJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			WriteJSON(w, "body too large", http.StatusRequestEntityTooLarge)
			return false
		}
		WriteJSON(w, "read body: "+err.Error(), http.StatusBadRequest)
		return false
	}
	if len(body) == 0 {
		return true
	}
	if err := json.Unmarshal(body, dst); err != nil {
		WriteJSON(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return false
	}
	return true
}

// Version reports the build version. GET /version
func Version(serverVersion string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, serverVersion, http.StatusOK)
	}
}

// Status is a read-only monitoring snapshot. GET /v1/status
func Status(quene delayquene.Quene) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, err := quene.Stats()
		if err != nil {
			writeErr(w, "status", err)
			return
		}
		WriteJSON(w, s, http.StatusOK)
	}
}

// History returns recent execution records for a group.
// GET /v1/groups/{group}/history?limit=N
func History(quene delayquene.Quene) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := 100
		if v := r.URL.Query().Get("limit"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil {
				WriteJSON(w, "limit must be an integer", http.StatusBadRequest)
				return
			}
			limit = n
		}
		entries, err := quene.History(r.PathValue("group"), limit)
		if err != nil {
			writeErr(w, "history", err)
			return
		}
		WriteJSON(w, entries, http.StatusOK)
	}
}

// GroupInfo returns a single group's name. GET /v1/groups/{group}
func GroupInfo(quene delayquene.Quene) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		group, err := quene.GroupInfo(r.PathValue("group"))
		if err != nil {
			writeErr(w, "group info", err)
			return
		}
		WriteJSON(w, group, http.StatusOK)
	}
}

// GetGroupList lists all groups. GET /v1/groups
func GetGroupList(quene delayquene.Quene) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groups, err := quene.QueryGroups()
		if err != nil {
			writeErr(w, "list groups", err)
			return
		}
		WriteJSON(w, groups, http.StatusOK)
	}
}

// GetJobList lists a group's jobs. GET /v1/groups/{group}/jobs
func GetJobList(quene delayquene.Quene) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jobs, err := quene.List(r.PathValue("group"))
		if err != nil {
			writeErr(w, "list jobs", err)
			return
		}
		list := make([]*ResponseJobList, 0, len(jobs))
		for _, v := range jobs {
			list = append(list, &ResponseJobList{
				Name:            v.Name,
				Id:              v.Id,
				Exectime:        v.Exectime,
				IntervalPattern: v.IntervalPattern,
				RequestUrl:      v.RequestUrl,
				Payload:         v.Payload,
				Timeout:         v.Timeout,
				GroupName:       v.GroupName,
				Active:          v.Active,
				Memo:            v.Memo,
			})
		}
		WriteJSON(w, list, http.StatusOK)
	}
}

// RegisterGroup creates a group. POST /v1/groups  body: {group_name}
func RegisterGroup(quene delayquene.Quene) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var p RegisterGroupPayload
		if !readJSON(w, r, &p) {
			return
		}
		if err := quene.RegisterGroup(p.GroupName); err != nil {
			writeErr(w, "register group", err)
			return
		}
		logger.Info("group registered", "group", p.GroupName)
		WriteJSON(w, p.GroupName, http.StatusOK)
	}
}

// RemoveGroup deletes a group and all its jobs. DELETE /v1/groups/{group}
func RemoveGroup(quene delayquene.Quene) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		group := r.PathValue("group")
		if err := quene.RemoveGroup(group); err != nil {
			writeErr(w, "remove group", err)
			return
		}
		logger.Info("group removed", "group", group)
		WriteJSON(w, "ok", http.StatusOK)
	}
}

// AddJob schedules a job in a group.
// POST /v1/groups/{group}/jobs  body: {name, exec_time, interval_pattern, request_url, ...}
func AddJob(quene delayquene.Quene) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		group := r.PathValue("group")
		var p AddJobPayload
		if !readJSON(w, r, &p) {
			return
		}
		jobID := p.JobId
		if jobID == "" {
			jobID = quene.GenerateUID()
		}
		job := &guaproto.Job{
			Name:            p.Name,
			GroupName:       group,
			Id:              jobID,
			Exectime:        p.Exectime,
			Timeout:         p.Timeout,
			IntervalPattern: p.IntervalPattern,
			RequestUrl:      p.RequestUrl,
			Payload:         p.Payload,
			Active:          true,
			Memo:            p.Memo,
		}
		if err := quene.Push(job); err != nil {
			writeErr(w, "add job", err)
			return
		}
		logger.Info("job added", "group", group, "job", jobID, "name", p.Name,
			"interval", job.IntervalPattern, "exec_time", job.Exectime)
		WriteJSON(w, jobID, http.StatusOK)
	}
}

// EditJob updates a job's request_url/payload.
// PATCH /v1/groups/{group}/jobs/{job}  body: {request_url, payload}
func EditJob(quene delayquene.Quene) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		group, jobID := r.PathValue("group"), r.PathValue("job")
		var p EditJobPayload
		if !readJSON(w, r, &p) {
			return
		}
		if p.RequestUrl == "" {
			WriteJSON(w, "request_url is required", http.StatusBadRequest)
			return
		}
		if err := quene.Edit(group, jobID, p.RequestUrl, p.Payload); err != nil {
			writeErr(w, "edit job", err)
			return
		}
		logger.Info("job edited", "group", group, "job", jobID)
		WriteJSON(w, jobID, http.StatusOK)
	}
}

// DeleteJob removes a single job by id. DELETE /v1/groups/{group}/jobs/{job}
func DeleteJob(quene delayquene.Quene) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		group, jobID := r.PathValue("group"), r.PathValue("job")
		if err := quene.Delete(group, jobID); err != nil {
			writeErr(w, "delete job", err)
			return
		}
		logger.Info("job deleted", "group", group, "job", jobID)
		WriteJSON(w, "ok", http.StatusOK)
	}
}

// DeleteJobs clears a group's jobs. DELETE /v1/groups/{group}/jobs
// With ?name=<job_name> it deletes only jobs with that name. Returns the
// number of jobs deleted.
func DeleteJobs(quene delayquene.Quene) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		group := r.PathValue("group")
		name := r.URL.Query().Get("name")
		n, err := quene.DeleteJobs(group, name)
		if err != nil {
			writeErr(w, "delete jobs", err)
			return
		}
		logger.Info("jobs deleted", "group", group, "name", name, "count", n)
		WriteJSON(w, n, http.StatusOK)
	}
}

// PauseJob pauses a job. POST /v1/groups/{group}/jobs/{job}/pause
func PauseJob(quene delayquene.Quene) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		group, jobID := r.PathValue("group"), r.PathValue("job")
		if err := quene.Pause(group, jobID); err != nil {
			writeErr(w, "pause job", err)
			return
		}
		logger.Info("job paused", "group", group, "job", jobID)
		WriteJSON(w, "ok", http.StatusOK)
	}
}

// ActiveJob (re)activates a job. POST /v1/groups/{group}/jobs/{job}/activate
// body: {exec_time}  (omitted or 0 = now)
func ActiveJob(quene delayquene.Quene) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		group, jobID := r.PathValue("group"), r.PathValue("job")
		var p ActiveJobPayload
		if !readJSON(w, r, &p) {
			return
		}
		if err := quene.Active(group, jobID, p.Exectime); err != nil {
			writeErr(w, "activate job", err)
			return
		}
		logger.Info("job activated", "group", group, "job", jobID, "exec_time", p.Exectime)
		WriteJSON(w, "ok", http.StatusOK)
	}
}
