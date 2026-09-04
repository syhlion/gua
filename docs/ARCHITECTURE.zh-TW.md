# gua 架構(PostgreSQL / River)

> 🌐 [English](ARCHITECTURE.md) · **繁體中文**

gua 是一套分散式、crontab 風格的排程器,後端為 PostgreSQL(透過
[River](https://riverqueue.com))。客戶端用 **HTTP REST 或 gRPC** 註冊 job;job
觸發時 gua 用 **HTTP POST 或 gRPC Push** 把 trigger 信封送給消費者。gua 節點無
狀態、可水平擴展。

> 圖的原始檔是每張 PNG 旁邊的 `.drawio` —— 用 draw.io 開即可編輯。

## 系統總覽

![architecture](diagrams/gua-architecture.png)

- **註冊 / CRUD**(消費者 → gua):`RegisterGroup`、`AddJob`、`EditJob`、
  `PauseJob`、`ActiveJob`、`DeleteJob`、`ListJobs` —— HTTP REST(`/v1/...`)與
  等價的 gRPC `GuaAdmin` service。兩者都是同一個 `Quene` 實作上的薄轉接層，
  驗證與錯誤語意完全一致。
- **投遞**(gua → 消費者,job 觸發時):同一份信封
  (`job_id, job_name, group_name, plan_time, exec_time, payload, idempotency_key`)以 JSON `POST`
  (HTTP)或 `GuaCallback.OnJobTrigger`(gRPC Push)送出。消費者的 `2xx` /
  `JobResult` 即執行結果。
- **監控**:`GET /v1/status`、`GET /v1/groups/{group}/history`、Web console `GET /ui`。
  見 [MONITORING.zh-TW.md](MONITORING.zh-TW.md)。

## Pipeline

![pipeline](diagrams/gua-pipeline.png)

`AddJob` 先驗證 job，把 **定義**寫進 `gua_jobs`（真實來源），並用
`river.Insert(ScheduledAt=run_at)` 排定一個 **occurrence** —— 兩者在同一個交易裡，
所以不會出現「有定義卻沒有 occurrence」。occurrence 只帶這次觸發的身分
（`job_id`、`group_name`、`plan_time`）。

River worker 用 `FOR UPDATE SKIP LOCKED` 撈到期的 row（由 LISTEN/NOTIFY 喚醒），然後：

1. **從 `gua_jobs` 讀定義** —— 排定之後被刪除或暫停的 job 會被跳過；中間做過的
   `Edit` 就是這次投遞的內容；
2. 用 job 的 `timeout`（預設 30 秒、上限 10 分鐘）**投遞**信封，並把這次嘗試記進
   `gua_executions`；
3. 成功時，`@once` 的定義直接刪除；recurring 則算 cron `Next()`，寫進定義的
   `exectime`（job list 顯示的就是它）並插入下一個 occurrence —— 同一個交易，而且
   只在 job 仍是 active 時才做；
4. 失敗時回傳錯誤讓 River 用退避重試；最後一次（`GUA_MAX_ATTEMPTS`，預設 25）也
   失敗時，定義會被**暫停**（`active=false`），用盡重試的 job 仍然看得到。

`Pause`、`Delete`、`Active`、`RemoveGroup` 都在同一個交易裡連帶取代或移除待跑的
occurrence；`Active` 永遠不會多加第二個 occurrence。

- **投遞是 at-least-once**：River 會重試失敗、並把崩潰 worker 的 job 撿回，所以
  一個 job 可能被投遞超過一次 —— **消費端必須冪等**。請對信封的
  **`idempotency_key`** 去重（同一次觸發的重投間穩定；`exec_time` 不是）。
  （`SKIP LOCKED` 讓「撈取」是 exactly-once；會重投的是「投遞成功後、commit 前崩潰」那個窗口。）
- **時間特性**：排在未來的 job 由 River 的 scheduler 提升（promote），相對於
  in-memory ticker 會多幾秒延遲。對分鐘/小時級的排程無感；若要 sub-second 精度，
  這就是換取持久性的代價。實測數字見 [EVAL.zh-TW.md](EVAL.zh-TW.md)。
- **關機**：收到 SIGTERM 時先排空 admin listener，再給 River 10 秒讓進行中的投遞
  完成，超過就取消；被取消的投遞會重試（同一個 `idempotency_key`）。

## 叢集與 HA

![cluster](diagrams/gua-cluster.png)

無狀態水平擴展:每台節點都對同一顆 Postgres 用 `SKIP LOCKED` 撈,所以每個 job
只會在一台節點上跑。**沒有** slot 選舉、owner-token fencing、per-node bucket、
down-server 接管、或去重 fence —— 由 Postgres 的 row lock 做協調。River 自己跑
leader election(PG advisory lock)來協調單例維護任務(scheduler / rescuer /
週期性 job),其 rescuer 會在 15 分鐘後把崩潰 worker 留下的 `running` job 撿回重跑。

## Postgres schema

| 表 | 用途 |
|---|---|
| `gua_jobs` | job 定義(active/paused)—— 真實來源;`exectime` = 下次觸發 |
| `gua_groups` | group 命名空間標記 |
| `gua_executions` | 執行歷史(每次嘗試);由週期性 River job 依 `GUA_HISTORY_TTL` 修剪(`created_at` 有索引)|
| `river_job`(+ River 的表) | 佇列:排定的 occurrence、重試、狀態;gua 用 `args @> {...}` 找自己的 row,吃得到 River 的 GIN 索引 |

## 延伸閱讀

- [apiv1.zh-TW.md](apiv1.zh-TW.md) — admin REST API
- [`proto/gua.proto`](../proto/gua.proto) — gRPC `GuaAdmin` + `GuaCallback`
- [MONITORING.zh-TW.md](MONITORING.zh-TW.md) — 狀態 / 歷史 / console / logging
- [EVAL.zh-TW.md](EVAL.zh-TW.md) — 取代 JobScheduler 評估
- [pg-migration.zh-TW.md](pg-migration.zh-TW.md) — Redis → Postgres 遷移
