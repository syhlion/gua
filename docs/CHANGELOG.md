[unrelease]

[v4.0.0]

> **Breaking**: HTTP REST API redesigned to a clean resource-oriented shape under
> `/v1` (groups/jobs as nested resources). gRPC `GuaAdmin` contract unchanged. No
> external consumers yet — hard cutover, no aliases.

[Changed / API]

* **REST API redesigned to a clean resource-oriented shape** and moved off
  `gorilla/mux` to the stdlib `net/http.ServeMux` (Go 1.22+ method routing; one
  fewer dependency). Groups and jobs are now nested resources with the
  identifier in the path and the HTTP verb carrying the action — e.g.
  `POST /v1/groups`, `POST /v1/groups/{group}/jobs`,
  `PATCH /v1/groups/{group}/jobs/{job}`,
  `POST /v1/groups/{group}/jobs/{job}/pause|activate`,
  `DELETE /v1/groups/{group}/jobs/{job}`,
  `DELETE /v1/groups/{group}/jobs?name=` (clear all / by name),
  `GET /v1/groups/{group}/history`. Replaces the old verb-in-path routes
  (`/v1/add/job`, `/v1/register/group`, `/v1/{group}/job/list`, …). **Breaking**;
  the gRPC `GuaAdmin` contract is unchanged.

[Added]

* `TestRiverRescuer`: crash-mid-process acceptance test — a worker that dies with
  its job stuck in `running` is re-run by River's rescuer. Closes the last open
  "job not lost" acceptance item from the Postgres migration. Gated by
  `GUA_PG_DSN`; runs in CI against `postgres:16`.

[Changed / hardened]

* gRPC delivery now **reuses a pooled `*grpc.ClientConn` per target** instead of
  dialling and tearing down a connection on every trigger. Recurring jobs hitting
  the same consumer no longer pay a TCP+HTTP/2 handshake each fire. Also replaces
  the deprecated `grpc.Dial`/`grpc.WithInsecure` with `grpc.NewClient`/
  `insecure.NewCredentials`, and adds client + server keepalive so pooled idle
  connections survive NAT/LB paths and dead peers are detected.

[Docs]

* recorded the decision to keep durable self-rescheduling (next occurrence as a
  `river_job` row) rather than adopt River **PeriodicJobs**, whose schedule state
  is in-memory/leader-only and non-durable — see `docs/pg-migration.md`.

[v3.2.0]

[Added]

* health/readiness probes: `GET /healthz` (liveness, no DB) and `GET /readyz`
  (200 when Postgres is reachable, 503 otherwise) for Kubernetes probes.
* end-to-end tests driving the real REST router + gRPC GuaAdmin against a live
  Postgres, asserting delivery over both HTTP and gRPC transports (the gRPC
  delivery path was previously untested). Run an isolated `gua_e2e` database.

[Changed / hardened]

* graceful shutdown: on SIGINT/SIGTERM the HTTP and gRPC servers now drain
  in-flight requests (`server.Shutdown` + `grpcServer.GracefulStop`) before the
  River workers/pool close.
* HTTP server gains `WriteTimeout`, `IdleTimeout`, and `ReadHeaderTimeout`
  (slowloris guard) — previously only `ReadTimeout` was set.

[Security]

* bump grpc 1.53.0 → 1.81.1 and protobuf 1.28.1 → 1.36.11, clearing 3
  Dependabot alerts on the gRPC delivery/admin path (critical: authz bypass via
  missing leading slash in `:path`; high: HTTP/2 Rapid Reset; medium: protojson
  infinite loop).

[CI]

* modernized the Drone pipeline: Go 1.25 + a `postgres:16` service so the
  River/Postgres tests actually run (the old step only did `go build` with no
  DB); docker push scoped to master + tags; removed the draft github-release
  step (releases are cut manually with `gh`). Deleted the dead `.gitlab-ci.yml`.

[v3.1.0]

[Added]

* `idempotency_key` in the delivery envelope (HTTP body + gRPC JobTrigger) = River's
  occurrence id, stable across retries/redeliveries of the same firing — consumers
  dedupe on it (delivery is at-least-once).
* docs: Docker Compose quick start; idempotency + `@every`-drift-vs-cron notes;
  Traditional Chinese (zh-TW) versions of the docs.

[Fixed]

* recurring chain no longer breaks on a transient insert error — `insertNext`
  failure now retries the whole `Work()` via River.

[Changed]

* Postgres-only cleanup: removed the old Redis `env.example` (`env.river.example`
  renamed to `env.example`, `BACKEND` toggle dropped); docker-compose rewritten for
  Postgres + gua; Dockerfile bumped to Go 1.25; deleted dead `loghook/` and stale
  `testdata/`; corrected stale comments and the removed dump/import API section.

[v3.0.0] — Postgres / River (breaking)

[Changed]

* backing store moved from Redis to **PostgreSQL** via River
  (`github.com/riverqueue/river`). gua is now a stateless horizontal scaler:
  every node dequeues with `FOR UPDATE SKIP LOCKED`.
