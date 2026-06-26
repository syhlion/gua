# gua admin API（v1）

> 🌐 [English](apiv1.md) · **繁體中文**

base path 為 `/v1`。body 為 JSON。回應包成 `{"success": ...}`。沒有 app 層認證
（見 [EVAL.zh-TW.md](./EVAL.zh-TW.md)）；需要的話請在傳輸層（network policy /
mTLS / gateway）保護。

同樣的操作也可透過 gRPC `GuaAdmin` service 使用（`proto/gua.proto`）。

## Group

| Method | Path | Body | 說明 |
|---|---|---|---|
| POST | `/v1/groups` | `{"group_name":"G"}` | 建立 group(命名空間) |
| GET | `/v1/groups` | | 列出 groups |
| GET | `/v1/groups/{group}` | | group 資訊 |
| DELETE | `/v1/groups/{group}` | | 移除 group 及其所有 job |

## Job

job 是 group 的子資源;group 與 job id 都從路徑帶。

| Method | Path | Body |
|---|---|---|
| POST | `/v1/groups/{group}/jobs` | 見下 → 回傳 `job_id` |
| GET | `/v1/groups/{group}/jobs` | |
| PATCH | `/v1/groups/{group}/jobs/{job}` | `{"request_url","payload"}` |
| POST | `/v1/groups/{group}/jobs/{job}/pause` | |
| POST | `/v1/groups/{group}/jobs/{job}/activate` | `{"exec_time"}` |
| DELETE | `/v1/groups/{group}/jobs/{job}` | 依 id 刪一個 job |
| DELETE | `/v1/groups/{group}/jobs` | 清空所有 job;`?name=<job_name>` 只刪同名的 |

### add-job 的 payload

```json
{
  "job_id": "",                 // 可選;空字串 → server 自動產生 snowflake id
  "name": "daily-report",
  "exec_time": 1782268640,      // unix 秒,首次觸發
  "interval_pattern": "@once",  // "@once" | cron(秒 分 時 日 月 週)| "@every 1h"
  "request_url": "HTTP@https://consumer/hook",  // 或 "GRPC@host:port"
  "payload": "觸發時原樣交還給消費者的字串",
  "timeout": 5,                 // 秒;作為 gRPC 呼叫 timeout
  "memo": ""
}
```

> **`@every` 會漂移;cron 會自我校正。** recurring 的下一個 occurrence 是在「這次
> 投遞完之後」才用完成當下的時間排定的。所以 `@every 5m` 每輪會累積投遞延遲而漂移;
> 若要對齊整點(00:00、00:05…),用 cron `*/5 * * * *`。

## 投遞（job 觸發時消費者收到什麼）

兩種傳輸帶相同信封:

- **HTTP** — 對 `<target>` 發 `POST`,body 為
  `{"job_id","job_name","group_name","plan_time","exec_time","payload","idempotency_key"}`。
  回 `2xx` 視為成功;body 當作結果 message 留存。
- **gRPC** — `GuaCallback.OnJobTrigger(JobTrigger) -> JobResult{success,message}`
  （消費者實作;gua 主動 dial `target`）。

**冪等。** 投遞是 **at-least-once**(River 會重試失敗、撿回崩潰 worker 的 job)。
請對 **`idempotency_key`** 做去重 —— 它在同一次觸發的重投間穩定不變。**不要**用
`exec_time`(每次嘗試都會變)。等價的替代:`job_id` + `plan_time`。

## 監控

| Method | Path | 說明 |
|---|---|---|
| GET | `/version` | |
| GET | `/healthz` | liveness——進程有在服務就回 `200 ok`,不碰 DB |
| GET | `/readyz` | readiness——Postgres 可達回 `200 ready`,否則 `503` |
| GET | `/v1/status` | 待跑佇列深度 + 佇列健康(Postgres 無 slot) |
| GET | `/v1/groups/{group}/history?limit=N` | 最近執行(成功/失敗、時間) |
| GET | `/ui` | 一頁工程 console |

> Kubernetes:`livenessProbe` 指 `/healthz`、`readinessProbe` 指 `/readyz`,
> DB 不可達時就把流量擋在外面。
