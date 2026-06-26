package httpv1

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"fmt"
	"github.com/syhlion/gua/delayquene"
	guaproto "github.com/syhlion/gua/proto"
	"github.com/syhlion/restresp"
	"log/slog"
)

var (
	logger  *slog.Logger
	jobRe   = regexp.MustCompile(`^([a-zA-Z0-9_]+)$`)
	groupRe = regexp.MustCompile(`^([a-zA-Z0-9_]+)$`)
)

func SetLogger(l *slog.Logger) {
	logger = l
}

func Version(serverVersion string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		restresp.Write(w, serverVersion, http.StatusOK)
	}
}

// Status is a read-only monitoring endpoint: pending-queue depth and queue
// health (no slots on the Postgres backend).
func Status(quene delayquene.Quene) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		s, err := quene.Stats()
		if err != nil {
			logger.Warn(fmt.Sprintf("status error: %v", err))
			restresp.Write(w, err.Error(), http.StatusInternalServerError)
			return
		}
		restresp.Write(w, s, http.StatusOK)
	}
}

// History returns recent execution records for a group (Monitor Tier 2).
// GET /v1/groups/{group}/history
func History(quene delayquene.Quene) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		groupName := r.PathValue("group")
		limit := 100
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				limit = n
			}
		}
		entries, err := quene.History(groupName, limit)
		if err != nil {
			logger.Warn(fmt.Sprintf("history error: %v", err))
			restresp.Write(w, err.Error(), http.StatusInternalServerError)
			return
		}
		restresp.Write(w, entries, http.StatusOK)
	}
}

// GroupInfo returns a single group's metadata. GET /v1/groups/{group}
func GroupInfo(quene delayquene.Quene) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		groupName := r.PathValue("group")
		group, err := quene.GroupInfo(groupName)
		if err != nil {
			logger.Warn(fmt.Sprintf("group info error: %v", err))
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		restresp.Write(w, group, http.StatusOK)
	}
}

// GetGroupList lists all groups. GET /v1/groups
func GetGroupList(quene delayquene.Quene) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		groups, err := quene.QueryGroups()
		if err != nil {
			logger.Warn(fmt.Sprintf("get group list error: %v", err))
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		list := make([]string, 0)
		for _, v := range groups {
			s := strings.TrimPrefix(v, "USER_")
			list = append(list, s)
		}
		restresp.Write(w, list, http.StatusOK)
	}
}

// GetJobList lists a group's jobs. GET /v1/groups/{group}/jobs
func GetJobList(quene delayquene.Quene) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		groupName := r.PathValue("group")
		jobs, err := quene.List(groupName)
		if err != nil {
			logger.Warn(fmt.Sprintf("jobs error: %v", err))
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		joblist := make([]*ResponseJobList, 0)
		for _, v := range jobs {
			job := &ResponseJobList{
				Name:            v.Name,
				Id:              v.Id,
				Exectime:        v.Exectime,
				IntervalPattern: v.IntervalPattern,
				RequestUrl:      v.RequestUrl,
				Payload:         v.Payload,
				GroupName:       v.GroupName,
				Active:          v.Active,
				Memo:            v.Memo,
			}
			joblist = append(joblist, job)
		}
		restresp.Write(w, joblist, http.StatusOK)
	}
}

// RegisterGroup creates a group. POST /v1/groups  body: {group_name}
func RegisterGroup(quene delayquene.Quene) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logger.Warn(fmt.Sprintf("Error reading body: %v", err))
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		payload := &RegisterGroupPayload{}
		err = json.Unmarshal(body, payload)
		if err != nil {
			logger.Warn(fmt.Sprintf("Error json umnarsal: %v", err))
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		if !groupRe.MatchString(payload.GroupName) {
			restresp.Write(w, "groupname illegal", http.StatusBadRequest)
			return
		}
		if len([]rune(payload.GroupName)) > 22 {
			restresp.Write(w, "groupname too long", http.StatusBadRequest)
			return
		}
		err = quene.RegisterGroup(payload.GroupName)
		if err != nil {
			logger.Warn(fmt.Sprintf("RegisterGroup Error: %v", err))
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		restresp.Write(w, payload.GroupName, http.StatusOK)
	}
}

// EditJob updates a job's request_url/payload.
// PATCH /v1/groups/{group}/jobs/{job}  body: {request_url, payload}
func EditJob(quene delayquene.Quene) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		groupName := r.PathValue("group")
		jobID := r.PathValue("job")
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logger.Warn(fmt.Sprintf("Error reading body: %v", err))
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		payload := &EditJobPayload{}
		err = json.Unmarshal(body, payload)
		if err != nil {
			logger.Warn(fmt.Sprintf("Error json umnarsal: %v", err))
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		if payload.RequestUrl == "" {
			restresp.Write(w, "payload no request_url", http.StatusBadRequest)
			return
		}
		err = quene.Edit(groupName, jobID, payload.RequestUrl, payload.Payload)
		if err != nil {
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		restresp.Write(w, jobID, http.StatusOK)
	}
}

