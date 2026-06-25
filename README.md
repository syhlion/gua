# gua

[![docs: English](https://img.shields.io/badge/docs-English-blue)](README.md)
[![docs: 繁體中文](https://img.shields.io/badge/docs-%E7%B9%81%E9%AB%94%E4%B8%AD%E6%96%87-lightgrey)](docs/README.zh-TW.md)
[![Go](https://img.shields.io/github/go-mod/go-version/syhlion/gua)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-green)](LICENSE)

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

## Quick start (Docker Compose)

Brings up Postgres + gua (River runs its own migrations on startup):

```
docker compose -f docker-compose/docker-compose.yml up --build
```

Then:

```
curl localhost:7777/version
# register a group and schedule a job that fires in 5s:
curl -XPOST localhost:7777/v1/register/group -d '{"group_name":"demo"}'
curl -XPOST localhost:7777/v1/add/job -d "{\"group_name\":\"demo\",\"name\":\"hi\",\"exec_time\":$(( $(date +%s) + 5 )),\"request_url\":\"HTTP@https://example.com/hook\",\"interval_pattern\":\"@once\",\"payload\":\"hello\"}"
```

Console at <http://localhost:7777/ui>.

## Usage (binary)

```
$ ./gua start -e env.example     # with an env file
$ ./gua start                    # or rely on the process environment
```

Needs a reachable Postgres (`PG_DSN`); River runs its own migrations on startup.
See [env.example](env.example) for all knobs (Postgres, history retention,
logging + rotation).

## Architecture

![architecture](docs/diagrams/gua-architecture.png)

gua nodes are **stateless**: they all dequeue from one Postgres with
`FOR UPDATE SKIP LOCKED`, so each job runs exactly once across the fleet — no
slot election, no per-node buckets, no de-dup fence. Add a node and it just
starts pulling; a crashed node's in-flight jobs are reclaimed by River's
rescuer. Full write-up (pipeline, HA, schema) in
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Docs

- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) — architecture, pipeline, HA (with diagrams)
- [apiv1.md](docs/apiv1.md) — admin REST API
- [`proto/gua.proto`](./proto/gua.proto) — gRPC `GuaAdmin` + `GuaCallback`
- [docs/MONITORING.md](docs/MONITORING.md) — `/v1/status`, `/v1/{group}/history`, `/ui`, logging
- [EVAL.md](docs/EVAL.md) — JobScheduler replacement evaluation & migration

## Tests

```
go test ./...                                          # unit (cron parser)
# integration / stress need Postgres:
GUA_PG_DSN='postgres://user:pass@host:5432/db?sslmode=disable' go test ./delayquene/ -run TestRiver
GUA_STRESS=1 GUA_STRESS_N=2000 GUA_PG_DSN=... go test ./delayquene/ -run TestRiverStress -v
```