* deleted the entire hand-rolled reliability layer that worked around Redis —
  per-node buckets, ready queue, distributed locks, de-dup fence, JOB-*-scan /
  JobCheck, down-server reclaim, and SERVER-N slot election + owner-token
  fencing. Postgres row locks + River (retry, periodic, rescuer, leader
  election) provide these.
* env: 4 Redis groups collapsed to one `PG_DSN`. `BACKEND` toggle removed.

[Added]

* `gua_jobs` (definitions), `gua_groups`, `gua_executions` (history) tables;
  `History` reads `gua_executions`; `Stats.ready_queue_depth` = pending River
  occurrences (no slots).
* retry/backoff on failed delivery (new — was fire-once before); River rescuer
  re-runs jobs from crashed workers.
* logging migrated logrus → `log/slog` with env output (stdout/file/both),
  format/level, and lumberjack file rotation.
* River stress/timing test; docs + diagrams redrawn for Postgres.

[Notes]

* delivery is at-least-once (retry/rescue) — consumers must be idempotent.
* future-scheduled jobs have a few seconds of River scheduler latency (vs the
  old 700ms ticker) — the trade-off for durability.

[v2.0.0] — harden (breaking)

[Changed]

* delivery reduced to two transports: HTTP and gRPC. Removed REMOTE (remote-shell
  / gua-node) and LUA (in-process script) trigger types.
* HTTP callback is now a JSON POST envelope (was a GET ping); same fields as the
  gRPC trigger. `exec_command` -> `payload` (string).
* new gRPC services: `GuaAdmin` (CRUD, mirrors the REST API) and `GuaCallback`
  (`OnJobTrigger`, Push). Removed old node/JobReply RPCs.
* removed app-level OTP auth; group is now a pure namespace. Use transport-level
  auth (mTLS / gateway) if needed.
* HTTP client swapped greq/requestwork (archived) -> resty.

[Added]

* monitoring: `GET /v1/status`, `GET /v1/{group}/history`, web console `GET /ui`.
* execution history persisted to Redis with TTL retention (`GUA_HISTORY_TTL`).
* test suite: cron parser units + miniredis lifecycle integration + lock/fencing
  + stress/timing harness.
* docs refresh: ARCHITECTURE.md, MONITORING.md, EVAL.md, new drawio diagrams.

[Fixed / hardened]

* `STARTLOCK`/`JOBCHECKLOCK` are TTL + owner-token locks (Lua CAS release) — no
  more deadlock on holder crash; JobCheck no longer runs unlocked.
* per-firing de-dup fence on the ready queue (reclaim / JobCheck can't
  double-enqueue).
* SERVER-N slot fencing (owner token + heartbeat CAS) closes the snowflake
  node-id collision window; gRPC delivery connection pooling.
* removed panics (nil request_url), context leaks, the uncatchable SIGKILL
  handler, and the heavy lua/mysql/telegram dependency tree.

[v1.4.1]

* remove keys use and use scan iter

[v1.4.0]

* add job key JOB-{}-{}-scan life-time
* add auto check miss job key
* fix server recover

[v1.3.0]

* add remote group api

[v1.2.0]

* add delete jobs by group and job name

[v1.1.0]

[add]

* add GroupJobClear api

[v1.0.0]

[fix]

* fix http respone
* fix logger panic

[v1.0.0-rc6]

[fix]

* fix delpy func
* fix time start

[v1.0.0-rc5]

[fix]

* fix redis disconnect


[v1.0.0-rc4]

[Add]

* redis heartbeat 1 op/second


[v1.0.0-rc3]

[Fix]

* gua-node provide fault tolerance (300s)

[Add]

* gua add edit api (only edit request_url & exec_cmd)


[v1.0.0-rc2]

[Fix]

* gua grcp remote to gua-node forget close connection
* init lock bug


[v1.0.0-rc1]

[Add]

* Add License
* Add mux handler CORS 
* Add job & func filed memo

[Fix]

* time trigger lua param add groupname
* use mulit build stage for docker
* fix api md
* fix admin namepsace to loghook namespace
* vendor remake


[Add]

* lua add telegram api



[v1.0.0.beta5] 2019-03-27

[Add]

* add dump all data & dump by group
* add import data
* add global version api


[Fix]

* fix api prefix path /v1



[v1.0.0.beta4] 2019-03-22

[Add]

* add get group list api
* add get group info api
* add get node list api

[Fix]

* fix get jobs  exec_cmd byte to string
* fix get job list api for restful old: /jobs    new: /{group_name}/job/list
* fix register api for restful old: /register    new: /register/group
* fix add job list api for restful old: /add    new: /add/job
* fix active job list api for restful old: /active    new: /active/job
* fix delete job list api for restful old: /delete    new: /delete/job
* fix pause job list api for restful old: /pause    new: /pause/job





[v1.0.0.beta3] 2019-03-19

[Fix]

* fix env default

[Add]

* add remote exec log callback
* add query nodes api



[v1.0.0.beta2] 2019-03-18

[Add]
* add group api
* add luaweb core
* add func api
* add query api


[Remove]
* remove luacore loader.go

[Fix]
* fix readme
* fix delayquene BLPOP use c.Do
* fix glua redis lib bug

[v1.0.0.beta1] 2019-02-25

[Add]

* add readme
* add gitlab ci
* add testdata
* add makefile
