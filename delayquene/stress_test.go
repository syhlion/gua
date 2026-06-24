package delayquene

import (
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"
)

// TestStress measures end-to-end timing accuracy and completeness: it schedules
// N jobs all due at the same instant and records, per job, the delay between the
// planned time and when its callback actually arrives.
//
// Run with: GUA_STRESS=1 GUA_STRESS_N=1000 go test ./delayquene/ -run TestStress -v -timeout 120s
func TestStress(t *testing.T) {
	if os.Getenv("GUA_STRESS") == "" {
		t.Skip("set GUA_STRESS=1 to run the stress/timing test")
	}
	n := 500
	if v := os.Getenv("GUA_STRESS_N"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			n = parsed
		}
	}

	q := newTestQuene(t)
	if err := q.RegisterGroup("STRESS"); err != nil {
		t.Fatalf("RegisterGroup: %v", err)
	}

	var mu sync.Mutex
	seen := make(map[string]int) // job_id -> delivery count (detect duplicates)
	delays := make([]time.Duration, 0, n)
	plan := make(map[string]time.Time)
	done := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()
		jid := r.URL.Query().Get("jid")
		mu.Lock()
		seen[jid]++
		if seen[jid] == 1 {
			delays = append(delays, now.Sub(plan[jid]))
			if len(delays) == n {
				close(done)
			}
		}
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	execUnix := time.Now().Add(2 * time.Second).Unix()
	// the scheduler honours whole-second exec times, so measure lateness from
	// the scheduled second boundary, not a sub-second wall clock.
	planBoundary := time.Unix(execUnix, 0)
	for i := 0; i < n; i++ {
		jid := "J" + strconv.Itoa(i)
		mu.Lock()
		plan[jid] = planBoundary
		mu.Unlock()
		job := httpJob("STRESS", jid, srv.URL+"?jid="+jid, "p", "@once")
		job.Exectime = execUnix
		if err := q.Push(job); err != nil {
			t.Fatalf("Push %s: %v", jid, err)
		}
	}

	select {
	case <-done:
	case <-time.After(60 * time.Second):
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
		idx := int(p * float64(len(delays)-1))
		return delays[idx]
	}

	t.Logf("stress N=%d | received=%d missing=%d duplicates=%d", n, received, n-received, dups)
	if received > 0 {
		t.Logf("timing error  p50=%v  p95=%v  p99=%v  max=%v",
			pct(0.50), pct(0.95), pct(0.99), delays[len(delays)-1])
	}

	// Acceptance gate: no missed fires, no duplicate fires.
	if received != n {
		t.Errorf("completeness: received %d/%d (missing %d)", received, n, n-received)
	}
	if dups != 0 {
		t.Errorf("idempotency: %d jobs fired more than once", dups)
	}
}
