package delayquene

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
	"github.com/syhlion/gua/internal/httpclient"
	guaproto "github.com/syhlion/gua/proto"
	"resty.dev/v3"
)

// RiverConfig configures the Postgres/River-backed queue.
type RiverConfig struct {
	DSN string // postgres connection string
	// MaxWorkers is the per-node delivery concurrency (default 50).
	MaxWorkers int
	// MaxAttempts is how many times one firing is delivered before it is given
	// up and the job is paused (default 25, River's exponential backoff).
	MaxAttempts int
	// DeliveryTimeout applies to a delivery whose job has timeout 0
	// (default 30s). A job's own timeout is capped at MaxDeliveryTimeout.
	DeliveryTimeout time.Duration
	MachineHost     string
	// HistoryTTL is the execution-history retention in seconds; 0 disables
	// history recording.
	HistoryTTL int
	// PruneInterval is how often history older than HistoryTTL is deleted
	// (default 10m). The prune runs as a River periodic job, once per cluster.
	PruneInterval time.Duration
	Logger        *slog.Logger
}

const (
	defaultMaxWorkers      = 50
	defaultMaxAttempts     = 25
	defaultDeliveryTimeout = 30 * time.Second
	defaultPruneInterval   = 10 * time.Minute

	// MaxDeliveryTimeout caps a job's timeout. River's per-job timeout is set
	// just above it so a runaway delivery can never outlive the rescue window.
	MaxDeliveryTimeout = 10 * time.Minute
	riverJobTimeout    = MaxDeliveryTimeout + 30*time.Second
	// rescueAfter is how long an occurrence may sit in `running` (a crashed
	// node) before River's rescuer re-runs it. Must be >= riverJobTimeout.
	rescueAfter = 15 * time.Minute
	// drainTimeout is how long Close waits for in-flight deliveries before
	// cancelling them.
	drainTimeout = 10 * time.Second

	pendingStates = `('available','scheduled','retryable','pending')`
)

// guaJobArgs is the River job payload for one gua occurrence. It carries only
// the identity of the firing: the job's definition (target, payload, timeout,
// interval) is read from gua_jobs when the occurrence runs, so Edit / Pause /
// Delete take effect on the already-scheduled occurrence.
type guaJobArgs struct {
	JobId     string `json:"job_id"`
	GroupName string `json:"group_name"`
	PlanTime  int64  `json:"plan_time"`
}

func (guaJobArgs) Kind() string { return "gua_delivery" }

// pruneArgs is the periodic history-prune job. River's leader inserts it on
// the PruneInterval; any node runs it. Unique so runs never pile up.
type pruneArgs struct{}

func (pruneArgs) Kind() string { return "gua_history_prune" }
func (pruneArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

type riverQuene struct {
	cfg        RiverConfig
	pool       *pgxpool.Pool
	client     *river.Client[pgx.Tx]
	httpClient *resty.Client
	grpcPool   *grpcClientPool
	logger     *slog.Logger
}

// NewRiver builds a Postgres/River-backed Quene: runs River migrations, creates
// the gua_* tables, registers the workers, and starts the client.
func NewRiver(cfg *RiverConfig) (Quene, error) {
	c := *cfg
	if c.MaxWorkers <= 0 {
		c.MaxWorkers = defaultMaxWorkers
	}
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = defaultMaxAttempts
	}
	if c.DeliveryTimeout <= 0 {
		c.DeliveryTimeout = defaultDeliveryTimeout
	}
	if c.DeliveryTimeout > MaxDeliveryTimeout {
		c.DeliveryTimeout = MaxDeliveryTimeout
	}
	if c.PruneInterval <= 0 {
		c.PruneInterval = defaultPruneInterval
	}
	if c.Logger == nil {
		c.Logger = slog.Default()
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, c.DSN)
	if err != nil {
		return nil, err
	}
	if err := migrate(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}

	q := &riverQuene{
		cfg:        c,
		pool:       pool,
		httpClient: httpclient.New(MaxDeliveryTimeout, false),
		grpcPool:   newGrpcClientPool(),
		logger:     c.Logger,
	}
	workers := river.NewWorkers()
	river.AddWorker(workers, &guaDeliverWorker{q: q})
	river.AddWorker(workers, &pruneWorker{q: q})

	var periodic []*river.PeriodicJob
	if c.HistoryTTL > 0 {
		periodic = append(periodic, river.NewPeriodicJob(
			river.PeriodicInterval(c.PruneInterval),
			func() (river.JobArgs, *river.InsertOpts) { return pruneArgs{}, nil },
			&river.PeriodicJobOpts{RunOnStart: true},
		))
	}
	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues:               map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: c.MaxWorkers}},
		Workers:              workers,
		MaxAttempts:          c.MaxAttempts,
		JobTimeout:           riverJobTimeout,
		RescueStuckJobsAfter: rescueAfter,
		// Stop() waits this long for in-flight deliveries, then cancels their
		// contexts so shutdown never hangs on a slow consumer.
		SoftStopTimeout: drainTimeout,
		PeriodicJobs:    periodic,
		Logger:          c.Logger,
	})
	if err != nil {
		pool.Close()
		return nil, err
	}
	q.client = client
	if err := client.Start(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return q, nil
}

