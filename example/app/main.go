// Command demo is a tiny backend for the gua example: it serves the single-page
// UI, schedules jobs in gua on request, and receives gua's delivery webhook —
// streaming "scheduled" / "fired" events to the browser over SSE so you can
// watch a job fire. Pure stdlib — no dependencies.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

const group = "demo"

var (
	guaURL  string
	hookURL string
	sse     = &broker{clients: map[chan string]bool{}}
)

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func main() {
	guaURL = env("GUA_URL", "http://localhost:7777")
	hookURL = env("HOOK_URL", "http://localhost:8080/hook")
	staticDir := env("STATIC_DIR", "static")
	addr := env("LISTEN", ":8080")

	ensureGroup() // best-effort; also retried per schedule

	mux := http.NewServeMux()
	mux.Handle("GET /", http.FileServer(http.Dir(staticDir)))
	mux.HandleFunc("POST /schedule", handleSchedule)
	mux.HandleFunc("POST /hook", handleHook) // gua delivers fired jobs here
	mux.HandleFunc("GET /events", handleEvents)

	log.Printf("gua example demo listening on %s (gua=%s, hook=%s)", addr, guaURL, hookURL)
	log.Fatal(http.ListenAndServe(addr, mux))
}

// handleSchedule registers a one-shot gua job that fires after `delay` seconds
// and delivers `payload` back to our /hook.
func handleSchedule(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Delay   int    `json:"delay"`
		Payload string `json:"payload"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	if in.Delay < 0 {
		in.Delay = 0
	}
	ensureGroup()
	execTime := time.Now().Unix() + int64(in.Delay)
	job := map[string]any{
		"name":             "demo-job",
		"exec_time":        execTime,
		"interval_pattern": "@once",
		"request_url":      "HTTP@" + hookURL,
		"payload":          in.Payload,
	}
	body, _ := json.Marshal(job)
	resp, err := http.Post(guaURL+"/v1/groups/"+group+"/jobs", "application/json", bytes.NewReader(body))
	if err != nil {
		http.Error(w, "schedule failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		http.Error(w, "gua rejected: "+string(rb), http.StatusBadGateway)
		return
	}
	sse.broadcast(map[string]any{"type": "scheduled", "payload": in.Payload, "delay": in.Delay, "at": execTime})
	w.Header().Set("Content-Type", "application/json")
	w.Write(rb) // gua returns the job id
}

// handleHook is the consumer endpoint gua POSTs to when a job fires.
func handleHook(w http.ResponseWriter, r *http.Request) {
	var env struct {
		Payload  string `json:"payload"`
		JobId    string `json:"job_id"`
		ExecTime int64  `json:"exec_time"`
	}
	_ = json.NewDecoder(r.Body).Decode(&env)
	sse.broadcast(map[string]any{"type": "fired", "payload": env.Payload, "job_id": env.JobId, "at": env.ExecTime})
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// handleEvents is the browser's SSE stream.
func handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "stream unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	ch := sse.sub()
	defer sse.unsub(ch)
	fmt.Fprintf(w, "data: %s\n\n", `{"type":"ready"}`)
	flusher.Flush()
	for {
		select {
		case msg := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func ensureGroup() {
	body, _ := json.Marshal(map[string]any{"group_name": group})
	resp, err := http.Post(guaURL+"/v1/groups", "application/json", bytes.NewReader(body))
	if err == nil {
		resp.Body.Close()
	}
}

// broker is a minimal SSE fan-out.
type broker struct {
	mu      sync.Mutex
	clients map[chan string]bool
}

func (b *broker) sub() chan string {
	ch := make(chan string, 16)
	b.mu.Lock()
	b.clients[ch] = true
	b.mu.Unlock()
	return ch
}
func (b *broker) unsub(ch chan string) {
	b.mu.Lock()
	if b.clients[ch] {
		delete(b.clients, ch)
		close(ch)
	}
	b.mu.Unlock()
}
func (b *broker) broadcast(v any) {
	data, _ := json.Marshal(v)
	b.mu.Lock()
	for ch := range b.clients {
		select {
		case ch <- string(data):
		default:
		}
	}
	b.mu.Unlock()
}
