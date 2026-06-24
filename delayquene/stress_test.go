package delayquene

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"
)

// TestRiverStress measures end-to-end timing accuracy and completeness of the
// Postgres/River backend: it schedules N jobs at a common instant and records,
// per job, how late its callback arrives.
//
// Run with:
//   GUA_STRESS=1 GUA_STRESS_N=1000 \
//   GUA_PG_DSN='postgres://postgres:gua@localhost:5433/gua?sslmode=disable' \
//   go test ./delayquene/ -run TestRiverStress -v -timeout 180s
func TestRiverStress(t *testing.T) {
	if os.Getenv("GUA_STRESS") == "" {
		t.Skip("set GUA_STRESS=1 to run the River stress/timing test")
	}
	n := 500
	if v := os.Getenv("GUA_STRESS_N"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			n = parsed
		}
	}
	q := newRiverTestQuene(t) // also skips unless GUA_PG_DSN is set
	if err := q.RegisterGroup("STRESS"); err != nil {
		t.Fatalf("RegisterGroup: %v", err)
	}

	var mu sync.Mutex
	seen := make(map[string]int)
	delays := make([]time.Duration, 0, n)
	done := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()
		var env TriggerEnvelope
		json.NewDecoder(r.Body).Decode(&env)
		mu.Lock()
		seen[env.JobId]++
		if seen[env.JobId] == 1 {
			delays = append(delays, now.Sub(time.Unix(env.PlanTime, 0)))
			if len(delays) == n {
				close(done)
			}
		}
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	execUnix := time.Now().Add(3 * time.Second).Unix() // common due instant
	for i := 0; i < n; i++ {
		job := httpJob("STRESS", "S"+strconv.Itoa(i), srv.URL, "x", "@once")
		job.Exectime = execUnix
		if err := q.Push(job); err != nil {
			t.Fatalf("Push %d: %v", i, err)
		}
	}

	select {
	case <-done:
	case <-time.After(120 * time.Second):
	}

	mu.Lock()
	defer mu.Unlock()
	received := len(delays)
	dups := 0
	for _, c := range seen {
		if c > 1 {
			dups++
		}
	}
	sort.Slice(delays, func(i, j int) bool { return delays[i] < delays[j] })
	pct := func(p float64) time.Duration {
		if len(delays) == 0 {
			return 0
		}
		return delays[int(p*float64(len(delays)-1))]
	}
	t.Logf("river stress N=%d | received=%d missing=%d duplicates=%d", n, received, n-received, dups)
	if received > 0 {
		t.Logf("lateness from scheduled time  p50=%v p95=%v p99=%v max=%v",
			pct(0.50), pct(0.95), pct(0.99), delays[len(delays)-1])
	}
	if received != n {
		t.Errorf("completeness: received %d/%d", received, n)
	}
	if dups != 0 {
		t.Errorf("idempotency: %d jobs fired more than once", dups)
	}
}
