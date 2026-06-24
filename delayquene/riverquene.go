package delayquene

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
	"github.com/sirupsen/logrus"
	"github.com/syhlion/gua/internal/httpclient"
	guaproto "github.com/syhlion/gua/proto"
	"google.golang.org/grpc"
	"resty.dev/v3"
)

// RiverConfig configures the Postgres/River-backed queue.
type RiverConfig struct {
	DSN         string // postgres connection string
	MaxWorkers  int    // per-queue worker concurrency (default 50)
	MachineHost string
	MachineMac  string
	MachineIp   string
	Logger      *logrus.Logger
}

// guaJobArgs is the River job payload for one gua delivery.
type guaJobArgs struct {
	JobId           string `json:"job_id"`
	JobName         string `json:"job_name"`
	GroupName       string `json:"group_name"`
	RequestUrl      string `json:"request_url"`
	Payload         string `json:"payload"`
	IntervalPattern string `json:"interval_pattern"`
	Timeout         int64  `json:"timeout"`
	PlanTime        int64  `json:"plan_time"`
}

func (guaJobArgs) Kind() string { return "gua_delivery" }

// guaDeliverWorker runs the actual delivery (HTTP POST envelope / gRPC
// OnJobTrigger). On success it re-schedules recurring jobs via insertNext.
type guaDeliverWorker struct {
	river.WorkerDefaults[guaJobArgs]
	pool        *pgxpool.Pool
	httpClient  *resty.Client
	machineHost string
	machineMac  string
	machineIp   string
	logger      *logrus.Logger
	insertNext  func(ctx context.Context, args guaJobArgs, at time.Time) error
}

// jobActive reports whether the job's definition still exists and is active.
// A missing row (deleted) or active=false (paused) means: do not deliver.
func (w *guaDeliverWorker) jobActive(ctx context.Context, jobId string) bool {
	var active bool
	err := w.pool.QueryRow(ctx, `SELECT active FROM gua_jobs WHERE id=$1`, jobId).Scan(&active)
	return err == nil && active
}

func (w *guaDeliverWorker) Work(ctx context.Context, job *river.Job[guaJobArgs]) error {
	a := job.Args
	// honour pause/delete that happened after this occurrence was scheduled
	if !w.jobActive(ctx, a.JobId) {
		return nil
	}
	if err := deliver(ctx, w.httpClient, a); err != nil {
		w.logger.WithError(err).Errorf("river delivery error job %s", a.JobId)
		return err // River retries per MaxAttempts/backoff
	}
	// fire-once: drop the definition after delivery (matches the Redis backend)
	if a.IntervalPattern == "" || a.IntervalPattern == "@once" {
		w.pool.Exec(ctx, `DELETE FROM gua_jobs WHERE id=$1`, a.JobId)
		return nil
	}
	// recurring: schedule the next occurrence (unless paused/deleted meanwhile)
	if w.jobActive(ctx, a.JobId) {
		sch, err := Parse(a.IntervalPattern)
		if err != nil {
			w.logger.WithError(err).Errorf("river cron parse error job %s", a.JobId)
			return nil // delivered ok; just stop recurring on a bad pattern
		}
		next := sch.Next(time.Now())
		na := a
		na.PlanTime = next.Unix()
		if err := w.insertNext(ctx, na, next); err != nil {
			w.logger.WithError(err).Errorf("river reschedule error job %s", a.JobId)
		}
	}
	return nil
}

// deliver sends the trigger envelope to the consumer. Self-contained (kept
// separate from the Redis worker so that path is untouched).
func deliver(ctx context.Context, httpClient *resty.Client, a guaJobArgs) error {
	ss := UrlRe.FindStringSubmatch(a.RequestUrl)
	if len(ss) == 0 {
		return fmt.Errorf("invalid request_url %q", a.RequestUrl)
	}
	env := TriggerEnvelope{
		JobId:     a.JobId,
		JobName:   a.JobName,
		GroupName: a.GroupName,
		PlanTime:  a.PlanTime,
		ExecTime:  time.Now().Unix(),
		Payload:   a.Payload,
	}
	switch ss[1] {
	case "HTTP":
		body, _ := json.Marshal(env)
		_, status, err := httpclient.PostRaw(httpClient, ss[2], body)
		if err != nil {
			return err
		}
		if status >= 400 {
			return fmt.Errorf("http callback status %d", status)
		}
		return nil
	case "GRPC":
		timeout := time.Duration(a.Timeout) * time.Second
		if timeout <= 0 {
			timeout = 5 * time.Second
		}
		conn, err := grpc.Dial(ss[2], grpc.WithInsecure())
		if err != nil {
			return err
		}
		defer conn.Close()
		cctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		res, err := guaproto.NewGuaCallbackClient(conn).OnJobTrigger(cctx, &guaproto.JobTrigger{
			JobId: env.JobId, JobName: env.JobName, GroupName: env.GroupName,
			PlanTime: env.PlanTime, ExecTime: env.ExecTime, Payload: env.Payload,
		})
		if err != nil {
			return err
		}
		if res != nil && !res.Success {
			return fmt.Errorf("grpc callback reported failure: %s", res.Message)
		}
		return nil
	default:
		return fmt.Errorf("unsupported request type %q", ss[1])
	}
}

