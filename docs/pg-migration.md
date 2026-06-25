# gua: Redis → PostgreSQL (River) migration plan

> 🌐 **English** · [繁體中文](pg-migration.zh-TW.md)

> **Status:** ✅ **done & merged to `master`** (the Redis line lives on `harden`). All
> phases complete; this doc is kept as the migration record / rationale. Postgres +
> River is now the only backing store — the Redis impl and its compensation layer were
> deleted (Phase 5).
>
> **Run the PG-backed tests:** start Postgres and point the tests at it:
> `GUA_PG_DSN='postgres://USER:PASS@HOST:PORT/DB?sslmode=disable' go test ./delayquene/ -run TestRiver -v`
> (without `GUA_PG_DSN` they skip.)
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
- [x] (optional) cron via River **PeriodicJobs** — **evaluated, declined.** River's
  PeriodicJobs keep their schedule state **in-memory on the elected leader** and are
  non-durable: per River's own docs, "anytime a process quits or a new leader is
  elected, the whole process starts over." gua instead inserts each next occurrence
  as a **durable `river_job` row** (`ScheduledAt`), which survives restarts/leader
  changes — strictly better for this migration's durability goal — and fits gua's
  **dynamic, per-job, pausable, DB-sourced** model (jobs are added at runtime and
  gated by `gua_jobs.active`), which a static leader-held periodic registry serves
  poorly. Decision: keep the self-reschedule-as-durable-row approach.

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

### Phase 3 — tests ✅ (mostly)
- [x] River-backed tests against real PG: `TestRiverHTTPDelivery / Recurring /
  ListDelete / Pause / Groups / History`.
- [x] **no double-run**: `TestRiverNoDuplicate` (30 same-time jobs each fire
  exactly once via SKIP LOCKED).
- [x] **retry/backoff**: `TestRiverRetry` (fail once → River retries → succeed).
- [x] crash-mid-process → **River rescuer** re-runs: `TestRiverRescuer`
  (`delayquene/rescuer_test.go`) drives a stand-in worker that never returns
  (row stuck in `running`), then asserts the rescuer re-runs it. Squeezes the
  rescue window via `RescueStuckJobsAfter`/`JobTimeout` so it doesn't wait the
  1h default; gated by `GUA_PG_DSN`, runs in CI against `postgres:16`.

### Phase 4 — monitoring on PG ✅ done
- [x] `gua_executions` table; the worker records every attempt (success/fail,
  message/error, timings, host) and prunes to `GUA_HISTORY_TTL` (default 5 days,
  0 disables). `History` reads it newest-first.
- [x] `Stats`: `ready_queue_depth` = count of pending River occurrences; the
  slot/`servers[]` section is empty (no slots on PG).
- [x] `/ui`, `/v1/status`, `/v1/{group}/history` work unchanged over the River
  backend (same handlers, same `Quene` interface).

### Phase 5 — delete compensation code + drop Redis ✅ done
- [x] deleted the Redis `Quene` impl: `quene.go` / `bucket.go` / `lock.go` /
  `job.go` / `worker.go` / `history.go` (Redis) / `help.go` (RedisScan), plus the
  miniredis tests. Shared types (Quene interface, Stats/HistoryEntry/
  TriggerEnvelope, UrlRe) moved to `types.go`; cron (`parser.go`/`spec.go`) kept.
- [x] deleted the `migrate` package (Redis dump/import) + its HTTP handlers.
- [x] `cmd.go` collapsed to the River path; `config.go` (4 Redis env groups)
  removed; one `PG_DSN` env. `BACKEND` toggle no longer needed — `start()` is River.
- [x] `go.mod` / vendor drop `redigo`, `miniredis`, `gopher-lua`. Net −2,956 lines.
- [x] verified: build/vet green, River suite green, PG-only binary exercised
  end-to-end (register → add → fire → history).

### Phase 6 — data migration + cutover — N/A for fresh deploy
A fresh Postgres deployment needs no data migration. A one-shot Redis→PG import
(read the Redis branch's buckets/jobs → `river.Insert` / `gua_jobs`) is only
needed for an *in-place* cutover of a running Redis instance — write it then if
that scenario arises.

### Phase 7 — stress + docs ✅ done
- [x] `TestRiverStress` (gated by `GUA_STRESS`+`GUA_PG_DSN`): N same-instant jobs,
  measures lateness p50/p95/p99/max + completeness. N=500/2000 → 0 missed, 0
  duplicate; lateness ~3–6s (River scheduler promotion latency — see EVAL.md).
- [x] diagrams redrawn for Postgres (architecture / pipeline / cluster — the
  cluster one drops slots/fencing entirely).
- [x] README / ARCHITECTURE / MONITORING / EVAL / CHANGELOG rewritten for PG.

## Acceptance

- [x] **job not lost**: kill a worker mid-run → River rescuer re-runs it
  (`TestRiverRescuer`, kill-mid-process test).
- [ ] delay / cron fire at the right time.
- [ ] multi-worker concurrent **dequeue** never double-runs (SKIP LOCKED).
- [ ] retry / backoff / `failed` terminal correct. **(NEW behavior — confirm intended.)**
- [ ] **delivery is still at-least-once**: a worker crashing after delivering but
      before commit re-delivers → consumers must stay idempotent (PG fixes
      job-loss, not duplicate delivery; not exactly-once).
- [ ] no longer depends on redis; `go.mod` / vendor drop `redigo`.
- [ ] compensation code deleted (scan / jobcheck / down-server / locks / fence / SERVER-N).
- [ ] external API + delivery types (HTTP / GRPC) unchanged; monitoring `status` adjusted.
