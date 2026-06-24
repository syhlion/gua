# gua as a JobScheduler replacement — evaluation & migration

> 🌐 **English** · [繁體中文](EVAL.zh-TW.md)

Light evaluation (JobScheduler usage is small; breaking changes are acceptable).

## Feature parity

| Capability | gua |
|---|---|
| One-shot / cron / interval scheduling | ✅ `@once`, 6-field cron, `@every` |
| Dynamic add / edit / pause / resume / delete | ✅ REST + gRPC |
| Delivery to consumer | ✅ HTTP POST envelope **or** gRPC `OnJobTrigger` (Push) |
| Cluster / HA | ✅ slot election + heartbeat, peer reclaim, JobCheck patch |
| Backup / restore | ✅ dump / import |
| Monitoring | ✅ `/v1/status`, per-group execution history, `/ui` console |

Verdict: **functionally covers JobScheduler and adds structured gRPC delivery +
built-in monitoring.**

## What changed for consumers (breaking)

1. **Trigger types reduced to HTTP / gRPC.** Remote-shell (`REMOTE`) and
   in-process Lua (`LUA`) are gone.
2. **HTTP callback is now `POST` with a JSON envelope** (was a `GET` ping with
   query params). Consumers must accept POST and read the body.
3. **`payload` replaces `exec_command`** (string, handed back verbatim on
   trigger). Job model: `request_url` = `HTTP@<url>` / `GRPC@<host:port>`; the
   gRPC API exposes this as `delivery` + `target`.
4. **No app-level OTP.** `register_group` no longer returns a secret; callbacks
   carry no `otp_code`. Put auth at the transport layer (mTLS / gateway /
   network policy) if required.

Migration = each dependent updates its callback endpoint to accept the POST
envelope (or implement the gRPC `GuaCallback`), and drops OTP handling.

## Delivery semantics

**At-least-once.** `FOR UPDATE SKIP LOCKED` makes dequeue exactly-once across the
fleet (stress runs show 0 duplicates to 2k same-instant jobs), but River retries
failures and rescues jobs from crashed workers, so a job can still be *delivered*
more than once — consumers must be **idempotent**.

## Timing (Postgres / River)

Future-scheduled jobs are promoted by River's scheduler, which adds a few
seconds of latency vs the old in-memory ticker. Stress (single node, HTTP
delivery, real Postgres): N jobs at a common scheduled instant →

| N | missed | dup | lateness p50 | p99 | max |
|---|---|---|---|---|---|
| 500 | 0 | 0 | ~3.0s | ~3.5s | ~3.5s |
| 2000 | 0 | 0 | ~4.2s | ~6.3s | ~6.3s |

Completeness + no-duplicate invariants hold throughout (`SKIP LOCKED`). The
lateness is River's scheduler-promotion latency + draining N jobs through the
worker pool — **not** missed fires. For jobs scheduled minutes/hours out this is
irrelevant; if you need sub-second precision, Postgres/River is the wrong
substrate (the Redis line on `harden` has a 700ms floor).

## Residual risks / follow-ups

- **Delivery is at-least-once** — `SKIP LOCKED` makes dequeue exactly-once, but a
  worker crashing after delivering and before commit (or River's rescuer) can
  re-deliver. Consumers must be **idempotent**.
- **Future-job latency** (above) — a few seconds for River to promote scheduled
  jobs. Acceptable for the target use; documented so nobody expects sub-second.
- **River is pre-1.0** (`v0.x`) — actively maintained (MPL-2.0), but the API is
  not frozen; pin the version and review on upgrade.
- **Postgres is the single source of truth** — run it with the usual durability
  (replication / backups). One database, one failure domain — but ACID, so no
  custom recovery code.
- **Rollout:** deploy alongside the old scheduler, migrate one dependent at a
  time (point its jobs at gua, update its callback), watch `/ui` + history; roll
  back by repointing the dependent. A fresh Postgres deployment needs no data
  migration; only an in-place Redis→PG cutover would.
