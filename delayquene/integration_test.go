package delayquene

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gomodule/redigo/redis"
	"github.com/sirupsen/logrus"
	guaproto "github.com/syhlion/gua/proto"
	"google.golang.org/grpc"
)

func testLogger() *logrus.Logger {
	l := logrus.New()
	if os.Getenv("GUA_TEST_LOG") != "" {
		l.SetOutput(os.Stderr)
		l.SetLevel(logrus.DebugLevel)
		return l
	}
	l.SetOutput(io.Discard)
	l.SetLevel(logrus.PanicLevel)
	return l
}

func testPool(addr string, db int) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     128,
		MaxActive:   0, // unlimited: bucketSize blocking BLPOP workers each hold a conn
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", addr)
			if err != nil {
				return nil, err
			}
			if _, err := c.Do("SELECT", db); err != nil {
				c.Close()
				return nil, err
			}
			return c, nil
		},
	}
}

func newTestQuene(t *testing.T) Quene {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	cfg := &Config{
		MachineHost: "test-host",
		MachineMac:  "00:00:00:00:00:00",
		MachineIp:   "127.0.0.1",
		Logger:      testLogger(),
	}
	q, err := New(cfg, testPool(mr.Addr(), 3), testPool(mr.Addr(), 1), testPool(mr.Addr(), 2))
	if err != nil {
		mr.Close()
		t.Fatalf("New quene: %v", err)
	}
	t.Cleanup(func() {
		q.Close()
		mr.Close()
	})
	return q
}

func httpJob(group, id, target, payload, interval string) *guaproto.Job {
	return &guaproto.Job{
		Name:            id,
		GroupName:       group,
		Id:              id,
		Exectime:        time.Now().Add(1 * time.Second).Unix(),
		IntervalPattern: interval,
		RequestUrl:      "HTTP@" + target,
		Payload:         payload,
		Active:          true,
	}
}

// HTTP delivery: a job firing should POST the trigger envelope to the target.
func TestHTTPDelivery(t *testing.T) {
	q := newTestQuene(t)
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

	if err := q.Push(httpJob("GRP", "JOB1", srv.URL, "hello-payload", "@once")); err != nil {
		t.Fatalf("Push: %v", err)
	}

	select {
	case env := <-got:
		if env.JobId != "JOB1" || env.Payload != "hello-payload" || env.GroupName != "GRP" {
			t.Fatalf("unexpected envelope: %+v", env)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("timed out waiting for HTTP callback")
	}
}

type callbackServer struct {
	ch chan *guaproto.JobTrigger
}

func (s *callbackServer) OnJobTrigger(ctx context.Context, req *guaproto.JobTrigger) (*guaproto.JobResult, error) {
	select {
	case s.ch <- req:
	default:
	}
	return &guaproto.JobResult{Success: true, Message: "ok"}, nil
}

// gRPC delivery: a job firing should call OnJobTrigger on the consumer.
func TestGRPCDelivery(t *testing.T) {
	q := newTestQuene(t)
	if err := q.RegisterGroup("GRP"); err != nil {
		t.Fatalf("RegisterGroup: %v", err)
	}

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	cb := &callbackServer{ch: make(chan *guaproto.JobTrigger, 1)}
	gs := grpc.NewServer()
	guaproto.RegisterGuaCallbackServer(gs, cb)
	go gs.Serve(lis)
	defer gs.Stop()

	job := httpJob("GRP", "JOB2", lis.Addr().String(), "grpc-payload", "@once")
	job.RequestUrl = "GRPC@" + lis.Addr().String()
	if err := q.Push(job); err != nil {
		t.Fatalf("Push: %v", err)
	}

	select {
	case req := <-cb.ch:
		if req.JobId != "JOB2" || req.Payload != "grpc-payload" {
			t.Fatalf("unexpected trigger: %+v", req)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("timed out waiting for gRPC callback")
	}
}

// Recurring jobs should fire repeatedly on their interval.
func TestRecurringDelivery(t *testing.T) {
	q := newTestQuene(t)
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

	if err := q.Push(httpJob("GRP", "JOB3", srv.URL, "x", "@every 1s")); err != nil {
		t.Fatalf("Push: %v", err)
	}

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		mu.Lock()
		c := count
		mu.Unlock()
		t.Fatalf("recurring job fired %d times, want >= 2", c)
	}
}

// Pausing a recurring job should stop further deliveries.
func TestPauseStopsDelivery(t *testing.T) {
	q := newTestQuene(t)
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

	if err := q.Push(httpJob("GRP", "JOB4", srv.URL, "x", "@every 1s")); err != nil {
		t.Fatalf("Push: %v", err)
	}
	time.Sleep(2500 * time.Millisecond)
	if err := q.Pause("GRP", "JOB4"); err != nil {
		t.Fatalf("Pause: %v", err)
	}
	mu.Lock()
	before := count
	mu.Unlock()
	if before == 0 {
		t.Fatal("job never fired before pause")
	}
	time.Sleep(3 * time.Second)
	mu.Lock()
	after := count
	mu.Unlock()
	// allow at most one in-flight delivery racing the pause
	if after-before > 1 {
		t.Fatalf("deliveries continued after pause: before=%d after=%d", before, after)
	}
}

// Stats reports cluster slot health and queue depth.
func TestStats(t *testing.T) {
	q := newTestQuene(t)
	s, err := q.Stats()
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if len(s.Servers) < 1 {
		t.Fatalf("expected >= 1 server slot, got %+v", s.Servers)
	}
	if !s.Servers[0].Alive {
		t.Fatalf("freshly started server should be alive: %+v", s.Servers[0])
	}
	if s.ReadyQueueDepth < 0 || s.DownServerBacklog < 0 {
		t.Fatalf("negative depths: %+v", s)
	}
}

// List reflects pushed jobs; Delete removes them.
func TestListAndDelete(t *testing.T) {
	q := newTestQuene(t)
	if err := q.RegisterGroup("GRP"); err != nil {
		t.Fatalf("RegisterGroup: %v", err)
	}
	job := httpJob("GRP", "JOB5", "http://example.invalid/", "p", "@once")
	job.Exectime = time.Now().Add(time.Hour).Unix() // far future: stays in the queue
	if err := q.Push(job); err != nil {
		t.Fatalf("Push: %v", err)
	}
	jobs, err := q.List("GRP")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(jobs) != 1 || jobs[0].Id != "JOB5" {
		t.Fatalf("List = %+v, want one job JOB5", jobs)
	}
	if err := q.Delete("GRP", "JOB5"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	jobs, err = q.List("GRP")
	if err != nil {
		t.Fatalf("List after delete: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("List after delete = %+v, want empty", jobs)
	}
}
