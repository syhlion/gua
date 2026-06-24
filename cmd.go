package main

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/syhlion/gua/delayquene"
	"github.com/syhlion/gua/httpv1"
	"github.com/urfave/cli"
)

// buildRouter wires the HTTP API over a Quene.
func buildRouter(quene delayquene.Quene) *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/version", Version(version)).Methods("GET")
	r.HandleFunc("/ui", UI()).Methods("GET")
	sub := r.PathPrefix("/v1/").Subrouter()
	sub.HandleFunc("/status", httpv1.Status(quene)).Methods("GET")
	sub.HandleFunc("/register/group", httpv1.RegisterGroup(quene)).Methods("POST")
	sub.HandleFunc("/remove/group", httpv1.RemoveGroup(quene)).Methods("POST")
	sub.HandleFunc("/add/job", httpv1.AddJob(quene)).Methods("POST")
	sub.HandleFunc("/delete/job", httpv1.RemoveJob(quene)).Methods("POST")
	sub.HandleFunc("/pause/job", httpv1.PauseJob(quene)).Methods("POST")
	sub.HandleFunc("/active/job", httpv1.ActiveJob(quene)).Methods("POST")
	sub.HandleFunc("/edit/job", httpv1.EditJob(quene)).Methods("POST")
	sub.HandleFunc("/group/list", httpv1.GetGroupList(quene)).Methods("GET")
	sub.HandleFunc("/{group_name}/job/list", httpv1.GetJobList(quene)).Methods("GET")
	sub.HandleFunc("/{group_name}/group/info", httpv1.GroupInfo(quene)).Methods("GET")
	sub.HandleFunc("/{group_name}/history", httpv1.History(quene)).Methods("GET")
	sub.HandleFunc("/{group_name}/job/clear", httpv1.GroupJobClear(quene)).Methods(http.MethodDelete)
	sub.HandleFunc("/{group_name}/job/delete/{job_name}", httpv1.RemoveJobsByJobName(quene)).Methods(http.MethodDelete)
	return r
}

// start runs gua. The backing store is Postgres (via River).
func start(c *cli.Context) {
	startRiver(c)
}
