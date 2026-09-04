package delayquene

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	guaproto "github.com/syhlion/gua/proto"
	"google.golang.org/grpc"
)

func errorsIs(err, target error) bool { return errors.Is(err, target) }

func namedJob(group, id, name, target, interval string) *guaproto.Job {
	j := httpJob(group, id, target, "p", interval)
	j.Name = name
	return j
}

// Push validates everything the API layers used to check piecemeal.
func TestRiverPushValidation(t *testing.T) {
	q := newRiverTestQuene(t)
	if err := q.RegisterGroup("GRP"); err != nil {
		t.Fatalf("RegisterGroup: %v", err)
	}
	far := time.Now().Add(time.Hour).Unix()
	mk := func(mut func(j *guaproto.Job)) *guaproto.Job {
		j := httpJob("GRP", "OK1", "http://example.invalid/", "p", "@once")
		j.Exectime = far
		mut(j)
		return j
	}
	cases := []struct {
		name string
		job  *guaproto.Job
		want error
	}{
		{"bad group name", mk(func(j *guaproto.Job) { j.GroupName = "no way" }), ErrInvalid},
		{"bad job id", mk(func(j *guaproto.Job) { j.Id = "a-b" }), ErrInvalid},
		{"job id too long", mk(func(j *guaproto.Job) { j.Id = "abcdefghijklmnopqrstuvw" }), ErrInvalid},
		{"no name", mk(func(j *guaproto.Job) { j.Name = "" }), ErrInvalid},
		{"empty target", mk(func(j *guaproto.Job) { j.RequestUrl = "HTTP@" }), ErrInvalid},
		{"unknown scheme", mk(func(j *guaproto.Job) { j.RequestUrl = "FTP@x" }), ErrInvalid},
		{"bad pattern", mk(func(j *guaproto.Job) { j.IntervalPattern = "not a cron at all" }), ErrInvalid},
		{"7-field pattern", mk(func(j *guaproto.Job) { j.IntervalPattern = "* * * * * * *" }), ErrInvalid},
		{"negative exec_time", mk(func(j *guaproto.Job) { j.Exectime = -1 }), ErrInvalid},
		{"negative timeout", mk(func(j *guaproto.Job) { j.Timeout = -1 }), ErrInvalid},
		{"no group", mk(func(j *guaproto.Job) { j.GroupName = "NOPE" }), ErrNotFound},
		{"ok 5-field cron", mk(func(j *guaproto.Job) { j.IntervalPattern = "*/5 * * * *" }), nil},
		{"duplicate id", mk(func(j *guaproto.Job) {}), ErrDuplicate},
	}
	for _, c := range cases {
		err := q.Push(c.job)
		if c.want == nil && err != nil {
			t.Errorf("%s: unexpected error %v", c.name, err)
		}
		if c.want != nil && !errors.Is(err, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, err, c.want)
		}
	}
	jobs, _ := q.List("GRP")
	if len(jobs) != 1 {
		t.Fatalf("only the valid push should be stored, got %+v", jobs)
	}
}

// Edit takes effect on the occurrence that is already scheduled.
func TestRiverEditAffectsPendingOccurrence(t *testing.T) {
	q := newRiverTestQuene(t)
	if err := q.RegisterGroup("GRP"); err != nil {
		t.Fatalf("RegisterGroup: %v", err)
	}
	a, b := &counter{}, &counter{}
	srvA := httptest.NewServer(a.handler(http.StatusOK))
	defer srvA.Close()
	srvB := httptest.NewServer(b.handler(http.StatusOK))
	defer srvB.Close()

	j := httpJob("GRP", "EDIT1", srvA.URL, "old", "@once")
	j.Exectime = time.Now().Add(3 * time.Second).Unix()
	if err := q.Push(j); err != nil {
		t.Fatalf("Push: %v", err)
	}
	if err := q.Edit("GRP", "EDIT1", "HTTP@"+srvB.URL, "new"); err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if err := q.Edit("GRP", "NOPE", "HTTP@"+srvB.URL, "new"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Edit unknown job: got %v, want ErrNotFound", err)
	}
	if err := q.Edit("GRP", "EDIT1", "HTTP@", "new"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Edit empty target: got %v, want ErrInvalid", err)
	}
	waitFor(t, 15*time.Second, "delivery to the edited target", func() bool { return b.count() >= 1 })
	b.mu.Lock()
	env := b.last
	b.mu.Unlock()
	if env.Payload != "new" {
		t.Fatalf("edited payload not delivered: %+v", env)
	}
	time.Sleep(1 * time.Second)
	if a.count() != 0 {
		t.Fatalf("old target still received %d deliveries", a.count())
	}
}

