# gua

[![Stars](https://img.shields.io/github/stars/syhlion/gua.svg)](https://github.com/syhlion/gua)
[![Build Status](https://drone.syhlion.tw/api/badges/syhlion/gua/status.svg)](https://drone.syhlion.tw/syhlion/gua)
[![Go](https://img.shields.io/github/go-mod/go-version/syhlion/gua.svg)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Backed by PostgreSQL](https://img.shields.io/badge/backed%20by-PostgreSQL-336791.svg)](https://www.postgresql.org)
[![docs English](https://img.shields.io/badge/docs-English-blue.svg)](README.md)
[![docs 繁體中文](https://img.shields.io/badge/docs-%E7%B9%81%E9%AB%94%E4%B8%AD%E6%96%87-lightgrey.svg)](README.zh-TW.md)

Distributed crontab-style scheduler in Go, backed by **PostgreSQL** (via
[River](https://riverqueue.com)).

Jobs are registered against a group and fire at a planned time (one-shot
`@once`, cron, or `@every`). When a job fires, gua delivers a **trigger
envelope** to the consumer over one of two transports:

- **HTTP** — `POST <target>` with a JSON body
- **gRPC** — `OnJobTrigger` (Push: gua dials the consumer's gRPC server)

Both carry the same fields: `job_id`, `job_name`, `group_name`, `plan_time`,
`exec_time`, `payload`. The consumer's HTTP `2xx` / gRPC `JobResult` is the
execution result, recorded in a short-retention history for monitoring.

> This is the PostgreSQL line. A Redis-backed version lives on the `harden`
> branch; see [docs/pg-migration.md](docs/pg-migration.md) for why and how the
> store moved to Postgres.

## Architecture

![architecture](docs/diagrams/gua-architecture.png)

- **Register / CRUD** (consumer → gua) — `RegisterGroup`, `AddJob`, `EditJob`,
  `PauseJob`, `ActiveJob`, `DeleteJob`, `ListJobs` over HTTP REST (`/v1/...`) or
  the equivalent gRPC `GuaAdmin`.
- **Delivery** (gua → consumer, when a job fires) — the trigger envelope as a
  JSON `POST` (HTTP) or `GuaCallback.OnJobTrigger` (gRPC Push).
- **NATS-free** — gua nodes are **stateless**: they all dequeue from one
  Postgres with `FOR UPDATE SKIP LOCKED`, so each job runs exactly once across
  the fleet — no slot election, no per-node buckets, no de-dup fence.

Add a node and it just starts pulling; a crashed node's in-flight jobs are
reclaimed by River's rescuer. Full write-up (pipeline, HA, schema) in
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Requirements

- **PostgreSQL** (the only backend) — reachable via `PG_DSN`; River runs its own
  migrations on startup, so no manual schema step.
- **Go** 1.x to build from source (see [go.mod](go.mod) for the version).

## Quick start (Docker Compose)

Brings up Postgres + gua (River runs its own migrations on startup) — no setup:

```
docker compose -f docker-compose/docker-compose.yml up --build
```

**See it work** — register a group, schedule a one-shot job ~5s out, then watch it fire:

```
curl localhost:7777/version
# 1. register a group
curl -XPOST localhost:7777/v1/register/group -d '{"group_name":"demo"}'
# 2. schedule a job that POSTs a trigger envelope in ~5s. Point request_url at a
#    catcher you control (e.g. grab a URL from https://webhook.site) to watch it land:
curl -XPOST localhost:7777/v1/add/job -d "{\"group_name\":\"demo\",\"name\":\"hi\",\"exec_time\":$(( $(date +%s) + 5 )),\"request_url\":\"HTTP@https://webhook.site/your-id\",\"interval_pattern\":\"@once\",\"payload\":\"hello\"}"
# 3. after it fires, confirm in the execution history
curl localhost:7777/v1/demo/history
```

Or open the console at <http://localhost:7777/ui> to register, schedule, and watch
history live.

## Run (binary)

```
$ ./gua start -e env.example     # with an env file
$ ./gua start                    # or rely on the process environment
```

Needs a reachable Postgres (`PG_DSN`); River runs its own migrations on startup.
See [env.example](env.example) for all knobs (Postgres, history retention, logging).

## Ops

- **Health**: `GET /version` (build/version) · web console `GET /ui`.
- **Monitoring**: `GET /v1/status` (queue health) · `GET /v1/{group}/history`
  (recent executions). Full reference in [docs/MONITORING.md](docs/MONITORING.md).

## Logging

Output is selectable and rotated, via env:

| Env | Values |
|---|---|
| `LOG_OUTPUT` | `stdout` (default) / `file` / `both` |
| `LOG_FILE` | path (for `file` / `both`) |
| `LOG_FORMAT` | `json` (default) / `text` |
| `LOG_LEVEL` | `debug` / `info` (default) / `warn` / `error` |
| `LOG_ROTATE_MAX_SIZE_MB` / `_MAX_BACKUPS` / `_MAX_AGE_DAYS` / `_COMPRESS` | rotation |

## Docs

- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) — architecture, pipeline, HA (with diagrams)
- [docs/apiv1.md](docs/apiv1.md) — admin REST API
- [`proto/gua.proto`](./proto/gua.proto) — gRPC `GuaAdmin` + `GuaCallback`
- [docs/MONITORING.md](docs/MONITORING.md) — `/v1/status`, `/v1/{group}/history`, `/ui`
- [docs/EVAL.md](docs/EVAL.md) — JobScheduler replacement evaluation & migration

## Tests

```
go test ./...                                          # unit (cron parser)
# integration / stress need Postgres:
GUA_PG_DSN='postgres://user:pass@host:5432/db?sslmode=disable' go test ./delayquene/ -run TestRiver
GUA_STRESS=1 GUA_STRESS_N=2000 GUA_PG_DSN=... go test ./delayquene/ -run TestRiverStress -v
```
