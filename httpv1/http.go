package httpv1

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/syhlion/gua/delayquene"
	"github.com/syhlion/gua/migrate"
	guaproto "github.com/syhlion/gua/proto"
	"github.com/syhlion/restresp"
)

var (
	logger  *logrus.Logger
	jobRe   = regexp.MustCompile(`^([a-zA-Z0-9_]+)$`)
	groupRe = regexp.MustCompile(`^([a-zA-Z0-9_]+)$`)
)

func SetLogger(l *logrus.Logger) {
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
			logger.Warnf("status error: %v", err)
			restresp.Write(w, err.Error(), http.StatusInternalServerError)
			return
		}
		restresp.Write(w, s, http.StatusOK)
	}
}
func Import(m *migrate.Migrate) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return
		}
		err = m.Import(b)
		if err != nil {
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		restresp.Write(w, "success", http.StatusOK)

		return

	}
}
func DumpBy(m *migrate.Migrate) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		groupName := vars["group_name"]
		buff, err := m.Dump(groupName)
		if err != nil {
			logger.Errorf("Dump error: %#v", err)
			return
		}
		tf := time.Now().Format("2006-01-02-150405")
		fileName := fmt.Sprintf("%s-%s-backup.tar.gz", tf, groupName)
		w.Header().Set("Content-Disposition", "attachment; filename="+fileName)
		w.Header().Set("Content-Type", r.Header.Get("Content-Type"))
		b := bufio.NewReader(buff)
		io.Copy(w, b)
		return

	}
}
func DumpAll(m *migrate.Migrate) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		buff, err := m.Dump("*")
		if err != nil {
			logger.Errorf("Dump error: %#v", err)
			return
		}
		tf := time.Now().Format("2006-01-02-150405")
		fileName := fmt.Sprintf("%s-backup.tar.gz", tf)
		w.Header().Set("Content-Disposition", "attachment; filename="+fileName)
		w.Header().Set("Content-Type", r.Header.Get("Content-Type"))
		b := bufio.NewReader(buff)
		io.Copy(w, b)
		return

	}
}
func GroupInfo(quene delayquene.Quene) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		groupName := vars["group_name"]
		group, err := quene.GroupInfo(groupName)
		if err != nil {
			logger.Warnf("group info error: %v", err)
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
			logger.Warnf("get group list error: %v", err)
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
			logger.Warnf("jobs error: %v", err)
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
			logger.Warnf("Error reading body: %v", err)
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		payload := &RegisterGroupPayload{}
		err = json.Unmarshal(body, payload)
		if err != nil {
			logger.Warnf("Error json umnarsal: %v", err)
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
			logger.Warnf("RegisterGroup Error: %v", err)
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
			logger.Warnf("Error reading body: %v", err)
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}

		payload := &EditJobPayload{}
		err = json.Unmarshal(body, payload)
		if err != nil {
			logger.Warnf("Error json umnarsal: %v", err)
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
			logger.Warnf("Error reading body: %v", err)
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}

		payload := &AddJobPayload{}
		err = json.Unmarshal(body, payload)
		var jobId string
		if err != nil {
			logger.Warnf("Error json umnarsal: %v", err)
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
		logger.Infof("success add job: %v, origin payload: %v", job, payload)

		//nodeId := strconv.FormatInt(node.id, 10)
		restresp.Write(w, job.Id, http.StatusOK)
		//w.Write([]byte(nodeId))
	}
}
func RemoveJob(quene delayquene.Quene) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logger.Warnf("Error reading body: %v", err)
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		payload := &JobControlPayload{}
		json.Unmarshal(body, payload)
		if err != nil {
			logger.Warnf("Error json umnarsal: %v", err)
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		err = quene.Delete(payload.GroupName, payload.JobId)
		if err != nil {
			logger.Warnf("jobs error: %v", err)
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		logger.Infof("success remove job: %v", payload)
		restresp.Write(w, "ok", http.StatusOK)
		return

	}
}
func PauseJob(quene delayquene.Quene) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logger.Warnf("Error reading body: %v", err)
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		payload := &JobControlPayload{}
		json.Unmarshal(body, payload)
		if err != nil {
			logger.Warnf("Error json umnarsal: %v", err)
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		err = quene.Pause(payload.GroupName, payload.JobId)
		if err != nil {
			logger.Warnf("jobs error: %v", err)
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		logger.Infof("success pause job: %v", payload)
		restresp.Write(w, "ok", http.StatusOK)
		return

	}
}
func ActiveJob(quene delayquene.Quene) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logger.Warnf("Error reading body: %v", err)
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		payload := &ActiveJobPayload{}
		err = json.Unmarshal(body, payload)
		if err != nil {
			logger.Warnf("Error json umnarsal: %v", err)
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		err = quene.Active(payload.GroupName, payload.JobId, payload.Exectime)
		if err != nil {
			logger.Warnf("jobs error: %v", err)
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		logger.Infof("success active job: %v", payload)
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
			logger.Warnf("list jobs error: %v", err)
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}

		for _, v := range jobs {
			err = quene.Delete(groupName, v.Id)
			if err != nil {
				logger.Warnf("delete jobs error: %v", err)
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
			logger.Warnf("RemoveJobsByJobName - list jobs error: %v", err)
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}

		for _, v := range allJobs {
			if removeJobName == v.Name {
				err = quene.Delete(groupName, v.Id)
				if err != nil {
					logger.Warnf("RemoveJobsByJobName delete job error: %v", err)
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
			logger.Warnf("RemoveGroup Error reading body: %v", err)
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		payload := &RegisterGroupPayload{}
		err = json.Unmarshal(body, payload)
		if err != nil {
			logger.Warnf("RemoveGroup Error json umnarsal: %v", err)
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		removeGroupErr := quene.RemoveGroup(payload.GroupName)
		if removeGroupErr != nil {
			logger.Warnf("RemoveGroup Error: %v", removeGroupErr)
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		restresp.Write(w, "ok", http.StatusOK)
	}
}
