# gua admin API（v1）

> 🌐 [English](apiv1.md) · **繁體中文**

base path 為 `/v1`。body 為 JSON。回應包成 `{"success": ...}`。沒有 app 層認證
（見 [EVAL.zh-TW.md](./EVAL.zh-TW.md)）；需要的話請在傳輸層（network policy /
mTLS / gateway）保護。

同樣的操作也可透過 gRPC `GuaAdmin` service 使用（`proto/gua.proto`）。

## Group

| Method | Path | Body | 說明 |
|---|---|---|---|
| POST | `/register/group` | `{"group_name":"G"}` | group 是命名空間 |
| POST | `/remove/group` | `{"group_name":"G"}` | |
| GET | `/group/list` | | |
| GET | `/{group}/group/info` | | |

## Job

| Method | Path | Body |
|---|---|---|
| POST | `/add/job` | 見下 → 回傳 `job_id` |
| POST | `/edit/job` | `{"group_name","id","request_url","payload"}` |
| POST | `/pause/job` | `{"group_name","job_id"}` |
| POST | `/active/job` | `{"group_name","job_id","exec_time"}` |
| POST | `/delete/job` | `{"group_name","job_id"}` |
| GET | `/{group}/job/list` | |
| DELETE | `/{group}/job/clear` | |
| DELETE | `/{group}/job/delete/{job_name}` | |

### add/job 的 payload

```json
{
  "group_name": "G",
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

## 投遞（job 觸發時消費者收到什麼）

兩種傳輸帶相同信封:

- **HTTP** — 對 `<target>` 發 `POST`,body 為
  `{"job_id","job_name","group_name","plan_time","exec_time","payload"}`。
  回 `2xx` 視為成功;body 當作結果 message 留存。
- **gRPC** — `GuaCallback.OnJobTrigger(JobTrigger) -> JobResult{success,message}`
  （消費者實作;gua 主動 dial `target`）。

## 監控

| Method | Path | 說明 |
|---|---|---|
| GET | `/version` | |
| GET | `/v1/status` | ready 佇列深度、down-server backlog、節點健康 |
| GET | `/v1/{group}/history?limit=N` | 最近執行(成功/失敗、時間) |
| GET | `/ui` | 一頁工程 console |
