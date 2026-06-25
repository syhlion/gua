package main

// End-to-end tests that drive gua through its PUBLIC surfaces — the real HTTP
// REST router (buildRouter) and the real gRPC GuaAdmin server — against a live
// Postgres, and assert that a registered job is actually delivered to a
// consumer over BOTH transports (HTTP POST envelope and gRPC OnJobTrigger).
//
// Unlike the delayquene package tests (which call the Quene interface directly),
// these exercise request parsing/validation/routing and the gRPC adapter, plus
// the outbound gRPC delivery path that nothing else covers.
//
// Set GUA_PG_DSN to run. They use an isolated `gua_e2e` database so they never
// race the delayquene package tests, which truncate/drop the shared gua_* tables
// (go test runs packages in parallel).

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/syhlion/gua/delayquene"
	"github.com/syhlion/gua/httpv1"
	guaproto "github.com/syhlion/gua/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// withDB rewrites a Postgres DSN to point at a different database name.
func withDB(dsn, db string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", err
	}
	u.Path = "/" + db
	return u.String(), nil
}

// newE2EQuene spins up a Quene on an isolated, freshly-created gua_e2e database.
func newE2EQuene(t *testing.T) delayquene.Quene {
	t.Helper()
	base := os.Getenv("GUA_PG_DSN")
	if base == "" {
		t.Skip("set GUA_PG_DSN to run the end-to-end API tests")
	}
	ctx := context.Background()
	const e2eDB = "gua_e2e"

	admin, err := pgxpool.New(ctx, base)
	if err != nil {
		t.Fatalf("connect base dsn: %v", err)
	}
	if _, err := admin.Exec(ctx, "DROP DATABASE IF EXISTS "+e2eDB+" WITH (FORCE)"); err != nil {
		admin.Close()
		t.Fatalf("drop e2e db: %v", err)
	}
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+e2eDB); err != nil {
		admin.Close()
		t.Fatalf("create e2e db: %v", err)
	}
	admin.Close()

	dsn, err := withDB(base, e2eDB)
	if err != nil {
		t.Fatalf("rewrite dsn: %v", err)
	}
	q, err := delayquene.NewRiver(&delayquene.RiverConfig{
		DSN: dsn, MachineHost: "e2e", MachineIp: "127.0.0.1", MachineMac: "e2e",
		HistoryTTL: 3600,
		Logger:     discardLogger(),
	})
	if err != nil {
		t.Fatalf("NewRiver: %v", err)
	}
	t.Cleanup(func() {
		q.Close()
		if a, err := pgxpool.New(ctx, base); err == nil {
			_, _ = a.Exec(ctx, "DROP DATABASE IF EXISTS "+e2eDB+" WITH (FORCE)")
			a.Close()
		}
	})
	return q
}

// captureCallback is a gRPC GuaCallback consumer that records the trigger it
// receives — the delivery target for the gRPC path.
type captureCallback struct {
	guaproto.UnimplementedGuaCallbackServer
	ch chan *guaproto.JobTrigger
}

func (c *captureCallback) OnJobTrigger(ctx context.Context, in *guaproto.JobTrigger) (*guaproto.JobResult, error) {
	select {
	case c.ch <- in:
	default:
	}
	return &guaproto.JobResult{Success: true, Message: "ok"}, nil
}

func mustPostJSON(t *testing.T, target string, body any) {
	t.Helper()
	b, _ := json.Marshal(body)
	resp, err := http.Post(target, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST %s: %v", target, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		rb, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST %s -> %d: %s", target, resp.StatusCode, rb)
	}
}

func TestE2E_DeliveryThroughPublicAPIs(t *testing.T) {
	q := newE2EQuene(t)
	httpv1.SetLogger(discardLogger())

	// The real REST router used by the binary.
	rest := httptest.NewServer(buildRouter(q))
	defer rest.Close()

	// The real gRPC admin (GuaAdmin) CRUD surface.
	adminLis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	adminSrv := grpc.NewServer()
	guaproto.RegisterGuaAdminServer(adminSrv, &GuaAdmin{quene: q})
	go func() { _ = adminSrv.Serve(adminLis) }()
	defer adminSrv.Stop()

	// readiness probe should report ready against a live DB.
	t.Run("readyz reports ready", func(t *testing.T) {
		resp, err := http.Get(rest.URL + "/readyz")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("/readyz = %d, want 200", resp.StatusCode)
		}
	})

	t.Run("REST register+add -> HTTP delivery", func(t *testing.T) {
		got := make(chan delayquene.TriggerEnvelope, 1)
		consumer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var env delayquene.TriggerEnvelope
			_ = json.NewDecoder(r.Body).Decode(&env)
			select {
			case got <- env:
			default:
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer consumer.Close()

		mustPostJSON(t, rest.URL+"/v1/register/group", map[string]any{"group_name": "E2EHTTP"})
		mustPostJSON(t, rest.URL+"/v1/add/job", map[string]any{
			"group_name":       "E2EHTTP",
			"name":             "e2e-http-job",
			"exec_time":        time.Now().Unix(),
			"interval_pattern": "@once",
			"request_url":      "HTTP@" + consumer.URL,
			"payload":          "hello-http",
		})

		select {
		case env := <-got:
			if env.GroupName != "E2EHTTP" || env.Payload != "hello-http" {
				t.Fatalf("unexpected envelope: %+v", env)
			}
			if env.IdempotencyKey == "" {
				t.Fatalf("missing idempotency_key: %+v", env)
			}
		case <-time.After(25 * time.Second):
			t.Fatal("no HTTP delivery within 25s")
		}
	})

	t.Run("gRPC register+add -> gRPC delivery", func(t *testing.T) {
		// gRPC GuaCallback consumer = the delivery target.
		cbLis, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		got := make(chan *guaproto.JobTrigger, 1)
		cbSrv := grpc.NewServer()
		guaproto.RegisterGuaCallbackServer(cbSrv, &captureCallback{ch: got})
		go func() { _ = cbSrv.Serve(cbLis) }()
		defer cbSrv.Stop()

		conn, err := grpc.NewClient(adminLis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		admin := guaproto.NewGuaAdminClient(conn)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := admin.RegisterGroup(ctx, &guaproto.GroupRequest{GroupName: "E2EGRPC"}); err != nil {
			t.Fatalf("gRPC RegisterGroup: %v", err)
		}
		if _, err := admin.AddJob(ctx, &guaproto.AddJobRequest{
			GroupName:       "E2EGRPC",
			Name:            "e2e-grpc-job",
			ExecTime:        time.Now().Unix(),
			IntervalPattern: "@once",
			Delivery:        guaproto.DeliveryType_GRPC,
			Target:          cbLis.Addr().String(),
			Payload:         "hello-grpc",
		}); err != nil {
			t.Fatalf("gRPC AddJob: %v", err)
		}

		select {
		case trig := <-got:
			if trig.GroupName != "E2EGRPC" || trig.Payload != "hello-grpc" {
				t.Fatalf("unexpected trigger: %+v", trig)
			}
			if trig.IdempotencyKey == "" {
				t.Fatalf("missing idempotency_key: %+v", trig)
			}
		case <-time.After(25 * time.Second):
			t.Fatal("no gRPC delivery within 25s")
		}
	})
}