// migrate runs River's migrations and creates gua's own tables.
func migrate(ctx context.Context, pool *pgxpool.Pool) error {
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return err
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return err
	}
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS gua_groups (
			group_name text PRIMARY KEY,
			created_at timestamptz NOT NULL DEFAULT now())`,
		// gua_jobs is the source of truth for a job's definition (active or
		// paused), independent of whether an occurrence is scheduled in River.
		`CREATE TABLE IF NOT EXISTS gua_jobs (
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
			created_at       timestamptz NOT NULL DEFAULT now())`,
		`CREATE INDEX IF NOT EXISTS gua_jobs_group_idx ON gua_jobs (group_name)`,
		`CREATE TABLE IF NOT EXISTS gua_executions (
			seq               bigserial PRIMARY KEY,
			job_id            text NOT NULL,
			group_name        text NOT NULL,
			type              text NOT NULL,
			plan_time         bigint NOT NULL,
			exec_time         bigint NOT NULL,
			finish_time       bigint NOT NULL,
			success           boolean NOT NULL,
			message           text NOT NULL DEFAULT '',
			error             text NOT NULL DEFAULT '',
			exec_machine_host text NOT NULL DEFAULT '',
			created_at        timestamptz NOT NULL DEFAULT now())`,
		`CREATE INDEX IF NOT EXISTS gua_exec_group_idx ON gua_executions (group_name, seq DESC)`,
		`CREATE INDEX IF NOT EXISTS gua_exec_created_idx ON gua_executions (created_at)`,
	}
	for _, s := range stmts {
		if _, err := pool.Exec(ctx, s); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// workers
// ---------------------------------------------------------------------------

// guaDeliverWorker runs one occurrence: load the definition, deliver the
// envelope (HTTP POST / gRPC OnJobTrigger), record the attempt, then either
// drop a fire-once definition or schedule the next occurrence.
type guaDeliverWorker struct {
	river.WorkerDefaults[guaJobArgs]
	q *riverQuene
}

func (w *guaDeliverWorker) Work(ctx context.Context, job *river.Job[guaJobArgs]) error {
	q := w.q
	a := job.Args
	def, err := q.loadJob(ctx, a.JobId)
	if errors.Is(err, ErrNotFound) {
		return nil // deleted after this occurrence was scheduled
	}
	if err != nil {
		return err // storage hiccup: let River retry
	}
	if !def.Active {
		return nil // paused after this occurrence was scheduled
	}

	execTime := time.Now().Unix()
	// River's occurrence id is stable across retries/rescues of THIS firing —
	// hand it to the consumer as the idempotency key.
	idemKey := strconv.FormatInt(job.ID, 10)
	resp, derr := q.deliver(ctx, def, a.PlanTime, idemKey)
	q.recordExecution(ctx, def, a.PlanTime, execTime, resp, derr)
	if derr != nil {
		if job.Attempt >= job.MaxAttempts {
			// River is about to discard this occurrence. Pause the definition
			// so the exhausted job is visible (active=false) rather than
			// silently dead; Active re-arms it.
			if _, perr := q.pool.Exec(ctx, `UPDATE gua_jobs SET active=false WHERE id=$1`, def.Id); perr != nil {
				q.logger.Error("pause exhausted job", "job", def.Id, "error", perr)
			}
			q.logger.Error("delivery attempts exhausted, job paused",
				"job", def.Id, "group", def.GroupName, "attempts", job.Attempt, "error", derr)
		} else {
			q.logger.Warn("delivery failed, will retry",
				"job", def.Id, "group", def.GroupName, "attempt", job.Attempt, "error", derr)
		}
		return derr // River retries per MaxAttempts/backoff
	}

	if IsOnce(def.IntervalPattern) {
		// Delivered: the definition is done. Never return an error here — a
		// retry would re-deliver a job that already succeeded.
		if _, err := q.pool.Exec(ctx, `DELETE FROM gua_jobs WHERE id=$1`, def.Id); err != nil {
			q.logger.Error("drop fire-once job", "job", def.Id, "error", err)
		}
		return nil
	}
	sch, err := Parse(def.IntervalPattern)
	if err != nil {
		// Only reachable for rows written before Push validated patterns.
		q.logger.Error("bad interval_pattern, job paused", "job", def.Id, "pattern", def.IntervalPattern, "error", err)
		_, _ = q.pool.Exec(ctx, `UPDATE gua_jobs SET active=false WHERE id=$1`, def.Id)
		return nil
	}
	next := sch.Next(time.Now())
	if err := q.scheduleNext(ctx, def.Id, def.GroupName, next); err != nil {
		// Return the error so River retries the whole Work() (re-deliver,
		// caught by the idempotency key) — the recurring chain must not break
		// on a transient insert failure.
		q.logger.Error("reschedule failed", "job", def.Id, "error", err)
		return err
	}
	return nil
}

// scheduleNext records the next fire time on the definition and inserts the
// matching occurrence, atomically. If the job was paused or deleted in the
// meantime nothing is inserted.
func (q *riverQuene) scheduleNext(ctx context.Context, jobId, group string, next time.Time) error {
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit
	ct, err := tx.Exec(ctx, `UPDATE gua_jobs SET exectime=$1 WHERE id=$2 AND active=true`, next.Unix(), jobId)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return nil
	}
	if err := q.insertOccurrenceTx(ctx, tx, jobId, group, next); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// deliver sends the trigger envelope to the consumer and returns the result
// message. idemKey is stable across re-deliveries of this firing.
func (q *riverQuene) deliver(ctx context.Context, def *guaproto.Job, planTime int64, idemKey string) (string, error) {
	kind, target, err := SplitRequestURL(def.RequestUrl)
	if err != nil {
		return "", err
	}
	timeout := time.Duration(def.Timeout) * time.Second
	if timeout <= 0 {
		timeout = q.cfg.DeliveryTimeout
	}
	if timeout > MaxDeliveryTimeout {
		timeout = MaxDeliveryTimeout
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	env := TriggerEnvelope{
		JobId:          def.Id,
		JobName:        def.Name,
		GroupName:      def.GroupName,
		PlanTime:       planTime,
		ExecTime:       time.Now().Unix(),
		Payload:        def.Payload,
		IdempotencyKey: idemKey,
	}
	switch kind {
	case "HTTP":
		body, err := json.Marshal(env)
		if err != nil {
			return "", err
		}
		respBody, status, err := httpclient.PostRaw(cctx, q.httpClient, target, body)
		if err != nil {
			return "", err
		}
		if status >= 400 {
			return "", fmt.Errorf("http callback status %d", status)
		}
		return string(respBody), nil
	case "GRPC":
		conn, err := q.grpcPool.conn(target)
		if err != nil {
			return "", err
		}
		// conn is pooled and reused across triggers — do NOT close it here.
		res, err := guaproto.NewGuaCallbackClient(conn).OnJobTrigger(cctx, &guaproto.JobTrigger{
			JobId: env.JobId, JobName: env.JobName, GroupName: env.GroupName,
			PlanTime: env.PlanTime, ExecTime: env.ExecTime, Payload: env.Payload,
			IdempotencyKey: env.IdempotencyKey,
		})
		if err != nil {
			return "", err
		}
		if res == nil {
			return "", nil
		}
		if !res.Success {
			return "", fmt.Errorf("grpc callback reported failure: %s", res.Message)
		}
		return res.Message, nil
	default:
		return "", fmt.Errorf("unsupported request type %q", kind)
	}
}

// recordExecution appends one execution record (Monitor Tier 2). No-op when
// history is disabled. Retention is enforced by the periodic prune job.
func (q *riverQuene) recordExecution(ctx context.Context, def *guaproto.Job, planTime, execTime int64, resp string, derr error) {
	if q.cfg.HistoryTTL <= 0 {
		return
	}
	kind, _, _ := SplitRequestURL(def.RequestUrl)
	var errStr, msg string
	if derr != nil {
		errStr = derr.Error()
	} else {
		msg = resp
	}
	_, err := q.pool.Exec(ctx, `INSERT INTO gua_executions
		(job_id, group_name, type, plan_time, exec_time, finish_time, success, message, error, exec_machine_host)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		def.Id, def.GroupName, kind, planTime, execTime, time.Now().Unix(), derr == nil, msg, errStr, q.cfg.MachineHost)
	if err != nil {
		q.logger.Error("record execution", "job", def.Id, "error", err)
	}
}

