# gua 監控

> 🌐 [English](MONITORING.md) · **繁體中文**

三條讀取路徑,全部由 HTTP listener 提供。

## `GET /v1/status` — 佇列健康

唯讀快照。

```json
{
  "now": 1782268643,
  "ready_queue_depth": 0,
  "down_server_backlog": 0,
  "servers": []
}
```

| 欄位 | 意義 |
|---|---|
| `ready_queue_depth` | 待跑的 River occurrence 數(`river_job` 尚未執行) |
| `down_server_backlog` | 恆為 0 —— Postgres 上沒有 per-node bucket |
| `servers[]` | 恆為空 —— gua 節點無狀態、無 slot 選舉(為了 API 形狀保留) |

## `GET /v1/{group}/history?limit=N` — 執行歷史

某 group 最近的執行紀錄,由新到舊(`limit` 預設 100,上限 1000)。

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

- 存在 `gua_executions` 表;worker 每次嘗試都記一筆,寫入時順手修剪掉超過保留
  窗口的舊資料(不需要另外的 sweeper)。
- **保留期** 由 env 設定:`GUA_HISTORY_TTL`(秒,預設 `432000` = 5 天);設 `0`
  則完全關閉歷史記錄。
- `success`/`message` 來自消費者的回應(HTTP body / gRPC `JobResult.message`);
  失敗時設 `error`。

## `GET /ui` — 工程 console

一頁自包含 HTML(免 build、免認證)。輸入 group 名,它會輪詢
`/v1/status`、`/v1/{group}/job/list`、`/v1/{group}/history` —— 叢集健康、已排程
job、最近執行 —— 並可選 3 秒自動刷新。定位是 RD/ops 的排查 + API 驗證台,不是給
end user 的產品。

## Logging

gua 透過標準庫 `log/slog` 記 log,由 env 設定(見 `env.river.example`):

- `LOG_OUTPUT` — `stdout`(預設)/ `file` / `both`
- `LOG_FILE` — 寫檔時的路徑(預設 `gua.log`)
- `LOG_FORMAT` — `json`(預設)/ `text`;`LOG_LEVEL` — `debug|info|warn|error`
- 檔案輸出由 lumberjack 滾動:`LOG_ROTATE_MAX_SIZE_MB`(預設 100)、
  `LOG_ROTATE_MAX_BACKUPS`(7)、`LOG_ROTATE_MAX_AGE_DAYS`(30)、
  `LOG_ROTATE_COMPRESS`(false)。
