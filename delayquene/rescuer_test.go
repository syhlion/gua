package delayquene

import (
	"context"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

// rescueProbeArgs is a stand-in River job used only to exercise the rescuer.
type rescueProbeArgs struct{}

func (rescueProbeArgs) Kind() string { return "rescue_probe" }

// rescueProbeWorker simulates a worker process that dies mid-run: on the first
// attempt it never returns (the row stays `running`), so the only thing that
// can re-run it is River's rescuer — not the normal error/retry path. The
// rescued re-run (attempt >= 2) signals success.
type rescueProbeWorker struct {
	river.WorkerDefaults[rescueProbeArgs]
	attempts *int32
	ran2     chan struct{}
}

func (w *rescueProbeWorker) Work(ctx context.Context, job *river.Job[rescueProbeArgs]) error {
	atomic.AddInt32(w.attempts, 1)
	if job.Attempt <= 1 {
		// A crashed process wouldn't honour ctx cancellation, so we deliberately
		// ignore ctx and block well past the rescue window. The row is left in
		// `running` with no live worker finalizing it — exactly the stuck state
		// the rescuer exists to recover. (This goroutine is abandoned when the
		// test ends; harmless.) Sleep longer than the wait window below so
		// attempt 1 never returns mid-test.
		time.Sleep(150 * time.Second)
		return nil
	}
	select {
	case w.ran2 <- struct{}{}:
	default:
	}
	return nil
}

// TestRiverRescuer is the "job not lost" acceptance test: a job whose worker
// dies mid-run (row stuck in `running`) must be picked back up by River's
// rescuer and re-run. This validates, against the pinned River version, the
// crash-recovery guarantee gua relies on after deleting its hand-rolled
// down-server/JobCheck reclaim layer.
//
// It uses a stand-in worker rather than the real delivery worker because a
// faithful "process died" state can't be produced in-process by the real
// worker (returning from Work — even on ctx cancel — is the retry path, not the
// rescuer path); only a worker that never returns leaves the row genuinely
// stuck.
func TestRiverRescuer(t *testing.T) {
	dsn := os.Getenv("GUA_PG_DSN")
	if dsn == "" {
		t.Skip("set GUA_PG_DSN to run the River/Postgres queue tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool: %v", err)
	}
	defer pool.Close()
	mig, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		t.Fatalf("migrator: %v", err)
	}
	if _, err := mig.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool.Exec(ctx, `TRUNCATE river_job`)

	var attempts int32
	ran2 := make(chan struct{}, 1)
	workers := river.NewWorkers()
	river.AddWorker(workers, &rescueProbeWorker{attempts: &attempts, ran2: ran2})

	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues:  map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 5}},
		Workers: workers,
		// Squeeze the rescue window so the test doesn't have to wait the 1h
		// default. RescueStuckJobsAfter must be >= JobTimeout (River validates).
		JobTimeout:           1 * time.Second,
		RescueStuckJobsAfter: 1 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() {
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		client.Stop(sctx)
	}()

	if _, err := client.Insert(ctx, rescueProbeArgs{}, nil); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	// River's rescuer runs on a fixed ~30s interval, so a missed tick can push
	// detection toward ~60s on a slow/loaded runner; allow generous headroom.
	select {
	case <-ran2:
	case <-time.After(90 * time.Second):
		t.Fatalf("stuck job was not rescued/re-run within 75s (attempts=%d)", atomic.LoadInt32(&attempts))
	}
	if a := atomic.LoadInt32(&attempts); a < 2 {
		t.Fatalf("expected >= 2 attempts (initial stuck run + rescued re-run), got %d", a)
	}
}
