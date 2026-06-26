# gua admin API (v1)

> 🌐 **English** · [繁體中文](apiv1.zh-TW.md)

Base path `/v1`. Bodies are JSON. Responses are wrapped as `{"success": ...}`.
There is no app-level auth (see [EVAL.md](./EVAL.md)); protect at the transport
layer (network policy / mTLS / gateway) as needed.

The same operations are available over gRPC via the `GuaAdmin` service
(`proto/gua.proto`).

## Groups

| Method | Path | Body | Notes |
|---|---|---|---|
| POST | `/v1/groups` | `{"group_name":"G"}` | create a group (a namespace) |
| GET | `/v1/groups` | | list groups |
| GET | `/v1/groups/{group}` | | group info |
| DELETE | `/v1/groups/{group}` | | remove the group and all its jobs |

## Jobs

Jobs are a sub-resource of a group; the group and job id come from the path.

| Method | Path | Body |
|---|---|---|
| POST | `/v1/groups/{group}/jobs` | see below → returns `job_id` |
| GET | `/v1/groups/{group}/jobs` | |
| PATCH | `/v1/groups/{group}/jobs/{job}` | `{"request_url","payload"}` |
| POST | `/v1/groups/{group}/jobs/{job}/pause` | |
| POST | `/v1/groups/{group}/jobs/{job}/activate` | `{"exec_time"}` |
| DELETE | `/v1/groups/{group}/jobs/{job}` | delete one job by id |
| DELETE | `/v1/groups/{group}/jobs` | clear all jobs; `?name=<job_name>` deletes only matching jobs |

### add-job payload

```json
{
  "job_id": "",                 // optional; empty -> server generates a snowflake
  "name": "daily-report",
  "exec_time": 1782268640,      // unix seconds, first fire
  "interval_pattern": "@once",  // "@once" | cron (sec min hour dom mon dow) | "@every 1h"
  "request_url": "HTTP@https://consumer/hook",  // or "GRPC@host:port"
  "payload": "arbitrary string handed back on trigger",
  "timeout": 5,                 // seconds; used as gRPC call timeout
  "memo": ""
}
```

> **`@every` drifts; cron self-corrects.** A recurring job's next occurrence is
> scheduled (from the completion time) only after the current one is delivered.
> `@every 5m` therefore drifts by the delivery latency each cycle; use a cron
> like `*/5 * * * *` if you need to land on fixed wall-clock boundaries.

## Delivery (what the consumer receives when a job fires)

Both transports carry the same envelope:

- **HTTP** — `POST <target>` with body
  `{"job_id","job_name","group_name","plan_time","exec_time","payload","idempotency_key"}`.
  Return `2xx` for success; the body is kept as the result message.
- **gRPC** — `GuaCallback.OnJobTrigger(JobTrigger) -> JobResult{success,message}`
  (the consumer implements this; gua dials `target`).

**Idempotency.** Delivery is **at-least-once** (River retries failures and
rescues crashed workers). Dedupe on **`idempotency_key`** — it is stable across
re-deliveries of the same firing. Do **not** dedupe on `exec_time` (it changes
per attempt). Equivalent fallback: `job_id` + `plan_time`.

## Monitoring

| Method | Path | Notes |
|---|---|---|
| GET | `/version` | |
| GET | `/healthz` | liveness — `200 ok` while the process serves; does not touch the DB |
| GET | `/readyz` | readiness — `200 ready` when Postgres is reachable, `503` otherwise |
| GET | `/v1/status` | pending-queue depth + queue health (no slots on Postgres) |
| GET | `/v1/groups/{group}/history?limit=N` | recent executions (success/fail, timings) |
| GET | `/ui` | single-page engineering console |

> For Kubernetes: point `livenessProbe` at `/healthz` and `readinessProbe` at
> `/readyz` so traffic is held back while the database is unreachable.
