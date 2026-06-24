# gua monitoring

Three read paths, all served from the HTTP listener.

## `GET /v1/status` — queue health

Read-only snapshot.

```json
{
  "now": 1782268643,
  "ready_queue_depth": 0,
  "down_server_backlog": 0,
  "servers": []
}
```

| Field | Meaning |
|---|---|
| `ready_queue_depth` | count of pending River occurrences (`river_job` not yet run) |
| `down_server_backlog` | always 0 — no per-node buckets on Postgres |
| `servers[]` | always empty — gua nodes are stateless, no slot election (kept for API shape) |

## `GET /v1/{group}/history?limit=N` — execution history

Recent executions for a group, newest first (`limit` default 100, max 1000).

```json
[
  {
    "seq": 42,
    "job_id": "2069610792084312064",
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

- Stored in the `gua_executions` table; the worker records every attempt and
  prunes rows older than the retention window on write (no separate sweeper).
- **Retention** is env-configurable: `GUA_HISTORY_TTL` (seconds, default
  `432000` = 5 days). Set `0` to disable history recording entirely.
- `success`/`message` come from the consumer's response (HTTP body / gRPC
  `JobResult.message`); `error` is set on failure.

## `GET /ui` — engineering console

A single self-contained HTML page (no build step, no auth). Enter a group name
and it polls `/v1/status`, `/v1/{group}/job/list`, and
`/v1/{group}/history` — cluster health, scheduled jobs, and recent executions —
with an optional 3s auto-refresh. It is a probe + API-validation surface for
RD/ops, not an end-user product.

## Logging

gua logs via the standard library `log/slog`, configured from the environment
(see `env.river.example`):

- `LOG_OUTPUT` — `stdout` (default) / `file` / `both`
- `LOG_FILE` — path when writing to a file (default `gua.log`)
- `LOG_FORMAT` — `json` (default) / `text`; `LOG_LEVEL` — `debug|info|warn|error`
- File output is rotated by lumberjack: `LOG_ROTATE_MAX_SIZE_MB` (default 100),
  `LOG_ROTATE_MAX_BACKUPS` (7), `LOG_ROTATE_MAX_AGE_DAYS` (30),
  `LOG_ROTATE_COMPRESS` (false).
