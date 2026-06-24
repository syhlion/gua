package httpv1

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"fmt"
	"github.com/gorilla/mux"
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

// Status is a read-only monitoring endpoint: ready-queue depth, orphaned
// bucket backlog, and per-slot cluster health.
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
func History(quene delayquene.Quene) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		groupName := mux.Vars(r)["group_name"]
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
func GroupInfo(quene delayquene.Quene) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		groupName := vars["group_name"]
		group, err := quene.GroupInfo(groupName)
		if err != nil {
			logger.Warn(fmt.Sprintf("group info error: %v", err))
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		restresp.Write(w, group, http.StatusOK)
		return

	}
}
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
		return

	}
}
func GetJobList(quene delayquene.Quene) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		groupName := vars["group_name"]
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
		return

	}
}
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
func EditJob(quene delayquene.Quene) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
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
		if payload.GroupName == "" {
			restresp.Write(w, "payload no group_name", http.StatusBadRequest)
			return
		}
		if payload.Id == "" {
			restresp.Write(w, "payload no job id ", http.StatusBadRequest)
			return
		}
		if payload.RequestUrl == "" {
			restresp.Write(w, "payload no request_url", http.StatusBadRequest)
			return
		}
		err = quene.Edit(payload.GroupName, payload.Id, payload.RequestUrl, payload.Payload)
		if err != nil {
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}

		//nodeId := strconv.FormatInt(node.id, 10)
		restresp.Write(w, payload.Id, http.StatusOK)
		//w.Write([]byte(nodeId))
	}
}
func AddJob(quene delayquene.Quene) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
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
		if payload.GroupName == "" {
			restresp.Write(w, "payload no group_name", http.StatusBadRequest)
			return
		}
		exists, err := quene.ExistsGroup(payload.GroupName)
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
			GroupName:       payload.GroupName,
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

		//nodeId := strconv.FormatInt(node.id, 10)
		restresp.Write(w, job.Id, http.StatusOK)
		//w.Write([]byte(nodeId))
	}
}
func RemoveJob(quene delayquene.Quene) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logger.Warn(fmt.Sprintf("Error reading body: %v", err))
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		payload := &JobControlPayload{}
		json.Unmarshal(body, payload)
		if err != nil {
			logger.Warn(fmt.Sprintf("Error json umnarsal: %v", err))
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		err = quene.Delete(payload.GroupName, payload.JobId)
		if err != nil {
			logger.Warn(fmt.Sprintf("jobs error: %v", err))
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		logger.Info(fmt.Sprintf("success remove job: %v", payload))
		restresp.Write(w, "ok", http.StatusOK)
		return

	}
}
func PauseJob(quene delayquene.Quene) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logger.Warn(fmt.Sprintf("Error reading body: %v", err))
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		payload := &JobControlPayload{}
		json.Unmarshal(body, payload)
		if err != nil {
			logger.Warn(fmt.Sprintf("Error json umnarsal: %v", err))
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		err = quene.Pause(payload.GroupName, payload.JobId)
		if err != nil {
			logger.Warn(fmt.Sprintf("jobs error: %v", err))
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		logger.Info(fmt.Sprintf("success pause job: %v", payload))
		restresp.Write(w, "ok", http.StatusOK)
		return

	}
}
func ActiveJob(quene delayquene.Quene) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logger.Warn(fmt.Sprintf("Error reading body: %v", err))
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		payload := &ActiveJobPayload{}
		err = json.Unmarshal(body, payload)
		if err != nil {
			logger.Warn(fmt.Sprintf("Error json umnarsal: %v", err))
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		err = quene.Active(payload.GroupName, payload.JobId, payload.Exectime)
		if err != nil {
			logger.Warn(fmt.Sprintf("jobs error: %v", err))
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		logger.Info(fmt.Sprintf("success active job: %v", payload))
		restresp.Write(w, "ok", http.StatusOK)
		return

	}
}

func GroupJobClear(quene delayquene.Quene) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		groupName := vars["group_name"]
		jobs, err := quene.List(groupName)
		if err != nil {
			logger.Warn(fmt.Sprintf("list jobs error: %v", err))
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}

		for _, v := range jobs {
			err = quene.Delete(groupName, v.Id)
			if err != nil {
				logger.Warn(fmt.Sprintf("delete jobs error: %v", err))
				restresp.Write(w, err.Error(), http.StatusBadRequest)
				return
			}
		}

		restresp.Write(w, "ok", http.StatusOK)
		return
	}
}

func RemoveJobsByJobName(quene delayquene.Quene) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		groupName := vars["group_name"]
		removeJobName := vars["job_name"]
		allJobs, err := quene.List(groupName)
		if err != nil {
			logger.Warn(fmt.Sprintf("RemoveJobsByJobName - list jobs error: %v", err))
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}

		for _, v := range allJobs {
			if removeJobName == v.Name {
				err = quene.Delete(groupName, v.Id)
				if err != nil {
					logger.Warn(fmt.Sprintf("RemoveJobsByJobName delete job error: %v", err))
					restresp.Write(w, err.Error(), http.StatusBadRequest)
					return
				}
			}
		}

		restresp.Write(w, "ok", http.StatusOK)
		return
	}
}

func RemoveGroup(quene delayquene.Quene) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// clear this group all job first
		GroupJobClear(quene)

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logger.Warn(fmt.Sprintf("RemoveGroup Error reading body: %v", err))
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		payload := &RegisterGroupPayload{}
		err = json.Unmarshal(body, payload)
		if err != nil {
			logger.Warn(fmt.Sprintf("RemoveGroup Error json umnarsal: %v", err))
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		removeGroupErr := quene.RemoveGroup(payload.GroupName)
		if removeGroupErr != nil {
			logger.Warn(fmt.Sprintf("RemoveGroup Error: %v", removeGroupErr))
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		restresp.Write(w, "ok", http.StatusOK)
	}
}
