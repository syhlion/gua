# gua docker-compose

Brings up Postgres + gua. River runs its own migrations on startup.

```
docker compose up --build
```

- gua HTTP API on `:7777`, gRPC on `:6666`.
- Quick check: `curl localhost:7777/version`, console at `http://localhost:7777/ui`.
