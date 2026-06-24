# gua admin API (v1)

Base path `/v1`. Bodies are JSON. Responses are wrapped as `{"success": ...}`.
There is no app-level auth (see [EVAL.md](./EVAL.md)); protect at the transport
layer (network policy / mTLS / gateway) as needed.

The same operations are available over gRPC via the `GuaAdmin` service
(`proto/gua.proto`).

## Groups

| Method | Path | Body | Notes |
|---|---|---|---|
| POST | `/register/group` | `{"group_name":"G"}` | group is a namespace |
| POST | `/remove/group` | `{"group_name":"G"}` | |
| GET | `/group/list` | | |
| GET | `/{group}/group/info` | | |

## Jobs

| Method | Path | Body |
|---|---|---|
| POST | `/add/job` | see below → returns `job_id` |
| POST | `/edit/job` | `{"group_name","id","request_url","payload"}` |
| POST | `/pause/job` | `{"group_name","job_id"}` |
| POST | `/active/job` | `{"group_name","job_id","exec_time"}` |
| POST | `/delete/job` | `{"group_name","job_id"}` |
| GET | `/{group}/job/list` | |
| DELETE | `/{group}/job/clear` | |
| DELETE | `/{group}/job/delete/{job_name}` | |

### add/job payload

```json
{
  "group_name": "G",
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

## Delivery (what the consumer receives when a job fires)

Both transports carry the same envelope:

- **HTTP** — `POST <target>` with body
  `{"job_id","job_name","group_name","plan_time","exec_time","payload"}`.
  Return `2xx` for success; the body is kept as the result message.
- **gRPC** — `GuaCallback.OnJobTrigger(JobTrigger) -> JobResult{success,message}`
  (the consumer implements this; gua dials `target`).

## Monitoring

| Method | Path | Notes |
|---|---|---|
| GET | `/version` | |
| GET | `/v1/status` | ready-queue depth, down-server backlog, per-slot health |
| GET | `/v1/{group}/history?limit=N` | recent executions (success/fail, timings) |
| GET | `/ui` | single-page engineering console |

## Backup

| Method | Path |
|---|---|
| GET | `/{group}/dump` , `/dump/all` |
| POST | `/import` |
