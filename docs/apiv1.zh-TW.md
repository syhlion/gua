# gua admin API（v1）

> 🌐 [English](apiv1.md) · **繁體中文**

base path 為 `/v1`。body 為 JSON（上限 1 MiB）。200 時回應包成 `{"success": ...}`，
其他狀態回 `{"error": "<訊息>"}`：

| 狀態碼 | 意義 |
|---|---|
| 400 | 輸入不合法（名稱 / id / pattern / request_url 錯、JSON 壞掉） |
| 404 | group 或 job 不存在（job 在別的 group 底下也算不存在） |
| 409 | group / job id 已存在 |
| 413 | body 超過 1 MiB |
| 500 | 儲存層錯誤（細節在 server log，不放進回應） |

沒有 app 層認證（見 [EVAL.zh-TW.md](./EVAL.zh-TW.md)）；需要的話請在傳輸層
（network policy / mTLS / gateway）保護。

同樣的操作也可透過 gRPC `GuaAdmin` service 使用（`proto/gua.proto`）；同樣的錯誤
對應到 `INVALID_ARGUMENT`、`NOT_FOUND`、`ALREADY_EXISTS`、`INTERNAL`。

## Group

| Method | Path | Body | 說明 |
|---|---|---|---|
| POST | `/v1/groups` | `{"group_name":"G"}` | 建立 group（命名空間）；`[A-Za-z0-9_]`，最長 22 字 |
| GET | `/v1/groups` | | 列出 groups |
| GET | `/v1/groups/{group}` | | group 資訊 |
| DELETE | `/v1/groups/{group}` | | 移除 group **及其所有 job**（REST 與 gRPC 行為相同） |

## Job

job 是 group 的子資源；group 與 job id 都從路徑帶。

| Method | Path | Body | 說明 |
|---|---|---|---|
| POST | `/v1/groups/{group}/jobs` | 見下 | 回傳 `job_id` |
| GET | `/v1/groups/{group}/jobs` | | `exec_time` 是**下一次**觸發時間（recurring 每跑一次就往前推） |
| PATCH | `/v1/groups/{group}/jobs/{job}` | `{"request_url","payload"}` | 對已排定的下一次觸發立即生效 |
| POST | `/v1/groups/{group}/jobs/{job}/pause` | | 丟掉待跑的那次；定義保留，`active=false` |
| POST | `/v1/groups/{group}/jobs/{job}/activate` | `{"exec_time"}` | 在 `exec_time` 重新排定（省略或 0 = 現在）；會取代待跑的那次，所以連叫兩次只觸發一次 |
| DELETE | `/v1/groups/{group}/jobs/{job}` | | 依 id 刪一個 job |
| DELETE | `/v1/groups/{group}/jobs` | | 清空所有 job；`?name=<job_name>` 只刪同名的；回傳刪掉的數量 |

### add-job 的 payload

```json
{
  "job_id": "",                 // 可選；[A-Za-z0-9_] 最長 22；空字串 → server 產生 16 個 hex 字元的隨機 id
  "name": "daily-report",       // 必填，自由文字
  "exec_time": 1782268640,      // unix 秒，首次觸發；0 = 現在
  "interval_pattern": "@once",  // "@once" | cron（見下）| "@every 1h"
  "request_url": "HTTP@https://consumer/hook",  // 或 "GRPC@host:port"；target 必填
  "payload": "觸發時原樣交還給消費者的字串",
  "timeout": 5,                 // 每次投遞的秒數，兩種傳輸都適用；0 = server 預設（30 秒）；上限 600
  "memo": ""
}
```

REST 與 gRPC 兩條路徑都驗證每個欄位；壞的 `interval_pattern` 或空的 target 會當場
被拒絕，不會存進去。

### cron 格式

| 形式 | 欄位 | 範例 |
|---|---|---|
| 5 欄 | `分 時 日 月 週`，傳統 crontab，秒固定為 0 | `*/5 * * * *` = 每 5 分鐘 |
| 6 欄 | `秒 分 時 日 月 週` | `*/5 * * * * *` = 每 5 秒 |
| 描述子 | `@hourly` `@daily` `@midnight` `@weekly` `@monthly` `@yearly` | |
| 固定間隔 | `@every <Go duration>` | `@every 1h30m` |

其他欄位數一律回錯誤。`週` 是 `0-6`（週日 = 0）或 `sun`..`sat`；月份接受
`jan`..`dec`。pattern 用 server 的時區計算（`TZ` 環境變數；Docker image 預設 UTC）。

> **`@every` 會漂移；cron 會自我校正。** recurring 的下一個 occurrence 是在「這次
> 投遞完之後」才用完成當下的時間排定的。所以 `@every 5m` 每輪會累積投遞延遲而漂移；
> 若要對齊整點（00:00、00:05…），用 cron `*/5 * * * *`。

## 投遞（job 觸發時消費者收到什麼）

兩種傳輸帶相同信封：

- **HTTP** — 對 `<target>` 發 `POST`，body 為
  `{"job_id","job_name","group_name","plan_time","exec_time","payload","idempotency_key"}`。
  回 `2xx` 視為成功；body 當作結果 message 留存。其他狀態碼、或 `timeout` 內沒回應，
  都算失敗。
- **gRPC** — `GuaCallback.OnJobTrigger(JobTrigger) -> JobResult{success,message}`
  （消費者實作；gua 主動 dial `target`）。`success=false` 或 RPC 錯誤都算失敗。

**重試。** 投遞失敗由 River 用指數退避重試，最多 `GUA_MAX_ATTEMPTS` 次（預設 25）。
用完之後 job 會被**暫停**（`active=false`，job list 與 `/v1/status` 看得到），不會
默默消失；`activate` 可以重新啟動。每次嘗試都記進執行歷史。

**冪等。** 投遞是 **at-least-once**（會重試、River 也會撿回崩潰 worker 的 job）。
請對 **`idempotency_key`** 做去重 —— 它在同一次觸發的重投間穩定不變。**不要**用
`exec_time`（每次嘗試都會變）。等價的替代：`job_id` + `plan_time`。

## 監控

| Method | Path | 說明 |
|---|---|---|
| GET | `/version` | |
| GET | `/healthz` | liveness——進程有在服務就回 `200 ok`，不碰 DB |
| GET | `/readyz` | readiness——Postgres 可達回 `200 ready`，否則 `503` |
| GET | `/v1/status` | 待跑 / 執行中 / 待重試的 occurrence 數，active / paused 的 job 數 |
| GET | `/v1/groups/{group}/history?limit=N` | 最近執行（成功/失敗、時間） |
| GET | `/ui` | 一頁工程 console |

> Kubernetes：`livenessProbe` 指 `/healthz`、`readinessProbe` 指 `/readyz`，
> DB 不可達時就把流量擋在外面。
