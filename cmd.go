package main

import (
	"context"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/syhlion/gua/delayquene"
	"github.com/syhlion/gua/httpv1"
	"github.com/urfave/cli"
)

// Healthz is a liveness probe: 200 as long as the process is serving. It does
// not touch the database — use it for the k8s livenessProbe.
func Healthz() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}
}

// Readyz is a readiness probe: 200 only when Postgres is reachable, 503
// otherwise. Use it for the k8s readinessProbe so traffic is held back while
// the DB is down.
func Readyz(quene delayquene.Quene) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := quene.Ping(ctx); err != nil {
			http.Error(w, "not ready: "+err.Error(), http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	}
}

// buildRouter wires the HTTP API over a Quene.
func buildRouter(quene delayquene.Quene) *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/version", Version(version)).Methods("GET")
	r.HandleFunc("/ui", UI()).Methods("GET")
	r.HandleFunc("/healthz", Healthz()).Methods("GET")
	r.HandleFunc("/readyz", Readyz(quene)).Methods("GET")
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
