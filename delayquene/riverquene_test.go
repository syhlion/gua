package delayquene

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
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
	pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS gua_groups (group_name text PRIMARY KEY, created_at timestamptz DEFAULT now())`)
	pool.Exec(ctx, `TRUNCATE gua_groups`)
	pool.Close()

	q, err := NewRiver(&RiverConfig{
		DSN: dsn, MachineHost: "t", MachineIp: "127.0.0.1", MachineMac: "m",
		Logger: testLogger(),
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
