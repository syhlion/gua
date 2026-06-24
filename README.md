# gua

Distributed crontab-style scheduler in Go, backed by Redis.

Jobs are registered against a group and fire at a planned time (one-shot
`@once`, cron, or `@every`). When a job fires, gua delivers a **trigger
envelope** to the consumer over one of two transports:

- **HTTP** — `POST <target>` with a JSON body
- **gRPC** — `OnJobTrigger` (Push: gua dials the consumer's gRPC server)

Both carry the same fields: `job_id`, `job_name`, `group_name`, `plan_time`,
`exec_time`, `payload`. The consumer's HTTP `2xx` / gRPC `JobResult` is the
execution result, recorded in a short-retention history for monitoring.

> History note: the old `REMOTE` (remote-shell node) and `LUA` (in-process
> script) trigger types, and app-level OTP auth, were removed. See
> [EVAL.md](./EVAL.md) for the migration/compat notes.

## Usage

```
$ ./gua start -e env.example     # with an env file
$ ./gua start                    # or rely on the process environment
```

[env.example](./env.example) · [docker-compose](./docker-compose)

## Architecture

```
register/CRUD ─(HTTP REST or gRPC GuaAdmin)─▶ gua ──▶ Redis (bucket / ready queue)
                                                │
                  job fires ─▶ delivery ────────┤── HTTP  POST envelope ─▶ consumer
                                                └── gRPC  OnJobTrigger  ─▶ consumer
```

Multiple gua instances form a cluster (slot election + heartbeat, orphaned
work is reclaimed by peers, a periodic JobCheck patches lost jobs).

## API

- Admin REST: [apiv1.md](./apiv1.md)
- Admin gRPC (mirrors REST) + consumer callback: [`proto/gua.proto`](./proto/gua.proto)
- Monitoring: `GET /v1/status`, `GET /v1/{group}/history`, web console at `GET /ui`

## Tests

```
go test ./...                                   # unit + miniredis integration
GUA_STRESS=1 GUA_STRESS_N=2000 go test ./delayquene/ -run TestStress -v
```
