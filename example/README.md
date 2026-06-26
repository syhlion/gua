# gua example — scheduler demo

A one-command, self-contained demo: set a delay + payload in the browser, and
**watch gua fire the job** live. It shows the whole path — schedule → gua stores
& waits → fires → delivers to a consumer webhook → back to the page — with no
setup.

## Run

From the repo root:

```sh
docker compose -f example/docker-compose.yml up --build
```

Then open **<http://localhost:8080>**.

- **Left pane (backend → schedule)** — set *fire after N seconds* and a *payload*,
  hit *schedule*. The demo backend registers a group and schedules a one-shot job
  in gua: `POST /v1/groups/demo/jobs` with `request_url` pointing at its own
  `/hook`.
- **Right pane (frontend → live effect)** — an SSE stream. You'll see
  **⏱ scheduled** immediately, then **🔥 FIRED** when gua delivers the job to the
  webhook after the delay.

## What's in the stack

| Service | Role |
|---|---|
| `postgres` | gua's only backend (River runs its own migrations) |
| `gua` (`:7777`, internal) | the scheduler — the demo schedules jobs via its REST API |
| `demo` (`:8080`) | tiny stdlib Go backend: serves the page, schedules jobs, receives gua's delivery webhook, streams events to the browser (SSE) |

## Notes

- The job is **one-shot** (`@once`); try a cron / `@every` pattern by changing
  `interval_pattern` in `app/main.go`.
- Delivery is **at-least-once** and carries an `idempotency_key` — see the full
  API in [../docs/apiv1.md](../docs/apiv1.md).
- Demo only — no auth, fixed group (`demo`). For real use, point `request_url` at
  your own consumer (HTTP or gRPC).