// Active on an already-active job replaces the pending occurrence: one fire.
func TestRiverActiveReplacesOccurrence(t *testing.T) {
	q := newRiverTestQuene(t)
	if err := q.RegisterGroup("GRP"); err != nil {
		t.Fatalf("RegisterGroup: %v", err)
	}
	c := &counter{}
	srv := httptest.NewServer(c.handler(http.StatusOK))
	defer srv.Close()

	at := time.Now().Add(3 * time.Second).Unix()
	j := httpJob("GRP", "ACT1", srv.URL, "p", "@once")
	j.Exectime = at
	if err := q.Push(j); err != nil {
		t.Fatalf("Push: %v", err)
	}
	// client retry / double click
	if err := q.Active("GRP", "ACT1", at); err != nil {
		t.Fatalf("Active: %v", err)
	}
	if err := q.Active("GRP", "ACT1", at); err != nil {
		t.Fatalf("Active again: %v", err)
	}
	s, _ := q.Stats()
	if s.ReadyQueueDepth != 1 {
		t.Fatalf("pending occurrences = %d, want 1", s.ReadyQueueDepth)
	}
	waitFor(t, 15*time.Second, "delivery", func() bool { return c.count() >= 1 })
	time.Sleep(3 * time.Second)
	if c.count() != 1 {
		t.Fatalf("delivered %d times, want exactly 1", c.count())
	}
}

// Active with the wrong group is a not-found, and schedules nothing.
func TestRiverActiveWrongGroup(t *testing.T) {
	q := newRiverTestQuene(t)
	for _, g := range []string{"GRP", "OTHER"} {
		if err := q.RegisterGroup(g); err != nil {
			t.Fatalf("RegisterGroup: %v", err)
		}
	}
	j := httpJob("GRP", "XG1", "http://example.invalid/", "p", "@once")
	j.Exectime = time.Now().Add(time.Hour).Unix()
	if err := q.Push(j); err != nil {
		t.Fatalf("Push: %v", err)
	}
	if err := q.Active("OTHER", "XG1", time.Now().Unix()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Active wrong group: got %v, want ErrNotFound", err)
	}
	if err := q.Pause("OTHER", "XG1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Pause wrong group: got %v, want ErrNotFound", err)
	}
	if err := q.Delete("OTHER", "XG1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete wrong group: got %v, want ErrNotFound", err)
	}
	s, _ := q.Stats()
	if s.ReadyQueueDepth != 1 {
		t.Fatalf("pending occurrences = %d, want the original 1", s.ReadyQueueDepth)
	}
	jobs, _ := q.List("GRP")
	if len(jobs) != 1 || !jobs[0].Active || jobs[0].Exectime != j.Exectime {
		t.Fatalf("job should be untouched: %+v", jobs)
	}
}

// Pause then Active resumes a recurring job.
func TestRiverPauseThenActive(t *testing.T) {
	q := newRiverTestQuene(t)
	if err := q.RegisterGroup("GRP"); err != nil {
		t.Fatalf("RegisterGroup: %v", err)
	}
	c := &counter{}
	srv := httptest.NewServer(c.handler(http.StatusOK))
	defer srv.Close()

	if err := q.Push(httpJob("GRP", "PA1", srv.URL, "p", "@every 1s")); err != nil {
		t.Fatalf("Push: %v", err)
	}
	waitFor(t, 12*time.Second, "first fire", func() bool { return c.count() > 0 })
	if err := q.Pause("GRP", "PA1"); err != nil {
		t.Fatalf("Pause: %v", err)
	}
	time.Sleep(3 * time.Second)
	paused := c.count()
	if err := q.Active("GRP", "PA1", 0); err != nil { // 0 = now
		t.Fatalf("Active: %v", err)
	}
	waitFor(t, 12*time.Second, "fire after re-activate", func() bool { return c.count() > paused })
	jobs, _ := q.List("GRP")
	if len(jobs) != 1 || !jobs[0].Active {
		t.Fatalf("job should be active again: %+v", jobs)
	}
}

// A 6-field cron recurs, and List shows the next fire time after each run.
func TestRiverCronRecurringUpdatesNextExec(t *testing.T) {
	q := newRiverTestQuene(t)
	if err := q.RegisterGroup("GRP"); err != nil {
		t.Fatalf("RegisterGroup: %v", err)
	}
	c := &counter{}
	srv := httptest.NewServer(c.handler(http.StatusOK))
	defer srv.Close()

	j := httpJob("GRP", "CRON1", srv.URL, "p", "*/2 * * * * *") // every even second
	first := time.Now().Unix()
	j.Exectime = first
	if err := q.Push(j); err != nil {
		t.Fatalf("Push: %v", err)
	}
	waitFor(t, 25*time.Second, "2 cron deliveries", func() bool { return c.count() >= 2 })
	jobs, _ := q.List("GRP")
	if len(jobs) != 1 {
		t.Fatalf("List = %+v", jobs)
	}
	if jobs[0].Exectime <= first {
		t.Fatalf("exec_time should advance to the next fire, still %d", jobs[0].Exectime)
	}
	if jobs[0].Exectime%2 != 0 {
		t.Fatalf("next fire %d should land on an even second", jobs[0].Exectime)
	}
}