type riverQuene struct {
	pool   *pgxpool.Pool
	client *river.Client[pgx.Tx]
	logger *logrus.Logger
}

// NewRiver builds a Postgres/River-backed Quene: runs River migrations, creates
// the gua_groups table, registers the delivery worker, and starts the client.
func NewRiver(cfg *RiverConfig) (Quene, error) {
	if cfg.MaxWorkers <= 0 {
		cfg.MaxWorkers = 50
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DSN)
	if err != nil {
		return nil, err
	}
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		pool.Close()
		return nil, err
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		pool.Close()
		return nil, err
	}
	if _, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS gua_groups (
		group_name text PRIMARY KEY,
		created_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		pool.Close()
		return nil, err
	}
	// gua_jobs is the source of truth for a job's definition (active or paused),
	// independent of whether an occurrence is currently scheduled in River.
	if _, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS gua_jobs (
		id               text PRIMARY KEY,
		group_name       text NOT NULL,
		name             text NOT NULL,
		request_url      text NOT NULL,
		payload          text NOT NULL DEFAULT '',
		interval_pattern text NOT NULL DEFAULT '@once',
		timeout          bigint NOT NULL DEFAULT 0,
		exectime         bigint NOT NULL,
		active           boolean NOT NULL DEFAULT true,
		memo             text NOT NULL DEFAULT '',
		created_at       timestamptz NOT NULL DEFAULT now());
		CREATE INDEX IF NOT EXISTS gua_jobs_group_idx ON gua_jobs (group_name);`); err != nil {
		pool.Close()
		return nil, err
	}

	work := &guaDeliverWorker{
		pool:        pool,
		httpClient:  httpclient.New(60*time.Second, false),
		machineHost: cfg.MachineHost,
		machineMac:  cfg.MachineMac,
		machineIp:   cfg.MachineIp,
		logger:      cfg.Logger,
	}
	workers := river.NewWorkers()
	river.AddWorker(workers, work)

	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues:  map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: cfg.MaxWorkers}},
		Workers: workers,
	})
	if err != nil {
		pool.Close()
		return nil, err
	}
	// close the loop: the worker reschedules recurring jobs through the client
	work.insertNext = func(ctx context.Context, args guaJobArgs, at time.Time) error {
		_, ierr := client.Insert(ctx, args, &river.InsertOpts{ScheduledAt: at})
		return ierr
	}
	if err := client.Start(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &riverQuene{pool: pool, client: client, logger: cfg.Logger}, nil
}

func genID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (q *riverQuene) GenerateUID() string { return genID() }

func (q *riverQuene) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = q.client.Stop(ctx)
	q.pool.Close()
}

func argsOf(job *guaproto.Job) guaJobArgs {
	return guaJobArgs{
		JobId:           job.Id,
		JobName:         job.Name,
		GroupName:       job.GroupName,
		RequestUrl:      job.RequestUrl,
		Payload:         job.Payload,
		IntervalPattern: job.IntervalPattern,
		Timeout:         job.Timeout,
		PlanTime:        job.Exectime,
	}
}

// insertOccurrence schedules one River occurrence for a job at `at`.
func (q *riverQuene) insertOccurrence(ctx context.Context, job *guaproto.Job, at time.Time) error {
	a := argsOf(job)
	a.PlanTime = at.Unix()
	_, err := q.client.Insert(ctx, a, &river.InsertOpts{ScheduledAt: at})
	return err
}

// cancelOccurrences removes any not-yet-run River occurrence of a job (used by
// Pause/Delete). Running/finished occurrences are left alone.
func (q *riverQuene) cancelOccurrences(ctx context.Context, jobId string) error {
	_, err := q.pool.Exec(ctx, `DELETE FROM river_job
		WHERE kind='gua_delivery' AND args->>'job_id'=$1
		AND state IN ('available','scheduled','retryable','pending')`, jobId)
	return err
}

func (q *riverQuene) loadJob(ctx context.Context, jobId string) (*guaproto.Job, error) {
	j := &guaproto.Job{}
	err := q.pool.QueryRow(ctx, `SELECT id, group_name, name, request_url, payload,
		interval_pattern, timeout, exectime, active, memo FROM gua_jobs WHERE id=$1`, jobId).
		Scan(&j.Id, &j.GroupName, &j.Name, &j.RequestUrl, &j.Payload,
			&j.IntervalPattern, &j.Timeout, &j.Exectime, &j.Active, &j.Memo)
	if err != nil {
		return nil, err
	}
	return j, nil
}

func (q *riverQuene) Push(job *guaproto.Job) error {
	ss := UrlRe.FindStringSubmatch(job.RequestUrl)
	if len(ss) == 0 {
		return errors.New("type error")
	}
	switch ss[1] {
	case "HTTP", "GRPC":
	default:
		return errors.New("type error")
	}
	ctx := context.Background()
	// store the definition (source of truth); reject duplicate ids like the Redis backend
	ct, err := q.pool.Exec(ctx, `INSERT INTO gua_jobs
		(id, group_name, name, request_url, payload, interval_pattern, timeout, exectime, active, memo)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,true,$9) ON CONFLICT (id) DO NOTHING`,
		job.Id, job.GroupName, job.Name, job.RequestUrl, job.Payload,
		job.IntervalPattern, job.Timeout, job.Exectime, job.Memo)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return errors.New("key duplicate")
	}
	return q.insertOccurrence(ctx, job, time.Unix(job.Exectime, 0))
}

func (q *riverQuene) RegisterGroup(groupName string) error {
	ct, err := q.pool.Exec(context.Background(),
		`INSERT INTO gua_groups (group_name) VALUES ($1) ON CONFLICT DO NOTHING`, groupName)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return errors.New("duplicate key")
	}
	return nil
}

func (q *riverQuene) RemoveGroup(groupName string) error {
	_, err := q.pool.Exec(context.Background(), `DELETE FROM gua_groups WHERE group_name=$1`, groupName)
	return err
}

func (q *riverQuene) ExistsGroup(groupName string) (int, error) {
	var n int
	err := q.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM gua_groups WHERE group_name=$1`, groupName).Scan(&n)
	return n, err
}

