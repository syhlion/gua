// Package delayquene is gua's job scheduler. The backing store is Postgres
// (via River) — see riverquene.go. This file holds the backend-agnostic types:
// the Quene interface, the monitoring/history value types, the delivery
// envelope, and the request-url matcher.
package delayquene

import (
	"context"
	"regexp"

	guaproto "github.com/syhlion/gua/proto"
)

// UrlRe matches a job's delivery target: HTTP@<url> or GRPC@<host:port>.
var UrlRe = regexp.MustCompile(`^(HTTP|GRPC)\@(.+)?`)

// Quene is the scheduler surface used by the HTTP/gRPC API layer.
type Quene interface {
	GenerateUID() (s string)
	Remove(jobId string) (err error)
	Edit(groupName, jobId, requestUrl, payload string) (err error)
	Active(groupName string, jobId string, exectime int64) (err error)
	Pause(groupName string, jobId string) (err error)
	Delete(groupName string, jobId string) (err error)
	List(groupName string) (jobs []*guaproto.Job, err error)
	Push(job *guaproto.Job) (err error)
	RegisterGroup(groupName string) (err error)
	QueryGroups() (s []string, err error)
	GroupInfo(groupName string) (s string, err error)
	Close()
	RemoveGroup(groupName string) (err error)
	ExistsGroup(groupName string) (exists int, err error)
	Stats() (s *Stats, err error)
	History(group string, limit int) (entries []*HistoryEntry, err error)
	// Ping checks backing-store reachability for readiness probes.
	Ping(ctx context.Context) (err error)
}

// ServerStat is the health of a single node (kept for API compatibility; the
// Postgres backend has no slot election, so Stats.Servers is empty).
type ServerStat struct {
	Name          string `json:"name"`
	LastHeartbeat int64  `json:"last_heartbeat"`
	Alive         bool   `json:"alive"`
}

// Stats is a read-only snapshot of queue health for monitoring.
type Stats struct {
	Now               int64        `json:"now"`
	ReadyQueueDepth   int          `json:"ready_queue_depth"`
	DownServerBacklog int          `json:"down_server_backlog"`
	Servers           []ServerStat `json:"servers"`
}

// HistoryEntry is one execution record kept for monitoring.
type HistoryEntry struct {
	Seq             int64  `json:"seq"`
	JobId           string `json:"job_id"`
	GroupName       string `json:"group_name"`
	Type            string `json:"type"`
	PlanTime        int64  `json:"plan_time"`
	ExecTime        int64  `json:"exec_time"`
	FinishTime      int64  `json:"finish_time"`
	Success         bool   `json:"success"`
	Message         string `json:"message,omitempty"`
	Error           string `json:"error,omitempty"`
	ExecMachineHost string `json:"exec_machine_host"`
}

// TriggerEnvelope is the payload delivered to a consumer when a job fires
// (HTTP POST body / mapped onto guaproto.JobTrigger for gRPC).
type TriggerEnvelope struct {
	JobId     string `json:"job_id"`
	JobName   string `json:"job_name"`
	GroupName string `json:"group_name"`
	PlanTime  int64  `json:"plan_time"`
	ExecTime  int64  `json:"exec_time"`
	Payload   string `json:"payload"`
	// IdempotencyKey is stable across retries/redeliveries of the same firing;
	// dedupe on it (delivery is at-least-once). ExecTime is NOT stable.
	IdempotencyKey string `json:"idempotency_key"`
}