// pruneWorker deletes execution history older than HistoryTTL.
type pruneWorker struct {
	river.WorkerDefaults[pruneArgs]
	q *riverQuene
}

func (w *pruneWorker) Work(ctx context.Context, _ *river.Job[pruneArgs]) error {
	if w.q.cfg.HistoryTTL <= 0 {
		return nil
	}
	ct, err := w.q.pool.Exec(ctx,
		`DELETE FROM gua_executions WHERE created_at < now() - make_interval(secs => $1)`, w.q.cfg.HistoryTTL)
	if err != nil {
		return err
	}
	w.q.logger.Debug("history pruned", "rows", ct.RowsAffected())
	return nil
}

// ---------------------------------------------------------------------------
// Quene implementation
// ---------------------------------------------------------------------------

func genID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (q *riverQuene) GenerateUID() string { return genID() }

// Close drains in-flight deliveries (up to drainTimeout, then cancels them),
// then closes the delivery connections and the pool.
func (q *riverQuene) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), drainTimeout+5*time.Second)
	defer cancel()
	if err := q.client.Stop(ctx); err != nil {
		q.logger.Warn("river stop", "error", err)
	}
	q.grpcPool.Close()
	q.pool.Close()
}

// Ping verifies the Postgres pool can reach the database (readiness probe).
func (q *riverQuene) Ping(ctx context.Context) error {
	return q.pool.Ping(ctx)
}

