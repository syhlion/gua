# gua monitoring

Three read paths, all served from the HTTP listener.

## `GET /v1/status` — cluster & queue health

Read-only snapshot from live Redis state.

```json
{
  "now": 1782268643,
  "ready_queue_depth": 0,
  "down_server_backlog": 0,
  "servers": [
    { "name": "SERVER-1", "last_heartbeat": 1782268642, "alive": true }
  ]
}
```

| Field | Meaning |
|---|---|
| `ready_queue_depth` | `LLEN GUA-READY-JOB` — backlog of fired-but-not-yet-delivered jobs |
| `down_server_backlog` | `LLEN down-server` — orphaned buckets awaiting peer reclaim |
| `servers[]` | one per slot: `name`, `last_heartbeat` (unix), `alive` (beat within 15s) |

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

- Stored in a per-group Redis ZSET (`GUA-HIST-{group}`) scored by `exec_time`.
- Each write prunes entries older than the retention window, so the set stays
  bounded without a separate sweeper.
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

## Optional: reply webhook

Setting `JOB_REPLY_HOOK` makes gua also `POST` a `loghook.Payload` (the same
execution record) to that URL after each run. Off by default — the history API
above is the primary path; the webhook is for pushing into an external sink.