// AddJob schedules a job in a group.
// POST /v1/groups/{group}/jobs  body: {name, exec_time, interval_pattern, request_url, ...}
func AddJob(quene delayquene.Quene) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		groupName := r.PathValue("group")
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logger.Warn(fmt.Sprintf("Error reading body: %v", err))
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		payload := &AddJobPayload{}
		err = json.Unmarshal(body, payload)
		var jobId string
		if err != nil {
			logger.Warn(fmt.Sprintf("Error json umnarsal: %v", err))
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		if payload.Name == "" {
			restresp.Write(w, "payload no name", http.StatusBadRequest)
			return
		}
		if payload.Exectime < 0 {
			restresp.Write(w, "payload exec_time error", http.StatusBadRequest)
			return
		}
		if payload.IntervalPattern == "" {
			restresp.Write(w, "payload no interval_pattern", http.StatusBadRequest)
			return
		}
		if payload.RequestUrl == "" {
			restresp.Write(w, "payload no request_url", http.StatusBadRequest)
			return
		}
		exists, err := quene.ExistsGroup(groupName)
		if err != nil {
			restresp.Write(w, "exists group err", http.StatusBadRequest)
			return
		}
		if exists != 1 {
			restresp.Write(w, "no group", http.StatusBadRequest)
			return
		}
		if payload.JobId == "" {
			jobId = quene.GenerateUID()
		} else {
			jobId = payload.JobId
		}
		if !jobRe.MatchString(jobId) {
			restresp.Write(w, "jobid illegal", http.StatusBadRequest)
			return
		}
		if len([]rune(jobId)) > 22 {
			restresp.Write(w, "jobid too long", http.StatusBadRequest)
			return
		}
		job := &guaproto.Job{
			Name:            payload.Name,
			GroupName:       groupName,
			Id:              jobId,
			Exectime:        payload.Exectime,
			Timeout:         payload.Timeout,
			IntervalPattern: payload.IntervalPattern,
			RequestUrl:      payload.RequestUrl,
			Payload:         payload.Payload,
			Active:          true,
			Memo:            payload.Memo,
		}
		err = quene.Push(job)
		if err != nil {
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		logger.Info(fmt.Sprintf("success add job: %v, origin payload: %v", job, payload))
		restresp.Write(w, job.Id, http.StatusOK)
	}
}

// DeleteJob removes a single job by id.
// DELETE /v1/groups/{group}/jobs/{job}
func DeleteJob(quene delayquene.Quene) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		groupName := r.PathValue("group")
		jobID := r.PathValue("job")
		if err := quene.Delete(groupName, jobID); err != nil {
			logger.Warn(fmt.Sprintf("jobs error: %v", err))
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		logger.Info(fmt.Sprintf("success remove job: group=%s id=%s", groupName, jobID))
		restresp.Write(w, "ok", http.StatusOK)
	}
}

// DeleteJobs clears a group's jobs. DELETE /v1/groups/{group}/jobs
// With ?name=<job_name> it deletes only jobs matching that name; otherwise it
// clears every job in the group.
func DeleteJobs(quene delayquene.Quene) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		groupName := r.PathValue("group")
		filterName := r.URL.Query().Get("name")
		jobs, err := quene.List(groupName)
		if err != nil {
			logger.Warn(fmt.Sprintf("list jobs error: %v", err))
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		for _, v := range jobs {
			if filterName != "" && v.Name != filterName {
				continue
			}
			if err = quene.Delete(groupName, v.Id); err != nil {
				logger.Warn(fmt.Sprintf("delete jobs error: %v", err))
				restresp.Write(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
		restresp.Write(w, "ok", http.StatusOK)
	}
}

// PauseJob pauses a job. POST /v1/groups/{group}/jobs/{job}/pause
func PauseJob(quene delayquene.Quene) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		groupName := r.PathValue("group")
		jobID := r.PathValue("job")
		if err := quene.Pause(groupName, jobID); err != nil {
			logger.Warn(fmt.Sprintf("jobs error: %v", err))
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		logger.Info(fmt.Sprintf("success pause job: group=%s id=%s", groupName, jobID))
		restresp.Write(w, "ok", http.StatusOK)
	}
}

// ActiveJob (re)activates a job. POST /v1/groups/{group}/jobs/{job}/activate
// body: {exec_time}
func ActiveJob(quene delayquene.Quene) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		groupName := r.PathValue("group")
		jobID := r.PathValue("job")
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logger.Warn(fmt.Sprintf("Error reading body: %v", err))
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		payload := &ActiveJobPayload{}
		if len(body) > 0 {
			if err = json.Unmarshal(body, payload); err != nil {
				logger.Warn(fmt.Sprintf("Error json umnarsal: %v", err))
				restresp.Write(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
		if err = quene.Active(groupName, jobID, payload.Exectime); err != nil {
			logger.Warn(fmt.Sprintf("jobs error: %v", err))
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		logger.Info(fmt.Sprintf("success active job: group=%s id=%s", groupName, jobID))
		restresp.Write(w, "ok", http.StatusOK)
	}
}

// RemoveGroup deletes a group and all its jobs. DELETE /v1/groups/{group}
func RemoveGroup(quene delayquene.Quene) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		groupName := r.PathValue("group")
		// clear this group's jobs first
		jobs, err := quene.List(groupName)
		if err != nil {
			logger.Warn(fmt.Sprintf("RemoveGroup list jobs error: %v", err))
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		for _, v := range jobs {
			if err = quene.Delete(groupName, v.Id); err != nil {
				logger.Warn(fmt.Sprintf("RemoveGroup delete job error: %v", err))
				restresp.Write(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
		if err = quene.RemoveGroup(groupName); err != nil {
			logger.Warn(fmt.Sprintf("RemoveGroup Error: %v", err))
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		restresp.Write(w, "ok", http.StatusOK)
	}
}
