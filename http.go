package main

import (
	"io/ioutil"
	"log"
	"net/http"

	"github.com/pquerna/otp/totp"
	"github.com/syhlion/gua/delayquene"
	guaproto "github.com/syhlion/gua/proto"
	"github.com/syhlion/restresp"
)

func GetJobList(quene delayquene.Quene) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {

	}
}
func GetJob(quene delayquene.Quene) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {

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

	}
}
func EditJob(nquene delayquene.Quene) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {

	}
}
