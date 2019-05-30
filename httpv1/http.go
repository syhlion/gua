package httpv1

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/gomodule/redigo/redis"
	"github.com/gorilla/mux"
	"github.com/pquerna/otp/totp"
	"github.com/sirupsen/logrus"
	"github.com/syhlion/gua/delayquene"
	"github.com/syhlion/gua/luacore"
	"github.com/syhlion/gua/migrate"
	guaproto "github.com/syhlion/gua/proto"
	"github.com/syhlion/restresp"
)

var (
	logger *logrus.Logger
)

func SetLogger(logger *logrus.Logger) {
	logger = logger
}

func Version(serverVersion string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		restresp.Write(w, serverVersion, http.StatusOK)
	}
}
func AddFunc(quene delayquene.Quene, apiRedis *redis.Pool, lpool *luacore.LStatePool) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logger.Warnf("Error reading body: %v", err)
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		payload := &AddFuncPayload{}
		err = json.Unmarshal(body, payload)
		if err != nil {
			logger.Warnf("Error json umnarsal: %v", err)
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		_, err = quene.GroupInfo(payload.GroupName)
		if err != nil {
			logger.Warnf("Error get group: %v", err)
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		funcKey := fmt.Sprintf("FUNC-%s-%s", payload.GroupName, payload.Name)
		c := apiRedis.Get()
		defer c.Close()
		var otpToken string
		if payload.UseOtp {
			if payload.DisableGroupOtp {
				kkey, err := totp.Generate(totp.GenerateOpts{
					Issuer:      payload.GroupName,
					AccountName: payload.Name,
				})
				if err != nil {
					logger.Warnf("Error set lua: %v", err)
					restresp.Write(w, err.Error(), http.StatusBadRequest)
					return
				}
				otpToken = kkey.Secret()
			}
		}
		f := &guaproto.Func{
			Name:            payload.Name,
			GroupName:       payload.GroupName,
			UseOtp:          payload.UseOtp,
			DisableGroupOtp: payload.DisableGroupOtp,
			Memo:            payload.Memo,
			OtpToken:        otpToken,
			LuaBody:         []byte(payload.LuaBody),
		}
		b, err := proto.Marshal(f)
		if err != nil {
			logger.Warnf("Error set lua: %v", err)
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		_, err = c.Do("SET", funcKey, b)
		if err != nil {
			logger.Warnf("Error set lua: %v", err)
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		if otpToken != "" {
			restresp.Write(w, otpToken, http.StatusOK)
		} else {
			restresp.Write(w, payload.Name, http.StatusOK)
		}

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
func GetNodeList(quene delayquene.Quene) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		groupName := vars["group_name"]
		nodes, err := quene.QueryNodes(groupName)
		if err != nil {
			logger.Warnf("get node list error: %v", err)
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		restresp.Write(w, nodes, http.StatusOK)
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
				OtpToken:        v.OtpToken,
				Exectime:        v.Exectime,
				IntervalPattern: v.IntervalPattern,
				RequestUrl:      v.RequestUrl,
				ExecCmd:         string(v.ExecCmd),
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
			restresp.Write(w, err, http.StatusBadRequest)
			return
		}
		payload := &RegisterGroupPayload{}
		err = json.Unmarshal(body, payload)
		if err != nil {
			logger.Warnf("Error json umnarsal: %v", err)
			restresp.Write(w, err, http.StatusBadRequest)
			return
		}
		otp, err := quene.RegisterGroup(payload.GroupName)
		if err != nil {
			logger.Warnf("Error json umnarsal: %v", err)
			restresp.Write(w, err, http.StatusBadRequest)
			return
		}
		restresp.Write(w, otp, http.StatusOK)

	}
}
func EditJob(quene delayquene.Quene) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logger.Warnf("Error reading body: %v", err)
			restresp.Write(w, err, http.StatusBadRequest)
			return
		}

		payload := &EditJobPayload{}
		err = json.Unmarshal(body, payload)
		if err != nil {
			logger.Warnf("Error json umnarsal: %v", err)
			restresp.Write(w, err, http.StatusBadRequest)
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
		if payload.ExecCmd == "" {
			restresp.Write(w, "payload no ExecCmd", http.StatusBadRequest)
			return
		}
		err = quene.Edit(payload.GroupName, payload.Id, payload.RequestUrl, payload.ExecCmd)
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
			restresp.Write(w, err, http.StatusBadRequest)
			return
		}

		payload := &AddJobPayload{}
		err = json.Unmarshal(body, payload)
		if err != nil {
			logger.Warnf("Error json umnarsal: %v", err)
			restresp.Write(w, err, http.StatusBadRequest)
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
		job := &guaproto.Job{
			Name:            payload.Name,
			GroupName:       payload.GroupName,
			Id:              quene.GenerateUID(),
			Exectime:        payload.Exectime,
			Timeout:         payload.Timeout,
			IntervalPattern: payload.IntervalPattern,
			RequestUrl:      payload.RequestUrl,
			ExecCmd:         []byte(payload.ExecCommand),
			Active:          true,
			Memo:            payload.Memo,
		}
		if !payload.UseGroupOtp {
			kkey, err := totp.Generate(totp.GenerateOpts{
				Issuer:      job.Id,
				AccountName: payload.GroupName,
			})
			if err != nil {
				restresp.Write(w, "otp generate error", http.StatusBadRequest)
				return
			}
			job.OtpToken = kkey.Secret()
		}
		err = quene.Push(job)
		if err != nil {
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}

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
		restresp.Write(w, "ok", http.StatusOK)
		return

	}
}
func ActiveJob(quene delayquene.Quene) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logger.Warnf("Error reading body: %v", err)
			restresp.Write(w, err, http.StatusBadRequest)
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
		restresp.Write(w, "ok", http.StatusOK)
		return

	}
}
