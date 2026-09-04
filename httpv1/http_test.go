package httpv1

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/syhlion/gua/delayquene"
	guaproto "github.com/syhlion/gua/proto"
)

// fakeQuene records the last call and returns a canned error. It lets the
// handlers be tested without Postgres: request parsing, path values, and the
// error → status mapping.
type fakeQuene struct {
	err    error
	calls  []string
	pushed *guaproto.Job
	jobs   []*guaproto.Job
	active int64
}

func (f *fakeQuene) rec(format string, a ...any) {
	f.calls = append(f.calls, fmt.Sprintf(format, a...))
}

func (f *fakeQuene) GenerateUID() string { return "GENERATED" }
func (f *fakeQuene) Push(job *guaproto.Job) error {
	f.pushed = job
	f.rec("Push %s/%s", job.GroupName, job.Id)
	return f.err
}
func (f *fakeQuene) Edit(g, j, u, p string) error {
	f.rec("Edit %s/%s %s %s", g, j, u, p)
	return f.err
}
func (f *fakeQuene) Active(g, j string, e int64) error {
	f.active = e
	f.rec("Active %s/%s", g, j)
	return f.err
}
func (f *fakeQuene) Pause(g, j string) error  { f.rec("Pause %s/%s", g, j); return f.err }
func (f *fakeQuene) Delete(g, j string) error { f.rec("Delete %s/%s", g, j); return f.err }
func (f *fakeQuene) DeleteJobs(g, n string) (int, error) {
	f.rec("DeleteJobs %s name=%q", g, n)
	return 3, f.err
}
func (f *fakeQuene) List(g string) ([]*guaproto.Job, error) {
	f.rec("List %s", g)
	return f.jobs, f.err
}
func (f *fakeQuene) RegisterGroup(g string) error       { f.rec("RegisterGroup %s", g); return f.err }
func (f *fakeQuene) RemoveGroup(g string) error         { f.rec("RemoveGroup %s", g); return f.err }
func (f *fakeQuene) ExistsGroup(g string) (bool, error) { return true, f.err }
func (f *fakeQuene) QueryGroups() ([]string, error)     { return []string{"A", "B"}, f.err }
func (f *fakeQuene) GroupInfo(g string) (string, error) { f.rec("GroupInfo %s", g); return g, f.err }
func (f *fakeQuene) Stats() (*delayquene.Stats, error) {
	return &delayquene.Stats{Now: 1, ReadyQueueDepth: 2, JobsActive: 3}, f.err
}
func (f *fakeQuene) History(g string, limit int) ([]*delayquene.HistoryEntry, error) {
	f.rec("History %s limit=%d", g, limit)
	return []*delayquene.HistoryEntry{}, f.err
}
func (f *fakeQuene) Ping(ctx context.Context) error { return f.err }
func (f *fakeQuene) Close()                         {}

type resp struct {
	code int
	body map[string]json.RawMessage
}

// call invokes a handler with path values set the way the ServeMux would.
func call(t *testing.T, h http.HandlerFunc, method, body string, pv map[string]string, query string) resp {
	t.Helper()
	req := httptest.NewRequest(method, "/x"+query, strings.NewReader(body))
	for k, v := range pv {
		req.SetPathValue(k, v)
	}
	w := httptest.NewRecorder()
	h(w, req)
	out := resp{code: w.Code}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content-type = %q", ct)
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out.body); err != nil {
		t.Fatalf("response is not a JSON object: %s", w.Body.String())
	}
	return out
}

func (r resp) success(t *testing.T) string {
	t.Helper()
	if _, ok := r.body["success"]; !ok {
		t.Fatalf("expected {\"success\":...}, got %v", r.body)
	}
	return string(r.body["success"])
}

func (r resp) errMsg(t *testing.T) string {
	t.Helper()
	raw, ok := r.body["error"]
	if !ok {
		t.Fatalf("expected {\"error\":...}, got %v", r.body)
	}
	var s string
	_ = json.Unmarshal(raw, &s)
	return s
}