// A @once job's definition is dropped after delivery.
func TestRiverOnceDropsDefinition(t *testing.T) {
	q := newRiverTestQuene(t)
	if err := q.RegisterGroup("GRP"); err != nil {
		t.Fatalf("RegisterGroup: %v", err)
	}
	c := &counter{}
	srv := httptest.NewServer(c.handler(http.StatusOK))
	defer srv.Close()
	if err := q.Push(httpJob("GRP", "ONCE1", srv.URL, "p", "@once")); err != nil {
		t.Fatalf("Push: %v", err)
	}
	waitFor(t, 15*time.Second, "delivery", func() bool { return c.count() >= 1 })
	waitFor(t, 5*time.Second, "definition removed", func() bool {
		jobs, _ := q.List("GRP")
		return len(jobs) == 0
	})
}

// RemoveGroup takes the group's jobs and pending occurrences with it.
func TestRiverRemoveGroupCascade(t *testing.T) {
	q := newRiverTestQuene(t)
	for _, g := range []string{"GRP", "KEEP"} {
		if err := q.RegisterGroup(g); err != nil {
			t.Fatalf("RegisterGroup: %v", err)
		}
	}
	far := time.Now().Add(time.Hour).Unix()
	for _, id := range []string{"C1", "C2"} {
		j := httpJob("GRP", id, "http://example.invalid/", "p", "@once")
		j.Exectime = far
		if err := q.Push(j); err != nil {
			t.Fatalf("Push: %v", err)
		}
	}
	k := httpJob("KEEP", "K1", "http://example.invalid/", "p", "@once")
	k.Exectime = far
	if err := q.Push(k); err != nil {
		t.Fatalf("Push: %v", err)
	}
	if err := q.RemoveGroup("GRP"); err != nil {
		t.Fatalf("RemoveGroup: %v", err)
	}
	if jobs, _ := q.List("GRP"); len(jobs) != 0 {
		t.Fatalf("jobs survived RemoveGroup: %+v", jobs)
	}
	s, _ := q.Stats()
	if s.ReadyQueueDepth != 1 || s.JobsActive != 1 {
		t.Fatalf("only KEEP's occurrence should remain: %+v", s)
	}
	if err := q.Push(httpJob("GRP", "C3", "http://example.invalid/", "p", "@once")); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Push into removed group: got %v, want ErrNotFound", err)
	}
}

// DeleteJobs filters by name and returns the count.
func TestRiverDeleteJobsByName(t *testing.T) {
	q := newRiverTestQuene(t)
	if err := q.RegisterGroup("GRP"); err != nil {
		t.Fatalf("RegisterGroup: %v", err)
	}
	far := time.Now().Add(time.Hour).Unix()
	for _, p := range [][2]string{{"D1", "a"}, {"D2", "a"}, {"D3", "b"}} {
		j := namedJob("GRP", p[0], p[1], "http://example.invalid/", "@once")
		j.Exectime = far
		if err := q.Push(j); err != nil {
			t.Fatalf("Push: %v", err)
		}
	}
	n, err := q.DeleteJobs("GRP", "a")
	if err != nil || n != 2 {
		t.Fatalf("DeleteJobs(a) = %d, %v; want 2", n, err)
	}
	jobs, _ := q.List("GRP")
	if len(jobs) != 1 || jobs[0].Id != "D3" {
		t.Fatalf("List after DeleteJobs(a) = %+v", jobs)
	}
	s, _ := q.Stats()
	if s.ReadyQueueDepth != 1 {
		t.Fatalf("pending occurrences = %d, want 1", s.ReadyQueueDepth)
	}
	n, err = q.DeleteJobs("GRP", "")
	if err != nil || n != 1 {
		t.Fatalf("DeleteJobs(all) = %d, %v; want 1", n, err)
	}
	s, _ = q.Stats()
	if s.ReadyQueueDepth != 0 || s.JobsActive != 0 {
		t.Fatalf("stats after clear = %+v", s)
	}
}

