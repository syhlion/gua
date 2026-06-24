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

func newRiverTestQuene(t *testing.T) Quene {
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
	pool.Exec(ctx, `TRUNCATE river_job`)
	pool.Exec(ctx, `DROP TABLE IF EXISTS gua_jobs`)
	pool.Exec(ctx, `DROP TABLE IF EXISTS gua_groups`)
	pool.Exec(ctx, `DROP TABLE IF EXISTS gua_executions`)
	pool.Close()

	q, err := NewRiver(&RiverConfig{
		DSN: dsn, MachineHost: "t", MachineIp: "127.0.0.1", MachineMac: "m",
		HistoryTTL: 3600,
		Logger:     testLogger(),
	})
	if err != nil {
		t.Fatalf("NewRiver: %v", err)
	}
	t.Cleanup(func() { q.Close() })
	return q
}

// HTTP delivery through the River/Postgres backend.
func TestRiverHTTPDelivery(t *testing.T) {
	q := newRiverTestQuene(t)
	if err := q.RegisterGroup("GRP"); err != nil {
		t.Fatalf("RegisterGroup: %v", err)
	}
	got := make(chan TriggerEnvelope, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var env TriggerEnvelope
		json.NewDecoder(r.Body).Decode(&env)
		select {
		case got <- env:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := q.Push(httpJob("GRP", "RJOB1", srv.URL, "river-payload", "@once")); err != nil {
		t.Fatalf("Push: %v", err)
	}
	select {
	case env := <-got:
		if env.JobId != "RJOB1" || env.Payload != "river-payload" || env.GroupName != "GRP" {
			t.Fatalf("unexpected envelope: %+v", env)
		}
		if env.IdempotencyKey == "" {
			t.Fatalf("missing idempotency_key in envelope: %+v", env)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("river HTTP delivery did not fire within 15s")
	}
}

// Recurring jobs reschedule themselves (the worker's insertNext loop).
func TestRiverRecurring(t *testing.T) {
	q := newRiverTestQuene(t)
	if err := q.RegisterGroup("GRP"); err != nil {
		t.Fatalf("RegisterGroup: %v", err)
	}
	var mu sync.Mutex
	count := 0
	done := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		count++
		if count == 2 {
			close(done)
		}
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := q.Push(httpJob("GRP", "RJOB2", srv.URL, "x", "@every 1s")); err != nil {
		t.Fatalf("Push: %v", err)
	}
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		mu.Lock()
		c := count
		mu.Unlock()
		t.Fatalf("recurring fired %d times, want >= 2", c)
	}
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
}

// Pause stops a recurring job from firing further.
func TestRiverPause(t *testing.T) {
	q := newRiverTestQuene(t)
	if err := q.RegisterGroup("GRP"); err != nil {
		t.Fatalf("RegisterGroup: %v", err)
	}
	var mu sync.Mutex
	count := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		count++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := q.Push(httpJob("GRP", "RJOB4", srv.URL, "x", "@every 1s")); err != nil {
		t.Fatalf("Push: %v", err)
	}
	// wait for the first fire (River's scheduler has a few seconds of latency)
	deadline := time.Now().Add(12 * time.Second)
	for {
		mu.Lock()
		c := count
		mu.Unlock()
		if c > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("job never fired before pause")
		}
		time.Sleep(200 * time.Millisecond)
	}
	if err := q.Pause("GRP", "RJOB4"); err != nil {
		t.Fatalf("Pause: %v", err)
	}
	mu.Lock()
	before := count
	mu.Unlock()
	time.Sleep(4 * time.Second) // longer than a couple of intervals
	mu.Lock()
	after := count
	mu.Unlock()
	if after-before > 1 { // allow one in-flight racing the pause
		t.Fatalf("deliveries continued after pause: before=%d after=%d", before, after)
	}
}

// A fired job leaves a queryable execution-history record in Postgres.
func TestRiverHistory(t *testing.T) {
	q := newRiverTestQuene(t)
	if err := q.RegisterGroup("GRP"); err != nil {
		t.Fatalf("RegisterGroup: %v", err)
	}
	got := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case got <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := q.Push(httpJob("GRP", "RHIST", srv.URL, "h", "@once")); err != nil {
		t.Fatalf("Push: %v", err)
	}
	select {
	case <-got:
	case <-time.After(15 * time.Second):
		t.Fatal("job never fired")
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		hist, err := q.History("GRP", 10)
		if err != nil {
			t.Fatalf("History: %v", err)
		}
		if len(hist) >= 1 {
			h := hist[0]
			if h.JobId != "RHIST" || !h.Success || h.Type != "HTTP" {
				t.Fatalf("unexpected history entry: %+v", h)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("history not recorded")
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// Many same-time jobs must each fire exactly once (SKIP LOCKED, no double-run).
func TestRiverNoDuplicate(t *testing.T) {
	q := newRiverTestQuene(t)
	if err := q.RegisterGroup("GRP"); err != nil {
		t.Fatalf("RegisterGroup: %v", err)
	}
	const n = 30
	var mu sync.Mutex
	seen := map[string]int{}
	done := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var env TriggerEnvelope
		json.NewDecoder(r.Body).Decode(&env)
		mu.Lock()
		seen[env.JobId]++
		if len(seen) == n {
			select {
			case done <- struct{}{}:
			default:
			}
		}
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	for i := 0; i < n; i++ {
		job := httpJob("GRP", "ND"+strconv.Itoa(i), srv.URL, "x", "@once")
		if err := q.Push(job); err != nil {
			t.Fatalf("Push %d: %v", i, err)
		}
	}
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		mu.Lock()
		c := len(seen)
		mu.Unlock()
		t.Fatalf("only %d/%d distinct jobs fired", c, n)
	}
	time.Sleep(1 * time.Second) // let any straggler duplicate arrive
	mu.Lock()
	defer mu.Unlock()
	for id, c := range seen {
		if c != 1 {
			t.Fatalf("job %s fired %d times, want exactly 1", id, c)
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
}

// Group bookkeeping round-trips through Postgres.
func TestRiverGroups(t *testing.T) {
	q := newRiverTestQuene(t)
	if err := q.RegisterGroup("G1"); err != nil {
		t.Fatalf("RegisterGroup: %v", err)
	}
	if err := q.RegisterGroup("G1"); err == nil {
		t.Fatal("duplicate RegisterGroup should error")
	}
	if n, _ := q.ExistsGroup("G1"); n != 1 {
		t.Fatalf("ExistsGroup G1 = %d, want 1", n)
	}
	if n, _ := q.ExistsGroup("nope"); n != 0 {
		t.Fatalf("ExistsGroup nope = %d, want 0", n)
	}
	groups, err := q.QueryGroups()
	if err != nil || len(groups) != 1 || groups[0] != "G1" {
		t.Fatalf("QueryGroups = %v, %v", groups, err)
	}
	if err := q.RemoveGroup("G1"); err != nil {
		t.Fatalf("RemoveGroup: %v", err)
	}
	if n, _ := q.ExistsGroup("G1"); n != 0 {
		t.Fatal("group still exists after RemoveGroup")
	}
}
