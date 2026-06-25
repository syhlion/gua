# gua:Redis → PostgreSQL（River）遷移計畫

> 🌐 [English](pg-migration.md) · **繁體中文**

> **狀態:** Phase 0–7 完成,已合進 master（v3.0.0）。
> **動機:** job 不能掉（需 ACID 持久);且 Redis 版為了補 Redis 的不足堆了一整層
> 手刻可靠性碼 —— 換 PostgreSQL + River 後這層由資料庫保證、整批刪除。

## 為什麼換 —— 刪掉一整層補償碼

Redis 版（保留在 `harden`）為了繞過「Redis 沒有 ACID / ack / 故障接管 / lease」,
手刻了大量機制。換 PostgreSQL + River 後它們**由資料庫提供、整批消失**:

| Redis 版手刻 | Postgres / River |
|---|---|
| 每台 server 一個 sorted-set bucket + `down-server` `ZUNIONSTORE` 接管 | 不需要:job 是全域表的 row,任何 worker 都能撈 |
| `JOB-*-scan` 檢查點 + `JobCheck` 補單 | 不需要:row 持久存在 |
| `STARTLOCK`/`JOBCHECKLOCK` 分散式鎖 | 不需要:`FOR UPDATE SKIP LOCKED` 原生併發 |
| `FENCE-*` 去重 | 不需要:一個 row 不會被兩個 worker 撈到（撈取 exactly-once） |
| `SERVER-N` slot 選舉 + `OWN-SERVER-N` owner-token fencing | 不需要:沒有 per-server bucket、沒有 slot、沒有 snowflake 撞號 |
| `BLPOP` 無 ack | row 撈出後未完成 → 崩潰可被 River rescuer 撿回重跑 |
| sorted-set 時間桶 | 一個 `run_at` 欄位 + `WHERE run_at <= now()` |

## 目標:River,接在既有的 `Quene` 介面後面

gua 本就有 `delayquene.Quene` 介面（`cmd` / `grpc` / `httpv1` 都用它),那就是天然
的接縫 —— 加一個 River 實作即可。

**對應關係**

| gua | River |
|---|---|
| `AddJob` | `river.Insert(JobArgs{...}, ScheduledAt: run_at)` |
| 投遞（resty `POST` / gRPC `OnJobTrigger`) | River `Worker.Work()` 的內容 |
| cron / recurring | 投遞後用 cron `Next()` 重新 Insert |
| retry / backoff | River 內建（**新行為**:原本失敗不重試） |
| 崩潰回收 | River **rescuer**（不用手刻 reaper） |
| `Stats`（`ready_queue_depth`) | 待跑的 River occurrence 數;`servers[]` 為空 |
| `History` | `gua_executions` 表 |
| job id | 改用內部產生（uuid);gua 的外部 `job_id` 存進 `gua_jobs` |

**唯一要設計的整合縫:** gua 的「group 層級 pause / `job/clear`」River 沒有原生對
應 —— 用 `gua_jobs.active=false` + 取消 River 裡尚未跑的 occurrence 實現。

## 保留（不在變更範圍）

- 投遞型別只有 **`HTTP@` / `GRPC@`**（lua/remote 在 `harden` 已移除）與其投遞邏輯。
- `guaproto.Job` model;payload 存 proto bytes。
- 對外 API:`GuaAdmin` gRPC + HTTP REST 語意不變。
- cron（SpecSchedule)語意。
- HTTP client:resty（`internal/httpclient`)。

## 階段（全部完成）

- **Phase 0** 基線:`pg-store` 分支、docker PG、River 選定;`TestRiverSmoke` 證明
  migrate→insert→work 整條鏈對真 PG 可行。
- **Phase 1** River-backed `Quene`:`riverquene.go`;`gua_groups` 表;`Push` →
  `Insert(ScheduledAt)`;投遞 worker 複用 HTTP/gRPC 信封;recurring 自我重排。加
  上 `BACKEND=river` + `PG_DSN` 的 toggle（後續 Phase 5 拔掉,因為純 PG）。
- **Phase 2** job 級操作:`gua_jobs` 當定義真實來源;`List/Delete/Remove/Edit/
  Pause/Active/Stats` 落在它上面;worker 投遞前/重排前都重查 `active`。
- **Phase 3** 可靠性測試:`TestRiverNoDuplicate`(30 同刻 job 各只跑一次)、
  `TestRiverRetry`(失敗→重試→成功)、`TestRiverHistory`、`TestRiverRescuer`
  (worker 卡死、row 卡在 `running` → rescuer 撿回重跑)。
- **PeriodicJobs 評估後不採用**:River 的 PeriodicJobs 排程狀態存在 leader 記憶體、
  不持久(官方文件:程序退出或換 leader 就整個重來);gua 改以「下一次發生」寫成
  **持久的 `river_job` row**(`ScheduledAt`),可撐過重啟/換 leader——對本次遷移的
  持久性目標更好,也更貼 gua 動態/可暫停/DB 為真實來源的 job 模型。維持自我重排。
- **Phase 4** 監控落 PG:`gua_executions` 表 + `History` 查詢;`Stats` ready 深度。
- **Phase 5** 刪 Redis:刪掉整個 Redis 實作 + `migrate` 套件 + miniredis 測試;
  共用型別移到 `types.go`;拔 `redigo`。`cmd.go` 收斂為 River;4 組 Redis env →
  一個 `PG_DSN`（**−2,956 行**）。
- **Phase 6** 資料遷移:全新部署不需要;只有「原地切換跑著的 Redis」才需要寫腳本。
- **Phase 7** 壓測 + 文件:`TestRiverStress`(N=500/2000,0 漏 0 重複,lateness
  ~3–6s);架構圖與文件改寫為 Postgres 版。

## 驗收

- [x] **job 不能掉**:worker 中途崩潰 → River rescuer 重跑(`TestRiverRescuer`)。
- [x] 延遲 / cron 觸發時間正確。
- [x] 多 worker 併發撈取不重複（SKIP LOCKED）。
- [x] retry / backoff（新行為）。
- [x] 投遞仍 at-least-once → 消費端冪等。
- [x] 不再依賴 redis;`go.mod` / vendor 拔 `redigo`。
- [x] 補償碼已刪（scan / jobcheck / down-server / locks / fence / SERVER-N）。
- [x] 對外 API + 投遞型別行為不變;監控 `status` 已對應調整。