// insertOccurrenceTx schedules one River occurrence for a job at `at`.
func (q *riverQuene) insertOccurrenceTx(ctx context.Context, tx pgx.Tx, jobId, group string, at time.Time) error {
	_, err := q.client.InsertTx(ctx, tx,
		guaJobArgs{JobId: jobId, GroupName: group, PlanTime: at.Unix()},
		&river.InsertOpts{ScheduledAt: at})
	return err
}

// cancelOccurrencesTx removes any not-yet-run River occurrence of a job.
// Running/finished occurrences are left alone (the worker re-checks the
// definition before delivering). The containment match uses River's GIN index
// on args.
func (q *riverQuene) cancelOccurrencesTx(ctx context.Context, tx pgx.Tx, jobId string) error {
	_, err := tx.Exec(ctx, `DELETE FROM river_job
		WHERE kind='gua_delivery' AND state IN `+pendingStates+`
		AND args @> jsonb_build_object('job_id', $1::text)`, jobId)
	return err
}

const jobColumns = `id, group_name, name, request_url, payload, interval_pattern, timeout, exectime, active, memo`

func scanJob(row pgx.Row) (*guaproto.Job, error) {
	j := &guaproto.Job{}
	err := row.Scan(&j.Id, &j.GroupName, &j.Name, &j.RequestUrl, &j.Payload,
		&j.IntervalPattern, &j.Timeout, &j.Exectime, &j.Active, &j.Memo)
	if err != nil {
		return nil, err
	}
	return j, nil
}

func (q *riverQuene) loadJob(ctx context.Context, jobId string) (*guaproto.Job, error) {
	j, err := scanJob(q.pool.QueryRow(ctx, `SELECT `+jobColumns+` FROM gua_jobs WHERE id=$1`, jobId))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("job %w", ErrNotFound)
	}
	return j, err
}

