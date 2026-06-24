# gua architecture

gua is a distributed, crontab-style scheduler backed by Redis. Clients register
jobs over **HTTP REST or gRPC**; when a job fires gua delivers a **trigger
envelope** to the consumer over **HTTP POST or gRPC Push**. Multiple gua
instances form a cluster.

> Diagram sources are the `.drawio` files next to each PNG — open them in
> draw.io to edit.

## System overview

![architecture](diagrams/gua-architecture.png)

- **Register / CRUD** (consumer → gua): `RegisterGroup`, `AddJob`, `EditJob`,
  `PauseJob`, `ActiveJob`, `DeleteJob`, `ListJobs` — available as HTTP REST
  (`/v1/...`) and as the gRPC `GuaAdmin` service (equivalent).
- **Delivery** (gua → consumer, when a job fires): the same envelope
  (`job_id, job_name, group_name, plan_time, exec_time, payload`) is sent as a
  JSON `POST` (HTTP) or via `GuaCallback.OnJobTrigger` (gRPC Push — gua dials
  the consumer). The consumer's `2xx` / `JobResult` is the execution result.
- **Monitoring**: `GET /v1/status`, `GET /v1/{group}/history`, and a web
  console at `GET /ui`. See [MONITORING.md](MONITORING.md).

## Delay-queue pipeline

![pipeline](diagrams/gua-pipeline.png)

A job is stored as `JOB-{group}-{id}` plus a bucket entry (a Redis ZSET member
scored by `exec_time`). 80 ticker goroutines scan buckets every 700ms; a due
job is fenced for de-duplication, pushed to the ready queue, picked up by one of
80 BLPOP workers, delivered, and recorded in history. Recurring jobs are
re-scheduled with the cron `Next()` time.

- **Timing floor**: the 700ms ticker bounds resolution — unloaded lateness ≈
  `0–700ms + delivery latency`. Lower the ticker for sub-second precision.
- **De-dup fence** (`FENCE-{job}-{exec_time}`, `SET NX PX`): ensures the
  reclaim / JobCheck paths can't enqueue the same firing twice. Delivery is
  **at-least-once**; consumers should be idempotent.

## Cluster, fencing & failover (HA)

![cluster](diagrams/gua-cluster.png)

- **Slot election**: each node owns a `SERVER-N` slot (and snowflake node id).
  Startup is serialized by a `STARTLOCK` (TTL + owner-token lock).
- **Fencing**: the 1s heartbeat is a CAS — it refreshes the slot only while
  `OWN-SERVER-N` still holds this node's token. If a peer reclaimed the slot
  during a long pause, the next heartbeat fires `OnSupersede` (fatal exit), so
  the node never generates colliding ids.
- **Failover**: a dying node's bucket goroutines `LPUSH` to `down-server`; a
  peer `LPOP`s and `ZUNIONSTORE`-merges the orphaned buckets.
- **JobCheck**: every 30–60s one node takes the `JOBCHECKLOCK` (TTL + owner
  token, self-releases on crash), scans `JOB-*-scan` checkpoints, and re-pushes
  any active job that fell out of rotation.

## Redis keyspace

| Key | Type | Purpose |
|---|---|---|
| `JOB-{group}-{id}` | string (proto) | the stored job |
| `JOB-{group}-{id}-scan` | string (unix) | JobCheck liveness checkpoint |
| `BUCKET-[...]-{n}` | ZSET | due-time index (member=job key, score=exec_time) |
| `GUA-READY-JOB` | LIST | ready queue (RPUSH / BLPOP) |
| `FENCE-{job}-{exec_time}` | string (NX, TTL) | per-firing de-dup fence |
| `SERVER-N` / `OWN-SERVER-N` | string | slot heartbeat / owner token (fencing) |
| `down-server` | LIST | orphaned buckets awaiting peer reclaim |
| `STARTLOCK` / `JOBCHECKLOCK` | string (NX, TTL) | distributed locks |
| `USER_{group}` | string | group namespace marker |
| `GUA-HIST-{group}` | ZSET | execution history (score=exec_time, TTL-pruned) |

## See also

- [apiv1.md](../apiv1.md) — admin REST API
- [`proto/gua.proto`](../proto/gua.proto) — gRPC `GuaAdmin` + `GuaCallback`
- [MONITORING.md](MONITORING.md) — status / history / console
- [EVAL.md](../EVAL.md) — JobScheduler replacement evaluation & migration
