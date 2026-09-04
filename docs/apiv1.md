# gua admin API (v1)

> 🌐 **English** · [繁體中文](apiv1.zh-TW.md)

Base path `/v1`. Bodies are JSON (max 1 MiB). Responses are wrapped as
`{"success": ...}` on 200 and `{"error": "<message>"}` otherwise:

| Status | Meaning |
|---|---|
| 400 | invalid input (bad name / id / pattern / request_url, malformed JSON) |
| 404 | group or job does not exist (also when the job is in a different group) |
| 409 | group / job id already exists |
| 413 | body larger than 1 MiB |
| 500 | storage error (details in the server log, not in the response) |

There is no app-level auth (see [EVAL.md](./EVAL.md)); protect at the transport
layer (network policy / mTLS / gateway) as needed.

The same operations are available over gRPC via the `GuaAdmin` service
(`proto/gua.proto`); the same errors map to `INVALID_ARGUMENT`, `NOT_FOUND`,
`ALREADY_EXISTS` and `INTERNAL`.

## Groups

| Method | Path | Body | Notes |
|---|---|---|---|
| POST | `/v1/groups` | `{"group_name":"G"}` | create a group (a namespace); `[A-Za-z0-9_]`, max 22 chars |
| GET | `/v1/groups` | | list groups |
| GET | `/v1/groups/{group}` | | group info |
| DELETE | `/v1/groups/{group}` | | remove the group **and all its jobs** (REST and gRPC alike) |

## Jobs

Jobs are a sub-resource of a group; the group and job id come from the path.

| Method | Path | Body | Notes |
|---|---|---|---|
| POST | `/v1/groups/{group}/jobs` | see below | returns `job_id` |
| GET | `/v1/groups/{group}/jobs` | | `exec_time` is the **next** fire (advances after every run of a recurring job) |
| PATCH | `/v1/groups/{group}/jobs/{job}` | `{"request_url","payload"}` | takes effect on the already-scheduled next fire |
| POST | `/v1/groups/{group}/jobs/{job}/pause` | | drops the pending fire; definition kept with `active=false` |
| POST | `/v1/groups/{group}/jobs/{job}/activate` | `{"exec_time"}` | re-arms at `exec_time` (omitted / 0 = now); replaces any pending fire, so calling it twice fires once |
| DELETE | `/v1/groups/{group}/jobs/{job}` | | delete one job by id |
| DELETE | `/v1/groups/{group}/jobs` | | clear all jobs; `?name=<job_name>` deletes only matching jobs; returns the count deleted |

### add-job payload

```json
{
  "job_id": "",                 // optional; [A-Za-z0-9_] max 22; empty -> server generates a random 16-hex id
  "name": "daily-report",       // required, free text
  "exec_time": 1782268640,      // unix seconds, first fire; 0 = now
  "interval_pattern": "@once",  // "@once" | cron (see below) | "@every 1h"
  "request_url": "HTTP@https://consumer/hook",  // or "GRPC@host:port"; target required
  "payload": "arbitrary string handed back on trigger",
  "timeout": 5,                 // seconds per delivery, both transports; 0 = server default (30s); capped at 600
  "memo": ""
}
```

Every field is validated on both the REST and gRPC paths; a bad
`interval_pattern` or an empty target is rejected up front, never stored.

### cron patterns

| Form | Fields | Example |
|---|---|---|
| 5 fields | `minute hour dom month dow` — classic crontab, seconds are 0 | `*/5 * * * *` = every 5 minutes |
| 6 fields | `second minute hour dom month dow` | `*/5 * * * * *` = every 5 seconds |
| descriptor | `@hourly` `@daily` `@midnight` `@weekly` `@monthly` `@yearly` | |
| interval | `@every <Go duration>` | `@every 1h30m` |

Any other field count is an error. `dow` is `0-6` (Sunday = 0) or `sun`..`sat`;
months accept `jan`..`dec`. Patterns are evaluated in the server's time zone
(`TZ` env; the Docker image defaults to UTC).

> **`@every` drifts; cron self-corrects.** A recurring job's next occurrence is
> scheduled (from the completion time) only after the current one is delivered.
> `@every 5m` therefore drifts by the delivery latency each cycle; use a cron
> like `*/5 * * * *` if you need to land on fixed wall-clock boundaries.

## Delivery (what the consumer receives when a job fires)

Both transports carry the same envelope:

- **HTTP** — `POST <target>` with body
  `{"job_id","job_name","group_name","plan_time","exec_time","payload","idempotency_key"}`.
  Return `2xx` for success; the body is kept as the result message. Any other
  status, or no answer within `timeout`, counts as a failure.
- **gRPC** — `GuaCallback.OnJobTrigger(JobTrigger) -> JobResult{success,message}`
  (the consumer implements this; gua dials `target`). `success=false` or an
  RPC error counts as a failure.

**Retries.** A failed delivery is retried by River with exponential backoff, up
to `GUA_MAX_ATTEMPTS` (default 25). When the attempts are exhausted the job is
**paused** (`active=false`, visible in the job list and `/v1/status`) instead of
being dropped silently; `activate` re-arms it. Every attempt is recorded in the
execution history.

**Idempotency.** Delivery is **at-least-once** (retries, and River rescues jobs
from crashed workers). Dedupe on **`idempotency_key`** — it is stable across
re-deliveries of the same firing. Do **not** dedupe on `exec_time` (it changes
per attempt). Equivalent fallback: `job_id` + `plan_time`.

## Monitoring

| Method | Path | Notes |
|---|---|---|
| GET | `/version` | |
| GET | `/healthz` | liveness — `200 ok` while the process serves; does not touch the DB |
| GET | `/readyz` | readiness — `200 ready` when Postgres is reachable, `503` otherwise |
| GET | `/v1/status` | pending / running / retryable occurrences, active / paused jobs |
| GET | `/v1/groups/{group}/history?limit=N` | recent executions (success/fail, timings) |
| GET | `/ui` | single-page engineering console |

> For Kubernetes: point `livenessProbe` at `/healthz` and `readinessProbe` at
> `/readyz` so traffic is held back while the database is unreachable.