// Every sentinel error lands on its HTTP status; unknown errors are hidden.
func TestErrorMapping(t *testing.T) {
	cases := []struct {
		err  error
		code int
		msg  string
	}{
		{fmt.Errorf("%w: bad", delayquene.ErrInvalid), http.StatusBadRequest, "invalid argument: bad"},
		{fmt.Errorf("job %w", delayquene.ErrNotFound), http.StatusNotFound, "job not found"},
		{fmt.Errorf("group %w", delayquene.ErrDuplicate), http.StatusConflict, "group already exists"},
		{errors.New("pg: connection refused"), http.StatusInternalServerError, "internal error"},
	}
	for _, c := range cases {
		q := &fakeQuene{err: c.err}
		r := call(t, PauseJob(q), "POST", "", map[string]string{"group": "G", "job": "J"}, "")
		if r.code != c.code || r.errMsg(t) != c.msg {
			t.Errorf("%v: got %d %q, want %d %q", c.err, r.code, r.errMsg(t), c.code, c.msg)
		}
	}
}

func TestAddJob(t *testing.T) {
	q := &fakeQuene{}
	body := `{"name":"n","exec_time":10,"interval_pattern":"@once","request_url":"HTTP@http://x","payload":"p","timeout":5,"memo":"m"}`
	r := call(t, AddJob(q), "POST", body, map[string]string{"group": "G"}, "")
	if r.code != 200 || r.success(t) != `"GENERATED"` {
		t.Fatalf("got %d %v", r.code, r.body)
	}
	j := q.pushed
	if j.GroupName != "G" || j.Id != "GENERATED" || j.Name != "n" || j.Exectime != 10 || j.Timeout != 5 ||
		j.IntervalPattern != "@once" || j.RequestUrl != "HTTP@http://x" || j.Payload != "p" || j.Memo != "m" || !j.Active {
		t.Fatalf("pushed job = %+v", j)
	}

	// explicit id is passed through; validation errors come back as 400
	q = &fakeQuene{err: fmt.Errorf("%w: interval_pattern", delayquene.ErrInvalid)}
	r = call(t, AddJob(q), "POST", `{"job_id":"MINE","name":"n"}`, map[string]string{"group": "G"}, "")
	if r.code != 400 || q.pushed.Id != "MINE" {
		t.Fatalf("got %d, pushed id %q", r.code, q.pushed.Id)
	}

	// malformed JSON never reaches the queue
	q = &fakeQuene{}
	r = call(t, AddJob(q), "POST", `{"name":`, map[string]string{"group": "G"}, "")
	if r.code != 400 || q.pushed != nil || !strings.HasPrefix(r.errMsg(t), "invalid json") {
		t.Fatalf("got %d %v", r.code, r.body)
	}

	// oversized body is refused
	big := `{"payload":"` + strings.Repeat("x", maxBodyBytes) + `"}`
	r = call(t, AddJob(q), "POST", big, map[string]string{"group": "G"}, "")
	if r.code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized body: got %d", r.code)
	}
}

func TestRegisterGroup(t *testing.T) {
	q := &fakeQuene{}
	r := call(t, RegisterGroup(q), "POST", `{"group_name":"G1"}`, nil, "")
	if r.code != 200 || r.success(t) != `"G1"` || q.calls[0] != "RegisterGroup G1" {
		t.Fatalf("got %d %v %v", r.code, r.body, q.calls)
	}
	q = &fakeQuene{err: fmt.Errorf("group %w", delayquene.ErrDuplicate)}
	if r := call(t, RegisterGroup(q), "POST", `{"group_name":"G1"}`, nil, ""); r.code != 409 {
		t.Fatalf("duplicate: got %d", r.code)
	}
}

func TestEditJob(t *testing.T) {
	q := &fakeQuene{}
	r := call(t, EditJob(q), "PATCH", `{"payload":"p"}`, map[string]string{"group": "G", "job": "J"}, "")
	if r.code != 400 || len(q.calls) != 0 {
		t.Fatalf("missing request_url should be 400 before touching the queue: %d %v", r.code, q.calls)
	}
	r = call(t, EditJob(q), "PATCH", `{"request_url":"HTTP@http://y","payload":"p"}`, map[string]string{"group": "G", "job": "J"}, "")
	if r.code != 200 || q.calls[0] != "Edit G/J HTTP@http://y p" {
		t.Fatalf("got %d %v", r.code, q.calls)
	}
}

func TestActiveJob(t *testing.T) {
	q := &fakeQuene{}
	// empty body = now (0)
	r := call(t, ActiveJob(q), "POST", "", map[string]string{"group": "G", "job": "J"}, "")
	if r.code != 200 || q.active != 0 || q.calls[0] != "Active G/J" {
		t.Fatalf("got %d active=%d %v", r.code, q.active, q.calls)
	}
	r = call(t, ActiveJob(q), "POST", `{"exec_time":42}`, map[string]string{"group": "G", "job": "J"}, "")
	if r.code != 200 || q.active != 42 {
		t.Fatalf("got %d active=%d", r.code, q.active)
	}
}