func (q *riverQuene) Push(job *guaproto.Job) error {
	if err := ValidateName("group_name", job.GroupName); err != nil {
		return err
	}
	if err := ValidateName("job_id", job.Id); err != nil {
		return err
	}
	if job.Name == "" {
		return fmt.Errorf("%w: name is required", ErrInvalid)
	}
	if _, _, err := SplitRequestURL(job.RequestUrl); err != nil {
		return err
	}
	if job.IntervalPattern == "" {
		job.IntervalPattern = "@once"
	}
	if err := ValidatePattern(job.IntervalPattern); err != nil {
		return err
	}
	if job.Exectime < 0 {
		return fmt.Errorf("%w: exec_time must be >= 0", ErrInvalid)
	}
	if job.Exectime == 0 {
		job.Exectime = time.Now().Unix()
	}
	if job.Timeout < 0 {
		return fmt.Errorf("%w: timeout must be >= 0", ErrInvalid)
	}

	ctx := context.Background()
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit
	var one int
	if err := tx.QueryRow(ctx, `SELECT 1 FROM gua_groups WHERE group_name=$1`, job.GroupName).Scan(&one); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("group %w", ErrNotFound)
		}
		return err
	}
	// store the definition (source of truth); reject duplicate ids
	ct, err := tx.Exec(ctx, `INSERT INTO gua_jobs
		(id, group_name, name, request_url, payload, interval_pattern, timeout, exectime, active, memo)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,true,$9) ON CONFLICT (id) DO NOTHING`,
		job.Id, job.GroupName, job.Name, job.RequestUrl, job.Payload,
		job.IntervalPattern, job.Timeout, job.Exectime, job.Memo)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("job id %w", ErrDuplicate)
	}
	if err := q.insertOccurrenceTx(ctx, tx, job.Id, job.GroupName, time.Unix(job.Exectime, 0)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (q *riverQuene) Edit(group, jobId, requestUrl, payload string) error {
	if _, _, err := SplitRequestURL(requestUrl); err != nil {
		return err
	}
	ct, err := q.pool.Exec(context.Background(),
		`UPDATE gua_jobs SET request_url=$1, payload=$2 WHERE id=$3 AND group_name=$4`,
		requestUrl, payload, jobId, group)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("job %w", ErrNotFound)
	}
	return nil
}

func (q *riverQuene) Active(group, jobId string, exectime int64) error {
	if exectime < 0 {
		return fmt.Errorf("%w: exec_time must be >= 0", ErrInvalid)
	}
	if exectime == 0 {
		exectime = time.Now().Unix()
	}
	at := time.Unix(exectime, 0)
	ctx := context.Background()
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit
	ct, err := tx.Exec(ctx,
		`UPDATE gua_jobs SET active=true, exectime=$1 WHERE id=$2 AND group_name=$3`, exectime, jobId, group)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("job %w", ErrNotFound)
	}
	// replace, never add: a pending occurrence from Push or an earlier Active
	// would otherwise make the job fire twice.
	if err := q.cancelOccurrencesTx(ctx, tx, jobId); err != nil {
		return err
	}
	if err := q.insertOccurrenceTx(ctx, tx, jobId, group, at); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (q *riverQuene) Pause(group, jobId string) error {
	ctx := context.Background()
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit
	ct, err := tx.Exec(ctx, `UPDATE gua_jobs SET active=false WHERE id=$1 AND group_name=$2`, jobId, group)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("job %w", ErrNotFound)
	}
	// drop the pending occurrence so it won't fire (the worker also re-checks active)
	if err := q.cancelOccurrencesTx(ctx, tx, jobId); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (q *riverQuene) Delete(group, jobId string) error {
	ctx := context.Background()
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit
	ct, err := tx.Exec(ctx, `DELETE FROM gua_jobs WHERE id=$1 AND group_name=$2`, jobId, group)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("job %w", ErrNotFound)
	}
	if err := q.cancelOccurrencesTx(ctx, tx, jobId); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// DeleteJobs removes every job in the group (name == "") or only the jobs