func (q *riverQuene) GroupInfo(groupName string) (string, error) {
	var g string
	err := q.pool.QueryRow(context.Background(),
		`SELECT group_name FROM gua_groups WHERE group_name=$1`, groupName).Scan(&g)
	return g, err
}

func (q *riverQuene) QueryGroups() ([]string, error) {
	rows, err := q.pool.Query(context.Background(), `SELECT group_name FROM gua_groups ORDER BY group_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var g string
		if err := rows.Scan(&g); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// --- job-level operations: gua_jobs is the source of truth; River holds occurrences ---

func (q *riverQuene) List(group string) ([]*guaproto.Job, error) {
	rows, err := q.pool.Query(context.Background(), `SELECT id, group_name, name, request_url,
		payload, interval_pattern, timeout, exectime, active, memo FROM gua_jobs
		WHERE group_name=$1 ORDER BY exectime`, group)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	jobs := make([]*guaproto.Job, 0)
	for rows.Next() {
		j := &guaproto.Job{}
		if err := rows.Scan(&j.Id, &j.GroupName, &j.Name, &j.RequestUrl, &j.Payload,
			&j.IntervalPattern, &j.Timeout, &j.Exectime, &j.Active, &j.Memo); err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

func (q *riverQuene) Delete(group, jobId string) error {
	ctx := context.Background()
	if _, err := q.pool.Exec(ctx, `DELETE FROM gua_jobs WHERE id=$1 AND group_name=$2`, jobId, group); err != nil {
		return err
	}
	return q.cancelOccurrences(ctx, jobId)
}

func (q *riverQuene) Remove(jobId string) error {
	ctx := context.Background()
	if _, err := q.pool.Exec(ctx, `DELETE FROM gua_jobs WHERE id=$1`, jobId); err != nil {
		return err
	}
	return q.cancelOccurrences(ctx, jobId)
}

func (q *riverQuene) Edit(group, jobId, requestUrl, payload string) error {
	if ss := UrlRe.FindStringSubmatch(requestUrl); len(ss) == 0 {
		return errors.New("type error")
	}
	ct, err := q.pool.Exec(context.Background(),
		`UPDATE gua_jobs SET request_url=$1, payload=$2 WHERE id=$3 AND group_name=$4`,
		requestUrl, payload, jobId, group)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return errors.New("no job")
	}
	return nil
}

func (q *riverQuene) Pause(group, jobId string) error {
	ctx := context.Background()
	if _, err := q.pool.Exec(ctx,
		`UPDATE gua_jobs SET active=false WHERE id=$1 AND group_name=$2`, jobId, group); err != nil {
		return err
	}
	// drop the pending occurrence so it won't fire (the worker also re-checks active)
	return q.cancelOccurrences(ctx, jobId)
}

func (q *riverQuene) Active(group, jobId string, exectime int64) error {
	ctx := context.Background()
	if _, err := q.pool.Exec(ctx,
		`UPDATE gua_jobs SET active=true, exectime=$1 WHERE id=$2 AND group_name=$3`,
		exectime, jobId, group); err != nil {
		return err
	}
	job, err := q.loadJob(ctx, jobId)
	if err != nil {
		return err
	}
	return q.insertOccurrence(ctx, job, time.Unix(exectime, 0))
}

func (q *riverQuene) Stats() (*Stats, error) {
	s := &Stats{Now: time.Now().Unix(), Servers: []ServerStat{}}
	err := q.pool.QueryRow(context.Background(), `SELECT count(*) FROM river_job
		WHERE kind='gua_delivery' AND state IN ('available','scheduled','retryable','pending')`).
		Scan(&s.ReadyQueueDepth)
	return s, err
}

// History — Phase 4 (PG executions table). Empty for now.
func (q *riverQuene) History(group string, limit int) ([]*HistoryEntry, error) {
	return []*HistoryEntry{}, nil
}
