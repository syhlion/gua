# gua as a JobScheduler replacement — evaluation & migration

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

**At-least-once.** A cross-node dedup fence (`FENCE-<job>-<exectime>`) prevents
the common duplicate sources (orphan-bucket reclaim, JobCheck re-push) from
double-enqueuing the same firing, and stress runs show 0 duplicates to 5k
concurrent — but consumers should still be **idempotent** for safety.

## Timing

Resolution is bounded by the 700ms bucket ticker: unloaded lateness ≈
`0–700ms + delivery latency`. Stress (single node, miniredis, HTTP delivery,
resty client): same-second jobs, all to one host →

| N | missed | dup | p99 | max |
|---|---|---|---|---|
| 2000 | 0 | 0 | ~0.71s | ~0.72s |
| 5000 | 0 | 0 | ~2.4s | ~2.4s |

Completeness + no-duplicate invariants hold throughout. At realistic load (the
target use is *light*) timing sits at the ticker floor. The 5000-to-one-host
tail is the per-host connection cap + resty per-request overhead under extreme
synthetic fan-in — not representative of jobs spread across consumers. For
sub-second precision, lower the ticker (`delayquene` `RunForDelayQuene`).

## Residual risks / follow-ups

- **SERVER-N slot reuse** is mitigated (30s takeover window + STARTLOCK-
  serialized startup), not fully fenced; a live node paused >30s could still be
  reclaimed. Full lease/fencing is a larger change if ever needed.
- **Redis is the single source of truth** — run it with persistence (AOF) and
  treat the 4 logical DBs as one failure domain.
- **`greq` / `requestwork.v2` → `resty`** ✅ done (separate commit). HTTP client
  now `resty.dev/v3` via `internal/httpclient` (retry on network failure, 8MiB
  response cap, keep-alive, per-host conn cap). resty v3 is a release candidate;
  it shows higher per-request overhead than the archived greq under extreme
  fan-in (see Timing) — revisit at resty GA if that load profile ever matters.
- **Rollout:** deploy alongside JobScheduler, migrate one dependent at a time
  (point its jobs at gua, update its callback), watch `/ui` + history; roll back
  by repointing the dependent. No shared state between the two systems.
