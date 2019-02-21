package main

import (
	"errors"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/pquerna/otp/totp"
	"github.com/syhlion/gua/delayquene"
	guaproto "github.com/syhlion/gua/proto"
	"github.com/syhlion/restresp"
)

func GetJob(quene delayquene.Quene) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {

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
		json.Unmarshal(body, payload)
		if err != nil {
			log.Printf("Error json umnarsal: %v", err)
			restresp.Write(w, err, http.StatusBadRequest)
			return
		}
		if payload.Name != "" {
			restresp.Write(w, errors.New("payload no name"), http.StatusBadRequest)
			return
		}
		if payload.Exectime >= 0 {
			restresp.Write(w, errors.New("payload exec_time error"), http.StatusBadRequest)
			return
		}
		if payload.IntervalPattern != "" {
			restresp.Write(w, errors.New("payload no interval_pattern"), http.StatusBadRequest)
			return
		}
		if payload.RequestUrl != "" {
			restresp.Write(w, errors.New("payload no request_url"), http.StatusBadRequest)
			return
		}
		if payload.ExecCommand != "" {
			restresp.Write(w, errors.New("payload no exec_command"), http.StatusBadRequest)
			return
		}
		kkey, err := totp.Generate(totp.GenerateOpts{
			Issuer:      conf.Mac,
			AccountName: conf.ExternalIp,
		})
		if err != nil {
			restresp.Write(w, err, http.StatusBadRequest)
			return
		}
		job := &guaproto.Job{
			Name:            payload.Name,
			Id:              quene.GenerateUID(),
			Exectime:        payload.Exectime,
			OtpToken:        kkey.Secret(),
			Timeout:         payload.Timeout,
			IntervalPattern: payload.IntervalPattern,
			RequestUrl:      payload.RequestUrl,
			ExecCmd:         []byte(payload.ExecCommand),
		}
		quene.Push(job)

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
