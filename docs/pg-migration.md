# gua: Redis → PostgreSQL (River) migration plan

> **Status:** in progress on branch `pg-store`. Phase 0 done; Phase 1 **core** proven
> (groups + Push→Insert + delivery worker + recurring, all green against real Postgres
> via River). Phase 1 remainder + Phases 2–7 pending.
> **Base:** this repo, branch `harden`. Work is on the `pg-store` branch off `harden`.
>
> **Run the PG-backed tests:** start Postgres and point the tests at it:
> `GUA_PG_DSN='postgres://USER:PASS@HOST:PORT/DB?sslmode=disable' go test ./delayquene/ -run TestRiver -v`
> (without `GUA_PG_DSN` they skip; the Redis/miniredis suite is unaffected.)
> **Decision (2026-06-24):** move gua's backing service from Redis to **PostgreSQL**,
> using **River** (`github.com/riverqueue/river`). Binding to Postgres is accepted.
> Self-contained — can be handed to a separate session.

## Why — delete a whole layer of compensation code

`job` must not be lost (needs ACID durability), and `harden` carries an even
larger hand-rolled reliability layer than the old codebase — built only to work
around Redis having no ACID / ack / failover / lease. On Postgres + River, that
layer is provided by the database and **deleted wholesale**.

### Current state (`harden` / Redis) — **bold = compensation code that goes away**

| Mechanism | Implementation | File |
|---|---|---|
| delay bucket | sorted set `ZADD <bucket> <run_at> <jobId>`, one per server | `bucket.go` |
| due scan | 700ms ticker `ZRANGE` | `quene.go` / `bucket.go` |
| ready queue | `RPUSH` → `BLPOP GUA-READY-JOB` | `worker.go` |
| job body | `guaproto.Job` (payload string) as proto bytes | `job.go` |
| cron | SpecSchedule | `spec.go` / `parser.go` |
| **distributed lock** | `STARTLOCK` / `JOBCHECKLOCK` (`SET NX PX` + Lua CAS) | `lock.go` / `bucket.go` |
| **de-dup fence** | `FENCE-{job}-{run_at}` `SET NX` | `worker.go` |
| **lost-job patch** | `JOB-*-scan` checkpoints + `JobCheck` | `bucket.go` |
| **failover** | `down-server` `LPUSH` + `ZUNIONSTORE` bucket merge | `bucket.go` |
| **slot election + fencing** | `SERVER-N` / `OWN-SERVER-N` + heartbeat CAS + `OnSupersede` | `quene.go` |
| monitoring | `/v1/status`, `/v1/{group}/history` (`GUA-HIST` ZSET TTL), `/ui` | `quene.go` / `history.go` |

River replaces **all bold rows**: SKIP LOCKED dequeue (no fence, no locks, no
per-server buckets), built-in retry/backoff, periodic jobs (cron), a **rescuer**
(stuck-job reaper), and internal **leader election** (over PG advisory locks) for
its own singleton maintenance — so gua stops owning slot election, fencing,
down-server reclaim, JobCheck, and the de-dup fence entirely. Horizontal scaling
becomes "run N stateless instances against one PG"; River is at-least-once.

## Target: River, behind the existing `Quene` interface

gua already has `delayquene.Quene` (used by `cmd.go` / `grpc.go` / `httpv1`).
That **is** the seam — add a River-backed implementation, toggle by config,
keep the Redis implementation until River is proven.

**Mapping**

| gua | River |
|---|---|
| `AddJob` | `river.Insert(JobArgs{group, request_url, payload}, &InsertOpts{ScheduledAt: run_at})` |
| delivery (resty `POST` / gRPC `OnJobTrigger`) | the body of a River `Worker.Work()` |
| cron / recurring | River **PeriodicJobs** (or insert next with `ScheduledAt`) |
| retry / backoff / `failed` | River built-in — **NEW behavior** (current gua does not retry a failed callback; confirm enabling) |
| crash recovery | River **rescuer** (no hand-rolled reaper) |
| `Stats` (`ready_queue_depth`) | River queue stats; **drop the `servers[]` / slot section** |
| `History` | River completed-job records (retention) or a small `executions` table |
| job id | River assigns its own `int64`; gua's external `job_id` lives in `JobArgs` (+ River **unique** key). `List`/`Delete`/`Pause` query by it |

**Integration point to design (Phase 2):** gua's **group-level pause / `job/clear`**
has no native River equivalent → implement via cancel + re-insert, or a paused
queue. This is the one real seam River adoption introduces.

