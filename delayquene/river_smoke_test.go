package delayquene

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

type smokeArgs struct {
	Msg string `json:"msg"`
}

func (smokeArgs) Kind() string { return "gua_smoke" }

type smokeWorker struct {
	river.WorkerDefaults[smokeArgs]
	fired chan string
}

func (w *smokeWorker) Work(ctx context.Context, job *river.Job[smokeArgs]) error {
	select {
	case w.fired <- job.Args.Msg:
	default:
	}
	return nil
}

// TestRiverSmoke de-risks the River + Postgres stack in this environment:
// migrate up, insert a delayed job, confirm a worker runs it.
// Run with: GUA_PG_DSN='postgres://postgres:gua@localhost:5433/gua?sslmode=disable' go test ./delayquene/ -run TestRiverSmoke -v
func TestRiverSmoke(t *testing.T) {
	dsn := os.Getenv("GUA_PG_DSN")
	if dsn == "" {
		t.Skip("set GUA_PG_DSN to run the River/Postgres smoke test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool: %v", err)
	}
	defer pool.Close()

	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		t.Fatalf("migrator: %v", err)
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	fired := make(chan string, 1)
	workers := river.NewWorkers()
	river.AddWorker(workers, &smokeWorker{fired: fired})

	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues:  map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 5}},
		Workers: workers,
	})
	if err != nil {
		t.Fatalf("river client: %v", err)
	}
	if err := client.Start(ctx); err != nil {
		t.Fatalf("river start: %v", err)
	}
	defer client.Stop(ctx)

	if _, err := client.Insert(ctx, smokeArgs{Msg: "hi"}, &river.InsertOpts{
		ScheduledAt: time.Now().Add(1 * time.Second),
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	select {
	case m := <-fired:
		if m != "hi" {
			t.Fatalf("got %q want hi", m)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("river job did not run within 15s")
	}
}
