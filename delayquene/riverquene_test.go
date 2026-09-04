package delayquene

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

// newRiverTestQuene builds a Quene on a clean database (GUA_PG_DSN). opts may
// tweak the config before NewRiver (nil is fine).
func newRiverTestQuene(t *testing.T, opts ...func(*RiverConfig)) Quene {
	t.Helper()
	dsn := os.Getenv("GUA_PG_DSN")
	if dsn == "" {
		t.Skip("set GUA_PG_DSN to run the River/Postgres queue tests")
	}
	ctx := context.Background()
	// clean slate so leftover jobs from prior runs don't interfere
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool: %v", err)
	}
	mig, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		t.Fatalf("migrator: %v", err)
	}
	if _, err := mig.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, s := range []string{
		`TRUNCATE river_job`,
		`DROP TABLE IF EXISTS gua_jobs`,
		`DROP TABLE IF EXISTS gua_groups`,
		`DROP TABLE IF EXISTS gua_executions`,
	} {
		if _, err := pool.Exec(ctx, s); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
	pool.Close()

	cfg := &RiverConfig{
		DSN: dsn, MachineHost: "t",
		HistoryTTL: 3600,
		Logger:     testLogger(),
	}
	for _, o := range opts {
		if o != nil {
			o(cfg)
		}
	}
	q, err := NewRiver(cfg)
	if err != nil {
		t.Fatalf("NewRiver: %v", err)
	}
	t.Cleanup(func() { q.Close() })
	return q
}

// counter is a consumer that counts deliveries (optionally per job id).
type counter struct {
	mu    sync.Mutex
	n     int
	byJob map[string]int
	last  TriggerEnvelope
}

func (c *counter) handler(status int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var env TriggerEnvelope
		_ = json.NewDecoder(r.Body).Decode(&env)
		c.mu.Lock()
		c.n++
		if c.byJob == nil {
			c.byJob = map[string]int{}
		}
		c.byJob[env.JobId]++
		c.last = env
		c.mu.Unlock()
		w.WriteHeader(status)
	}
}

func (c *counter) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

