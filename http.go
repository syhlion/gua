package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/gogo/protobuf/proto"
	"github.com/gomodule/redigo/redis"
	"github.com/gorilla/mux"
	"github.com/pquerna/otp/totp"
	"github.com/syhlion/gua/delayquene"
	"github.com/syhlion/gua/luacore"
	guaproto "github.com/syhlion/gua/proto"
	"github.com/syhlion/restresp"
)

func AddFunc(group *Group, apiRedis *redis.Pool, lpool *luacore.LStatePool) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Printf("Error reading body: %v", err)
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		payload := &AddFuncPayload{}
		err = json.Unmarshal(body, payload)
		if err != nil {
			log.Printf("Error json umnarsal: %v", err)
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		_, err = group.GetGroup(payload.GroupName)
		if err != nil {
			log.Printf("Error get group: %v", err)
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
					log.Printf("Error set lua: %v", err)
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
			OtpToken:        otpToken,
			LuaBody:         []byte(payload.LuaBody),
		}
		b, err := proto.Marshal(f)
		if err != nil {
			log.Printf("Error set lua: %v", err)
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		_, err = c.Do("SET", funcKey, b)
		if err != nil {
			log.Printf("Error set lua: %v", err)
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
func GetNodeList(quene delayquene.Quene) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		groupName := vars["group_name"]
		nodes, err := quene.QueryNodes(groupName)
		if err != nil {
			log.Printf("get node list error: %v", err)
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
			log.Printf("group info error: %v", err)
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
			log.Printf("get group list error: %v", err)
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		restresp.Write(w, groups, http.StatusOK)
		return

	}
}
func GetJobList(quene delayquene.Quene) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		groupName := vars["group_name"]
		jobs, err := quene.List(groupName)
		if err != nil {
			log.Printf("jobs error: %v", err)
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
			}
			joblist = append(joblist, job)
		}
		restresp.Write(w, joblist, http.StatusOK)
		return

	}
}
func RegisterGroup(quene delayquene.Quene, conf *Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Printf("Error reading body: %v", err)
			restresp.Write(w, err, http.StatusBadRequest)
			return
		}
		payload := &RegisterGroupPayload{}
		err = json.Unmarshal(body, payload)
		if err != nil {
			log.Printf("Error json umnarsal: %v", err)
			restresp.Write(w, err, http.StatusBadRequest)
			return
		}
		otp, err := quene.RegisterGroup(payload.GroupName)
		if err != nil {
			log.Printf("Error json umnarsal: %v", err)
			restresp.Write(w, err, http.StatusBadRequest)
			return
		}
		restresp.Write(w, otp, http.StatusOK)

	}
}
func AddJob(quene delayquene.Quene, conf *Config) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Printf("Error reading body: %v", err)
			restresp.Write(w, err, http.StatusBadRequest)
			return
		}

		payload := &AddJobPayload{}
		err = json.Unmarshal(body, payload)
		if err != nil {
			log.Printf("Error json umnarsal: %v", err)
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
		/*
			if payload.ExecCommand == "" {
				restresp.Write(w, "payload no exec_command", http.StatusBadRequest)
				return
			}
		*/
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
		}
		if !payload.UseGroupOtp {
			kkey, err := totp.Generate(totp.GenerateOpts{
				Issuer:      conf.Mac,
				AccountName: conf.ExternalIp,
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
			log.Printf("Error reading body: %v", err)
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		payload := &JobControlPayload{}
		json.Unmarshal(body, payload)
		if err != nil {
			log.Printf("Error json umnarsal: %v", err)
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		err = quene.Delete(payload.GroupName, payload.JobId)
		if err != nil {
			log.Printf("jobs error: %v", err)
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
			log.Printf("Error reading body: %v", err)
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		payload := &JobControlPayload{}
		json.Unmarshal(body, payload)
		if err != nil {
			log.Printf("Error json umnarsal: %v", err)
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		err = quene.Pause(payload.GroupName, payload.JobId)
		if err != nil {
			log.Printf("jobs error: %v", err)
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
			log.Printf("Error reading body: %v", err)
			restresp.Write(w, err, http.StatusBadRequest)
			return
		}
		payload := &ActiveJobPayload{}
		err = json.Unmarshal(body, payload)
		if err != nil {
			log.Printf("Error json umnarsal: %v", err)
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		err = quene.Active(payload.GroupName, payload.JobId, payload.Exectime)
		if err != nil {
			log.Printf("jobs error: %v", err)
			restresp.Write(w, err.Error(), http.StatusBadRequest)
			return
		}
		restresp.Write(w, "ok", http.StatusOK)
		return

	}
}
