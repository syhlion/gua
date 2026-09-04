// Package delayquene is gua's job scheduler. The backing store is Postgres
// (via River) — see riverquene.go. This file holds the backend-agnostic types:
// the Quene interface, the sentinel errors and input validation shared by the
// HTTP and gRPC API layers, the monitoring/history value types, the delivery
// envelope, and the request-url matcher.
package delayquene

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	guaproto "github.com/syhlion/gua/proto"
)

// Sentinel errors. Every error a Quene returns for a caller mistake wraps one
// of these, so the API layers can map them to an HTTP status / gRPC code with
// errors.Is. Anything else is an internal (storage) error.
var (
	ErrInvalid   = errors.New("invalid argument")
	ErrNotFound  = errors.New("not found")
	ErrDuplicate = errors.New("already exists")
)

// UrlRe matches a job's delivery target: HTTP@<url> or GRPC@<host:port>.
// The target is mandatory and may not contain whitespace.
var UrlRe = regexp.MustCompile(`^(HTTP|GRPC)@(\S+)$`)

// SplitRequestURL parses "HTTP@<url>" / "GRPC@<host:port>" into its two parts.
func SplitRequestURL(requestURL string) (kind, target string, err error) {
	ss := UrlRe.FindStringSubmatch(requestURL)
	if len(ss) != 3 {
		return "", "", fmt.Errorf("%w: request_url must be HTTP@<url> or GRPC@<host:port>", ErrInvalid)
	}
	return ss[1], ss[2], nil
}

// nameRe is the character set allowed in group names and job ids.
var nameRe = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

// MaxNameLen bounds group names and job ids.
const MaxNameLen = 22

// ValidateName checks a group name or job id: [a-zA-Z0-9_], 1..MaxNameLen.
func ValidateName(field, s string) error {
	if !nameRe.MatchString(s) {
		return fmt.Errorf("%w: %s must match [a-zA-Z0-9_]+", ErrInvalid, field)
	}
	if len(s) > MaxNameLen {
		return fmt.Errorf("%w: %s longer than %d characters", ErrInvalid, field, MaxNameLen)
	}
	return nil
}

// IsOnce reports whether an interval pattern means "fire once".
func IsOnce(pattern string) bool { return pattern == "" || pattern == "@once" }

// ValidatePattern checks an interval_pattern: "@once", a cron spec (5 or 6
// fields), or "@every <duration>".
func ValidatePattern(pattern string) error {
	if IsOnce(pattern) {
		return nil
	}
	if _, err := Parse(pattern); err != nil {
		return fmt.Errorf("%w: interval_pattern: %v", ErrInvalid, err)
	}
	return nil
}

// Quene is the scheduler surface used by the HTTP/gRPC API layer. Every
// mutating call validates its input and returns ErrInvalid / ErrNotFound /
// ErrDuplicate (wrapped) for caller mistakes.
type Quene interface {
	GenerateUID() string

	// Push stores a job definition and schedules its first occurrence. The
	// group must exist; the id must be unique. Exectime 0 means now.
	Push(job *guaproto.Job) error
	// Edit replaces a job's delivery target and payload. It takes effect on
	// the already-scheduled next occurrence.
	Edit(groupName, jobId, requestUrl, payload string) error
	// Active (re)activates a job and schedules it at exectime (0 = now). Any
	// occurrence already pending is replaced, so calling it twice fires once.
	Active(groupName, jobId string, exectime int64) error
	// Pause stops a job: its pending occurrence is dropped and the definition
	// is kept with active=false.
	Pause(groupName, jobId string) error
	// Delete removes one job and its pending occurrence.
	Delete(groupName, jobId string) error
	// DeleteJobs removes every job in a group, or only those whose name equals
	// name when it is non-empty. It returns how many definitions were removed.
	DeleteJobs(groupName, name string) (int, error)
	List(groupName string) ([]*guaproto.Job, error)

	RegisterGroup(groupName string) error
	// RemoveGroup deletes a group together with all its jobs and occurrences.
	RemoveGroup(groupName string) error
	ExistsGroup(groupName string) (bool, error)
	QueryGroups() ([]string, error)
	GroupInfo(groupName string) (string, error)

	Stats() (*Stats, error)
	History(group string, limit int) ([]*HistoryEntry, error)
	// Ping checks backing-store reachability for readiness probes.
	Ping(ctx context.Context) error
	Close()
}

// Stats is a read-only snapshot of queue health for monitoring.
type Stats struct {
	Now int64 `json:"now"`
	// ReadyQueueDepth is the number of occurrences waiting to run (scheduled,
	// available, retryable or pending) — roughly one per active job.
	ReadyQueueDepth int `json:"ready_queue_depth"`
	// Running is the number of deliveries in flight right now.
	Running int `json:"running"`
	// Retryable is the number of occurrences whose last delivery failed and
	// are waiting for their next attempt.
	Retryable  int `json:"retryable"`
	JobsActive int `json:"jobs_active"`
	JobsPaused int `json:"jobs_paused"`
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
