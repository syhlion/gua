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
	httpClient  *resty.Client
	machineHost string
	machineMac  string
	machineIp   string
	logger      *logrus.Logger
	insertNext  func(ctx context.Context, args guaJobArgs, at time.Time) error
}

func (w *guaDeliverWorker) Work(ctx context.Context, job *river.Job[guaJobArgs]) error {
	a := job.Args
	if err := deliver(ctx, w.httpClient, a); err != nil {
		w.logger.WithError(err).Errorf("river delivery error job %s", a.JobId)
		return err // River retries per MaxAttempts/backoff
	}
	// recurring: schedule the next occurrence
	if a.IntervalPattern != "" && a.IntervalPattern != "@once" {
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

	work := &guaDeliverWorker{
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
	_, err := q.client.Insert(context.Background(), guaJobArgs{
		JobId:           job.Id,
		JobName:         job.Name,
		GroupName:       job.GroupName,
		RequestUrl:      job.RequestUrl,
		Payload:         job.Payload,
		IntervalPattern: job.IntervalPattern,
		Timeout:         job.Timeout,
		PlanTime:        job.Exectime,
	}, &river.InsertOpts{ScheduledAt: time.Unix(job.Exectime, 0)})
	return err
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

// --- Phase 2: job-level operations mapped onto River (List/Delete/Pause/...) ---
var errRiverTODO = errors.New("not implemented in river backend yet (Phase 2)")

func (q *riverQuene) Remove(jobId string) error                       { return errRiverTODO }
func (q *riverQuene) Edit(group, jobId, requestUrl, payload string) error { return errRiverTODO }
func (q *riverQuene) Active(group, jobId string, exectime int64) error { return errRiverTODO }
func (q *riverQuene) Pause(group, jobId string) error                 { return errRiverTODO }
func (q *riverQuene) Delete(group, jobId string) error                { return errRiverTODO }
func (q *riverQuene) List(group string) ([]*guaproto.Job, error)      { return nil, errRiverTODO }
func (q *riverQuene) Stats() (*Stats, error)                          { return &Stats{}, nil }
func (q *riverQuene) History(group string, limit int) ([]*HistoryEntry, error) {
	return []*HistoryEntry{}, nil
}
