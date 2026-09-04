# gua monitoring

> 🌐 **English** · [繁體中文](MONITORING.zh-TW.md)

Three read paths, all served from the HTTP listener.

## `GET /v1/status` — queue health

Read-only snapshot.

```json
{
  "now": 1782268643,
  "ready_queue_depth": 12,
  "running": 1,
  "retryable": 0,
  "jobs_active": 12,
  "jobs_paused": 1
}
```

| Field | Meaning |
|---|---|
| `ready_queue_depth` | occurrences waiting to run (scheduled / available / retryable / pending) — roughly one per active job |
| `running` | deliveries in flight right now |
| `retryable` | occurrences whose last delivery failed and are waiting for the next attempt — non-zero means a consumer is failing |
| `jobs_active` / `jobs_paused` | job definitions by state; a job lands in `paused` on `pause`, or after exhausting its delivery attempts |

## `GET /v1/groups/{group}/history?limit=N` — execution history

Recent executions for a group, newest first (`limit` default 100, max 1000).

```json
[
  {
    "seq": 42,
    "job_id": "1f3a9c2e7b4d5a60",
    "group_name": "SMOKE",
    "type": "HTTP",
    "plan_time": 1782268640,
    "exec_time": 1782268640,
    "finish_time": 1782268640,
    "success": true,
    "message": "sink-ok",
    "exec_machine_host": "host-1"
  }
]
```

- Stored in the `gua_executions` table; the worker records every attempt.
  Rows older than the retention window are deleted by a periodic prune job
  (every 10 minutes, run once per cluster via River's leader).
- **Retention** is env-configurable: `GUA_HISTORY_TTL` (seconds, default
  `432000` = 5 days). Set `0` to disable history recording entirely.
- `success`/`message` come from the consumer's response (HTTP body / gRPC
  `JobResult.message`); `error` is set on failure.

## `GET /ui` — engineering console

A single self-contained HTML page (no build step, no auth). Enter a group name
and it polls `/v1/status`, `/v1/groups/{group}/jobs`, and
`/v1/groups/{group}/history` — queue health, scheduled jobs (with the next fire
time), and recent executions — with an optional 3s auto-refresh. It is a probe +
API-validation surface for RD/ops, not an end-user product.

## Logging

gua logs via the standard library `log/slog`, configured from the environment
(see `env.example`):

- `LOG_OUTPUT` — `stdout` (default) / `file` / `both`
- `LOG_FILE` — path when writing to a file (default `gua.log`)
- `LOG_FORMAT` — `json` (default) / `text`; `LOG_LEVEL` — `debug|info|warn|error`
- File output is rotated by lumberjack: `LOG_ROTATE_MAX_SIZE_MB` (default 100),
  `LOG_ROTATE_MAX_BACKUPS` (7), `LOG_ROTATE_MAX_AGE_DAYS` (30),
  `LOG_ROTATE_COMPRESS` (false).

Delivery failures are logged at `warn` (will retry) and `error` (attempts
exhausted, job paused) with `job`, `group`, `attempt` and `error` attributes.