// named `name`, plus their pending occurrences, in one statement.
func (q *riverQuene) DeleteJobs(group, name string) (int, error) {
	var n int
	err := q.pool.QueryRow(context.Background(), `
		WITH del AS (
			DELETE FROM gua_jobs WHERE group_name=$1 AND ($2 = '' OR name=$2) RETURNING id
		), occ AS (
			DELETE FROM river_job
			WHERE kind='gua_delivery' AND state IN `+pendingStates+`
			AND args @> jsonb_build_object('group_name', $1::text)
			AND args->>'job_id' IN (SELECT id FROM del)
		)
		SELECT count(*) FROM del`, group, name).Scan(&n)
	return n, err
}

func (q *riverQuene) List(group string) ([]*guaproto.Job, error) {
	rows, err := q.pool.Query(context.Background(),
		`SELECT `+jobColumns+` FROM gua_jobs WHERE group_name=$1 ORDER BY exectime`, group)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	jobs := make([]*guaproto.Job, 0)
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

func (q *riverQuene) RegisterGroup(groupName string) error {
	if err := ValidateName("group_name", groupName); err != nil {
		return err
	}
	ct, err := q.pool.Exec(context.Background(),
		`INSERT INTO gua_groups (group_name) VALUES ($1) ON CONFLICT DO NOTHING`, groupName)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("group %w", ErrDuplicate)
	}
	return nil
}

// RemoveGroup deletes the group, all its job definitions and their pending
// occurrences, atomically.
func (q *riverQuene) RemoveGroup(groupName string) error {
	ctx := context.Background()
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit
	if _, err := tx.Exec(ctx, `DELETE FROM gua_jobs WHERE group_name=$1`, groupName); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM river_job
		WHERE kind='gua_delivery' AND state IN `+pendingStates+`
		AND args @> jsonb_build_object('group_name', $1::text)`, groupName); err != nil {
		return err
	}
	ct, err := tx.Exec(ctx, `DELETE FROM gua_groups WHERE group_name=$1`, groupName)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("group %w", ErrNotFound)
	}
	return tx.Commit(ctx)
}

func (q *riverQuene) ExistsGroup(groupName string) (bool, error) {
	var n int
	err := q.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM gua_groups WHERE group_name=$1`, groupName).Scan(&n)
	return n > 0, err
}

func (q *riverQuene) GroupInfo(groupName string) (string, error) {
	var g string
	err := q.pool.QueryRow(context.Background(),
		`SELECT group_name FROM gua_groups WHERE group_name=$1`, groupName).Scan(&g)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("group %w", ErrNotFound)
	}
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

func (q *riverQuene) Stats() (*Stats, error) {
	ctx := context.Background()
	s := &Stats{Now: time.Now().Unix()}
	if err := q.pool.QueryRow(ctx, `SELECT
			count(*) FILTER (WHERE state IN `+pendingStates+`),
			count(*) FILTER (WHERE state='running'),
			count(*) FILTER (WHERE state='retryable')
		FROM river_job WHERE kind='gua_delivery'
		AND state IN ('available','scheduled','retryable','pending','running')`).
		Scan(&s.ReadyQueueDepth, &s.Running, &s.Retryable); err != nil {
		return nil, err
	}
	if err := q.pool.QueryRow(ctx, `SELECT
			count(*) FILTER (WHERE active), count(*) FILTER (WHERE NOT active) FROM gua_jobs`).
		Scan(&s.JobsActive, &s.JobsPaused); err != nil {
		return nil, err
	}
	return s, nil
}

func (q *riverQuene) History(group string, limit int) ([]*HistoryEntry, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := q.pool.Query(context.Background(), `SELECT seq, job_id, group_name, type,
		plan_time, exec_time, finish_time, success, message, error, exec_machine_host
		FROM gua_executions WHERE group_name=$1 ORDER BY seq DESC LIMIT $2`, group, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*HistoryEntry, 0)
	for rows.Next() {
		e := &HistoryEntry{}
		if err := rows.Scan(&e.Seq, &e.JobId, &e.GroupName, &e.Type, &e.PlanTime,
			&e.ExecTime, &e.FinishTime, &e.Success, &e.Message, &e.Error, &e.ExecMachineHost); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
