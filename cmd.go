package main

import (
	"context"
	"net/http"
	"time"

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

// buildRouter wires the HTTP API over a Quene using the stdlib ServeMux
// (Go 1.22+ method + path-wildcard routing — no third-party router).
//
// Operational endpoints live at the root; the versioned REST API is resource
// oriented under /v1 (groups, and jobs as a sub-resource of a group).
func buildRouter(quene delayquene.Quene) *http.ServeMux {
	r := http.NewServeMux()

	// ops (unversioned)
	r.HandleFunc("GET /version", Version(version))
	r.HandleFunc("GET /ui", UI())
	r.HandleFunc("GET /healthz", Healthz())
	r.HandleFunc("GET /readyz", Readyz(quene))

	// system
	r.HandleFunc("GET /v1/status", httpv1.Status(quene))

	// groups
	r.HandleFunc("POST /v1/groups", httpv1.RegisterGroup(quene))
	r.HandleFunc("GET /v1/groups", httpv1.GetGroupList(quene))
	r.HandleFunc("GET /v1/groups/{group}", httpv1.GroupInfo(quene))
	r.HandleFunc("DELETE /v1/groups/{group}", httpv1.RemoveGroup(quene))

	// jobs (sub-resource of a group)
	r.HandleFunc("POST /v1/groups/{group}/jobs", httpv1.AddJob(quene))
	r.HandleFunc("GET /v1/groups/{group}/jobs", httpv1.GetJobList(quene))
	r.HandleFunc("DELETE /v1/groups/{group}/jobs", httpv1.DeleteJobs(quene)) // ?name= filters; else clears all
	r.HandleFunc("PATCH /v1/groups/{group}/jobs/{job}", httpv1.EditJob(quene))
	r.HandleFunc("DELETE /v1/groups/{group}/jobs/{job}", httpv1.DeleteJob(quene))
	r.HandleFunc("POST /v1/groups/{group}/jobs/{job}/pause", httpv1.PauseJob(quene))
	r.HandleFunc("POST /v1/groups/{group}/jobs/{job}/activate", httpv1.ActiveJob(quene))
	r.HandleFunc("GET /v1/groups/{group}/history", httpv1.History(quene))

	return r
}

// start runs gua. The backing store is Postgres (via River).
func start(c *cli.Context) {
	startRiver(c)
}