// waitFor polls cond until it is true or the deadline passes.
func waitFor(t *testing.T, d time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for {
		if cond() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out after %v waiting for %s", d, what)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// HTTP delivery through the River/Postgres backend.
func TestRiverHTTPDelivery(t *testing.T) {
	q := newRiverTestQuene(t)
	if err := q.RegisterGroup("GRP"); err != nil {
		t.Fatalf("RegisterGroup: %v", err)
	}
	c := &counter{}
	srv := httptest.NewServer(c.handler(http.StatusOK))
	defer srv.Close()

	if err := q.Push(httpJob("GRP", "RJOB1", srv.URL, "river-payload", "@once")); err != nil {
		t.Fatalf("Push: %v", err)
	}
	waitFor(t, 15*time.Second, "delivery", func() bool { return c.count() >= 1 })
	c.mu.Lock()
	env := c.last
	c.mu.Unlock()
	if env.JobId != "RJOB1" || env.Payload != "river-payload" || env.GroupName != "GRP" || env.JobName != "RJOB1" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if env.IdempotencyKey == "" {
		t.Fatalf("missing idempotency_key in envelope: %+v", env)
	}
}

// Recurring jobs reschedule themselves (the worker's scheduleNext loop).
func TestRiverRecurring(t *testing.T) {
	q := newRiverTestQuene(t)
	if err := q.RegisterGroup("GRP"); err != nil {
		t.Fatalf("RegisterGroup: %v", err)
	}
	c := &counter{}
	srv := httptest.NewServer(c.handler(http.StatusOK))
	defer srv.Close()

	if err := q.Push(httpJob("GRP", "RJOB2", srv.URL, "x", "@every 1s")); err != nil {
		t.Fatalf("Push: %v", err)
	}
	waitFor(t, 20*time.Second, "2 recurring deliveries", func() bool { return c.count() >= 2 })
}

// List reflects pushed jobs (gua_jobs source of truth); Delete removes them.
func TestRiverListDelete(t *testing.T) {
	q := newRiverTestQuene(t)
	if err := q.RegisterGroup("GRP"); err != nil {
		t.Fatalf("RegisterGroup: %v", err)
	}
	job := httpJob("GRP", "RJOB3", "http://example.invalid/", "p", "@once")
	job.Exectime = time.Now().Add(time.Hour).Unix() // far future: stays scheduled
	if err := q.Push(job); err != nil {
		t.Fatalf("Push: %v", err)
	}
	jobs, err := q.List("GRP")
	if err != nil || len(jobs) != 1 || jobs[0].Id != "RJOB3" || jobs[0].Payload != "p" {
		t.Fatalf("List = %+v, %v", jobs, err)
	}
	if err := q.Delete("GRP", "RJOB3"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	jobs, _ = q.List("GRP")
	if len(jobs) != 0 {
		t.Fatalf("List after delete = %+v, want empty", jobs)
	}
	s, err := q.Stats()
	if err != nil || s.ReadyQueueDepth != 0 {
		t.Fatalf("occurrence not cancelled: stats=%+v err=%v", s, err)
	}
}

// Pause stops a recurring job from firing further.
func TestRiverPause(t *testing.T) {
	q := newRiverTestQuene(t)
	if err := q.RegisterGroup("GRP"); err != nil {
		t.Fatalf("RegisterGroup: %v", err)
	}
	c := &counter{}
	srv := httptest.NewServer(c.handler(http.StatusOK))
	defer srv.Close()

	if err := q.Push(httpJob("GRP", "RJOB4", srv.URL, "x", "@every 1s")); err != nil {
		t.Fatalf("Push: %v", err)
	}
	waitFor(t, 12*time.Second, "first fire", func() bool { return c.count() > 0 })
	if err := q.Pause("GRP", "RJOB4"); err != nil {
		t.Fatalf("Pause: %v", err)
	}
	before := c.count()
	time.Sleep(4 * time.Second) // longer than a couple of intervals
	after := c.count()
	// One delivery may already be in flight when Pause lands; it completes
	// but must not reschedule, so at most one extra is tolerated.
	if after-before > 1 {
		t.Fatalf("deliveries continued after pause: before=%d after=%d", before, after)
	}
	jobs, _ := q.List("GRP")
	if len(jobs) != 1 || jobs[0].Active {
		t.Fatalf("paused job should be listed with active=false: %+v", jobs)
	}
}

// A fired job leaves a queryable execution-history record in Postgres.
func TestRiverHistory(t *testing.T) {
	q := newRiverTestQuene(t)
	if err := q.RegisterGroup("GRP"); err != nil {
		t.Fatalf("RegisterGroup: %v", err)
	}
	c := &counter{}
	srv := httptest.NewServer(c.handler(http.StatusOK))
	defer srv.Close()

	if err := q.Push(httpJob("GRP", "RHIST", srv.URL, "h", "@once")); err != nil {
		t.Fatalf("Push: %v", err)
	}
	waitFor(t, 15*time.Second, "delivery", func() bool { return c.count() >= 1 })
	var hist []*HistoryEntry
	waitFor(t, 5*time.Second, "history row", func() bool {
		var err error
		hist, err = q.History("GRP", 10)
		if err != nil {
			t.Fatalf("History: %v", err)
		}
		return len(hist) >= 1
	})
	h := hist[0]
	if h.JobId != "RHIST" || !h.Success || h.Type != "HTTP" || h.ExecMachineHost != "t" {
		t.Fatalf("unexpected history entry: %+v", h)
	}
}

// Many same-time jobs must each fire exactly once (SKIP LOCKED, no double-run).
func TestRiverNoDuplicate(t *testing.T) {
	q := newRiverTestQuene(t)
	if err := q.RegisterGroup("GRP"); err != nil {
		t.Fatalf("RegisterGroup: %v", err)
	}
	const n = 30
	c := &counter{}
	srv := httptest.NewServer(c.handler(http.StatusOK))
	defer srv.Close()

	for i := 0; i < n; i++ {
		job := httpJob("GRP", "ND"+strconv.Itoa(i), srv.URL, "x", "@once")
		if err := q.Push(job); err != nil {
			t.Fatalf("Push %d: %v", i, err)
		}
	}
	waitFor(t, 30*time.Second, "all distinct jobs", func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		return len(c.byJob) == n
	})
	time.Sleep(1 * time.Second) // let any straggler duplicate arrive
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, k := range c.byJob {
		if k != 1 {
			t.Fatalf("job %s fired %d times, want exactly 1", id, k)
		}
	}
}

// A delivery that fails once is retried by River until it succeeds.
func TestRiverRetry(t *testing.T) {
	q := newRiverTestQuene(t)
	if err := q.RegisterGroup("GRP"); err != nil {
		t.Fatalf("RegisterGroup: %v", err)
	}
	var mu sync.Mutex
	attempts := 0
	ok := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attempts++
		first := attempts == 1
		mu.Unlock()
		if first {
			w.WriteHeader(http.StatusInternalServerError) // fail the first attempt
			return
		}
		select {
		case ok <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := q.Push(httpJob("GRP", "RETRY", srv.URL, "x", "@once")); err != nil {
		t.Fatalf("Push: %v", err)
	}
	select {
	case <-ok:
	case <-time.After(25 * time.Second):
		t.Fatal("job did not succeed via retry")
	}
	mu.Lock()
	a := attempts
	mu.Unlock()
	if a < 2 {
		t.Fatalf("expected >=2 attempts (fail then succeed), got %d", a)
	}
	// both attempts are in the history: one failure, one success
	waitFor(t, 5*time.Second, "history", func() bool {
		h, _ := q.History("GRP", 10)
		return len(h) >= 2
	})
	h, _ := q.History("GRP", 10)
	if !h[0].Success || h[1].Success || h[1].Error == "" {
		t.Fatalf("history should be [success, failure]: %+v %+v", h[0], h[1])
	}
}

// Group bookkeeping round-trips through Postgres.
func TestRiverGroups(t *testing.T) {
	q := newRiverTestQuene(t)
	if err := q.RegisterGroup("G1"); err != nil {
		t.Fatalf("RegisterGroup: %v", err)
	}
	if err := q.RegisterGroup("G1"); !errorsIs(err, ErrDuplicate) {
		t.Fatalf("duplicate RegisterGroup: got %v, want ErrDuplicate", err)
	}
	if err := q.RegisterGroup("bad name!"); !errorsIs(err, ErrInvalid) {
		t.Fatalf("invalid group name: got %v, want ErrInvalid", err)
	}
	if ok, _ := q.ExistsGroup("G1"); !ok {
		t.Fatal("ExistsGroup G1 = false, want true")
	}
	if ok, _ := q.ExistsGroup("nope"); ok {
		t.Fatal("ExistsGroup nope = true, want false")
	}
	if g, err := q.GroupInfo("G1"); err != nil || g != "G1" {
		t.Fatalf("GroupInfo = %q, %v", g, err)
	}
	if _, err := q.GroupInfo("nope"); !errorsIs(err, ErrNotFound) {
		t.Fatalf("GroupInfo nope: got %v, want ErrNotFound", err)
	}
	groups, err := q.QueryGroups()
	if err != nil || len(groups) != 1 || groups[0] != "G1" {
		t.Fatalf("QueryGroups = %v, %v", groups, err)
	}
	if err := q.RemoveGroup("G1"); err != nil {
		t.Fatalf("RemoveGroup: %v", err)
	}
	if ok, _ := q.ExistsGroup("G1"); ok {
		t.Fatal("group still exists after RemoveGroup")
	}
	if err := q.RemoveGroup("G1"); !errorsIs(err, ErrNotFound) {
		t.Fatalf("RemoveGroup twice: got %v, want ErrNotFound", err)
	}
}