func TestDeleteJobs(t *testing.T) {
	q := &fakeQuene{}
	r := call(t, DeleteJobs(q), "DELETE", "", map[string]string{"group": "G"}, "?name=nightly")
	if r.code != 200 || r.success(t) != "3" || q.calls[0] != `DeleteJobs G name="nightly"` {
		t.Fatalf("got %d %v %v", r.code, r.body, q.calls)
	}
	r = call(t, DeleteJobs(q), "DELETE", "", map[string]string{"group": "G"}, "")
	if q.calls[1] != `DeleteJobs G name=""` {
		t.Fatalf("calls = %v", q.calls)
	}
	q = &fakeQuene{err: fmt.Errorf("job %w", delayquene.ErrNotFound)}
	if r := call(t, DeleteJob(q), "DELETE", "", map[string]string{"group": "G", "job": "J"}, ""); r.code != 404 {
		t.Fatalf("delete unknown job: got %d", r.code)
	}
}

func TestReadEndpoints(t *testing.T) {
	q := &fakeQuene{jobs: []*guaproto.Job{{Id: "J1", GroupName: "G", Name: "n", Exectime: 7, Timeout: 3, Active: true, RequestUrl: "HTTP@x"}}}

	r := call(t, GetJobList(q), "GET", "", map[string]string{"group": "G"}, "")
	var list []ResponseJobList
	if err := json.Unmarshal([]byte(r.success(t)), &list); err != nil || len(list) != 1 ||
		list[0].Id != "J1" || list[0].Exectime != 7 || list[0].Timeout != 3 || !list[0].Active {
		t.Fatalf("job list = %s (%v)", r.success(t), err)
	}

	r = call(t, GetGroupList(q), "GET", "", nil, "")
	if r.success(t) != `["A","B"]` {
		t.Fatalf("group list = %s", r.success(t))
	}

	r = call(t, GroupInfo(q), "GET", "", map[string]string{"group": "G"}, "")
	if r.code != 200 || r.success(t) != `"G"` {
		t.Fatalf("group info = %d %s", r.code, r.success(t))
	}

	r = call(t, Status(q), "GET", "", nil, "")
	var s delayquene.Stats
	if err := json.Unmarshal([]byte(r.success(t)), &s); err != nil || s.ReadyQueueDepth != 2 || s.JobsActive != 3 {
		t.Fatalf("status = %s", r.success(t))
	}

	r = call(t, History(q), "GET", "", map[string]string{"group": "G"}, "?limit=5")
	if r.code != 200 || q.calls[len(q.calls)-1] != "History G limit=5" {
		t.Fatalf("history: %d %v", r.code, q.calls)
	}
	r = call(t, History(q), "GET", "", map[string]string{"group": "G"}, "?limit=abc")
	if r.code != 400 {
		t.Fatalf("bad limit: got %d", r.code)
	}

	r = call(t, Version("v9"), "GET", "", nil, "")
	if r.success(t) != `"v9"` {
		t.Fatalf("version = %s", r.success(t))
	}
}

// The wire format is stable: {"success": x} on 200, {"error": x} otherwise.
func TestWriteJSONEnvelope(t *testing.T) {
	w := httptest.NewRecorder()
	WriteJSON(w, map[string]int{"a": 1}, http.StatusOK)
	if got := strings.TrimSpace(w.Body.String()); got != `{"success":{"a":1}}` {
		t.Fatalf("200 body = %s", got)
	}
	w = httptest.NewRecorder()
	WriteJSON(w, "nope", http.StatusNotFound)
	if got := strings.TrimSpace(w.Body.String()); got != `{"error":"nope"}` || w.Code != 404 {
		t.Fatalf("404 body = %s (%d)", got, w.Code)
	}
}

// readJSON tolerates an empty body and rejects garbage.
func TestReadJSON(t *testing.T) {
	var dst struct {
		A int `json:"a"`
	}
	req := httptest.NewRequest("POST", "/", bytes.NewReader(nil))
	if !readJSON(httptest.NewRecorder(), req, &dst) {
		t.Fatal("empty body should be accepted")
	}
	req = httptest.NewRequest("POST", "/", io.NopCloser(strings.NewReader(`{"a":2}`)))
	if !readJSON(httptest.NewRecorder(), req, &dst) || dst.A != 2 {
		t.Fatalf("decoded = %+v", dst)
	}
}