// When every attempt fails the job is paused, not silently discarded.
func TestRiverAttemptsExhaustedPausesJob(t *testing.T) {
	q := newRiverTestQuene(t, func(c *RiverConfig) { c.MaxAttempts = 2 })
	if err := q.RegisterGroup("GRP"); err != nil {
		t.Fatalf("RegisterGroup: %v", err)
	}
	c := &counter{}
	srv := httptest.NewServer(c.handler(http.StatusInternalServerError))
	defer srv.Close()
	if err := q.Push(httpJob("GRP", "EX1", srv.URL, "p", "@every 1s")); err != nil {
		t.Fatalf("Push: %v", err)
	}
	waitFor(t, 30*time.Second, "job paused after exhausting attempts", func() bool {
		jobs, _ := q.List("GRP")
		return len(jobs) == 1 && !jobs[0].Active
	})
	if c.count() != 2 {
		t.Fatalf("attempts = %d, want 2", c.count())
	}
	h, _ := q.History("GRP", 10)
	if len(h) != 2 || h[0].Success || h[1].Success {
		t.Fatalf("history should hold 2 failures: %+v", h)
	}
	s, _ := q.Stats()
	if s.ReadyQueueDepth != 0 || s.JobsPaused != 1 {
		t.Fatalf("stats = %+v", s)
	}
}

// flakyCallback is a gRPC GuaCallback consumer that rejects the first call.
type flakyCallback struct {
	guaproto.UnimplementedGuaCallbackServer
	mu    sync.Mutex
	calls int
	keys  []string
}

func (f *flakyCallback) OnJobTrigger(ctx context.Context, in *guaproto.JobTrigger) (*guaproto.JobResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.keys = append(f.keys, in.IdempotencyKey)
	if f.calls == 1 {
		return &guaproto.JobResult{Success: false, Message: "not now"}, nil
	}
	return &guaproto.JobResult{Success: true, Message: "ok"}, nil
}

// A gRPC consumer answering Success=false is retried, with the same
// idempotency key, until it succeeds.
func TestRiverGRPCFailureRetries(t *testing.T) {
	q := newRiverTestQuene(t)
	if err := q.RegisterGroup("GRP"); err != nil {
		t.Fatalf("RegisterGroup: %v", err)
	}
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	cb := &flakyCallback{}
	srv := grpc.NewServer()
	guaproto.RegisterGuaCallbackServer(srv, cb)
	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()

	j := httpJob("GRP", "G1", "", "p", "@once")
	j.RequestUrl = "GRPC@" + lis.Addr().String()
	if err := q.Push(j); err != nil {
		t.Fatalf("Push: %v", err)
	}
	waitFor(t, 25*time.Second, "successful retry", func() bool {
		cb.mu.Lock()
		defer cb.mu.Unlock()
		return cb.calls >= 2
	})
	cb.mu.Lock()
	keys := append([]string(nil), cb.keys...)
	cb.mu.Unlock()
	if keys[0] == "" || keys[0] != keys[1] {
		t.Fatalf("idempotency key must be stable across retries: %v", keys)
	}
	waitFor(t, 5*time.Second, "history", func() bool {
		h, _ := q.History("GRP", 10)
		return len(h) >= 2
	})
	h, _ := q.History("GRP", 10)
	if h[0].Type != "GRPC" || !h[0].Success || h[0].Message != "ok" || h[1].Success || h[1].Error == "" {
		t.Fatalf("history = %+v %+v", h[0], h[1])
	}
}

// Old history rows are pruned by the periodic job.
func TestRiverHistoryPrune(t *testing.T) {
	q := newRiverTestQuene(t, func(c *RiverConfig) {
		c.HistoryTTL = 1
		c.PruneInterval = 1 * time.Second
	})
	rq := q.(*riverQuene)
	ctx := context.Background()
	if _, err := rq.pool.Exec(ctx, `INSERT INTO gua_executions
		(job_id, group_name, type, plan_time, exec_time, finish_time, success, created_at)
		VALUES ('old','GRP','HTTP',0,0,0,true, now() - interval '1 hour')`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	waitFor(t, 20*time.Second, "old row pruned", func() bool {
		h, err := q.History("GRP", 10)
		if err != nil {
			t.Fatalf("History: %v", err)
		}
		return len(h) == 0
	})
}

// Stats counts jobs by state and occurrences by queue state.
func TestRiverStats(t *testing.T) {
	q := newRiverTestQuene(t)
	if err := q.RegisterGroup("GRP"); err != nil {
		t.Fatalf("RegisterGroup: %v", err)
	}
	far := time.Now().Add(time.Hour).Unix()
	for _, id := range []string{"S1", "S2"} {
		j := httpJob("GRP", id, "http://example.invalid/", "p", "@once")
		j.Exectime = far
		if err := q.Push(j); err != nil {
			t.Fatalf("Push: %v", err)
		}
	}
	if err := q.Pause("GRP", "S2"); err != nil {
		t.Fatalf("Pause: %v", err)
	}
	s, err := q.Stats()
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if s.ReadyQueueDepth != 1 || s.JobsActive != 1 || s.JobsPaused != 1 || s.Running != 0 || s.Retryable != 0 || s.Now == 0 {
		t.Fatalf("stats = %+v", s)
	}
}