## What stays (out of scope)

- delivery types **`HTTP@` / `GRPC@`** only (lua/remote already removed on `harden`)
  and the worker dispatch (resty POST envelope / gRPC `OnJobTrigger`).
- `guaproto.Job` model; payload as proto bytes.
- external API: `GuaAdmin` gRPC + HTTP REST (`AddJob`, …) semantics.
- cron (SpecSchedule) semantics.
- HTTP client: already resty (`internal/httpclient`).

## Phases

### Phase 0 — gate + baseline ✅ done
Branch `pg-store` off `harden`; docker Postgres up; River + driver chosen
(`riverpgxv5`). `TestRiverSmoke` proves the migrate→insert→work stack against PG.

### Phase 1 — River-backed `Quene` (Redis impl untouched)
- [x] **core**: `riverquene.go` — `NewRiver` (migrate + `gua_groups` table + delivery
  worker + Start); `Push` → `Insert(ScheduledAt)`; delivery worker reuses the
  HTTP POST / gRPC `OnJobTrigger` envelope; recurring self-reschedules via
  cron `Next()`. Tests `TestRiverHTTPDelivery / TestRiverRecurring / TestRiverGroups`
  green against PG. Redis path untouched (miniredis suite still green).
- [x] config toggle: `BACKEND=river` + `PG_DSN` → `startRiver` (`cmdriver.go`);
  shared `buildRouter`; `env.river.example`. Redis `start()` untouched.
- [ ] (optional) cron via River **PeriodicJobs** (currently re-insert on each fire —
  works; periodic is cleaner for fixed schedules).

### Phase 2 — map the rest of the `Quene` surface ✅ done
- [x] `gua_jobs` table = source of truth for job definitions (active/paused),
  River holds the scheduled occurrences.
- [x] `List / Delete / Remove / Edit / Pause / Active / Stats` on `gua_jobs`
  (+ cancel the not-yet-run River occurrence on pause/delete). The worker
  re-checks `active` before delivering and before rescheduling, so pause is
  robust; `@once` jobs are dropped after firing (matches Redis). group-level
  `job/clear` and `job/delete/{name}` work via List+Delete.
  Tests: `TestRiverListDelete`, `TestRiverPause` (green against PG).
- [ ] `History` → Phase 4 (PG executions table); currently returns empty.

### Phase 3 — tests
Re-point the integration tests (`newTestQuene`) to the River backend
(testcontainers / local PG). Add: crash-mid-process → rescuer re-runs (the
headline "job not lost"); multi-instance SKIP LOCKED no double-run;
retry/backoff; cron next-run.

### Phase 4 — monitoring on PG
`history` from River completed jobs (or `executions` table); `status` from River
stats with the slot section removed; adjust `/ui` (or surface River's own web UI).

### Phase 5 — delete compensation code + drop Redis  ← **gate: only after River is proven**
Delete the Redis `Quene` impl + bucket / lock / scan / JobCheck / down-server /
`SERVER-N` fencing / fence. Remove `redigo` from `go.mod` + vendor. Collapse the
4 Redis env groups into one PG DSN.

### Phase 6 — data migration + cutover
One-shot script: Redis in-flight jobs → `river.Insert`. Cutover + rollback
runbook (keep the Redis branch).

### Phase 7 — stress + acceptance + docs
Re-run the stress harness (timing now bound by River + LISTEN, not the 700ms
ticker — measure). Redraw the pipeline and **cluster** diagrams (the cluster one
simplifies a lot — no slots/fencing). Update ARCHITECTURE.md / EVAL.md.

## Acceptance

- [ ] **job not lost**: kill a worker mid-run → River rescuer re-runs it (kill-mid-process test).
- [ ] delay / cron fire at the right time.
- [ ] multi-worker concurrent **dequeue** never double-runs (SKIP LOCKED).
- [ ] retry / backoff / `failed` terminal correct. **(NEW behavior — confirm intended.)**
- [ ] **delivery is still at-least-once**: a worker crashing after delivering but
      before commit re-delivers → consumers must stay idempotent (PG fixes
      job-loss, not duplicate delivery; not exactly-once).
- [ ] no longer depends on redis; `go.mod` / vendor drop `redigo`.
- [ ] compensation code deleted (scan / jobcheck / down-server / locks / fence / SERVER-N).
- [ ] external API + delivery types (HTTP / GRPC) unchanged; monitoring `status` adjusted.
