# gua 取代 JobScheduler —— 評估與遷移

> 🌐 [English](EVAL.md) · **繁體中文**

輕量評估（JobScheduler 使用量少;破壞性變更可接受）。

## 功能對位

| 能力 | gua |
|---|---|
| 一次性 / cron / 間隔排程 | ✅ `@once`、6 欄位 cron、`@every` |
| 動態 新增 / 編輯 / 暫停 / 恢復 / 刪除 | ✅ REST + gRPC |
| 投遞給消費者 | ✅ HTTP POST 信封 **或** gRPC `OnJobTrigger`(Push） |
| 叢集 / HA | ✅ 無狀態水平擴展（SKIP LOCKED） |
| 監控 | ✅ `/v1/status`、per-group 執行歷史、`/ui` |

結論:**功能上涵蓋 JobScheduler,並多了結構化 gRPC 投遞 + 內建監控。**

## 消費者的破壞性變更

1. **投遞型別只剩 HTTP / gRPC**。遠端 shell（REMOTE）與 in-process Lua（LUA）已移除。
2. **HTTP callback 改成 `POST` + JSON 信封**（原本是 `GET` ping）。消費者要收 POST、讀 body。
3. **`payload` 取代 `exec_command`**（字串,觸發時原樣交還）。
4. **沒有 app 層 OTP**。`register_group` 不再回 secret;callback 不帶 `otp_code`。需要的話把認證放傳輸層（mTLS / gateway / network policy）。

遷移 = 每個相依方把 callback endpoint 改成收 POST 信封（或實作 gRPC
`GuaCallback`),並拔掉 OTP 處理。

## 投遞語意

**At-least-once。** `FOR UPDATE SKIP LOCKED` 讓「撈取」在整個叢集是 exactly-once
（壓測顯示 2k 同刻 job 0 重複),但 River 會重試失敗、並把崩潰 worker 的 job 撿
回,所以一個 job 仍可能被「投遞」超過一次 —— 消費端必須**冪等**。

## 時間特性（Postgres / River）

排在未來的 job 由 River 的 scheduler 提升(promote),相對於舊的 in-memory ticker
會多幾秒延遲。壓測（單節點、HTTP 投遞、真 Postgres):N 個 job 排在同一刻 →

| N | 漏觸發 | 重複 | lateness p50 | p99 | max |
|---|---|---|---|---|---|
| 500 | 0 | 0 | ~3.0s | ~3.5s | ~3.5s |
| 2000 | 0 | 0 | ~4.2s | ~6.3s | ~6.3s |

完整性 + 無重複的不變式全程成立（`SKIP LOCKED`)。lateness 是 River 的提升延遲 +
把 N 個 job 經由 worker pool 排空 —— **不是漏觸發**。對分鐘/小時級排程無感;若要
sub-second 精度,Postgres/River 不是對的底層(`harden` 上的 Redis 線有 700ms 地板)。

## 殘留風險 / 後續

- **投遞是 at-least-once** —— `SKIP LOCKED` 讓撈取 exactly-once,但「投遞成功後、
  commit 前崩潰」(或 River rescuer)可能重投。消費端必須**冪等**。
- **未來 job 延遲**(如上)—— River 提升排定 job 需要幾秒。對目標用途可接受,寫
  明只是別期待 sub-second。
- **River 是 pre-1.0**(`v0.x`)—— 活躍維護（MPL-2.0),但 API 未凍結;釘版本、
  升級時複檢。
- **Postgres 是唯一真實來源** —— 用正常的持久化(replication / backup)。一顆 DB、
  一個 failure domain —— 但 ACID,所以不用自己寫恢復碼。
- **上線**:與舊排程器並行部署,一次搬一個相依方(把它的 job 指向 gua、改它的
  callback),看 `/ui` + 歷史;回退就把相依方指回去。**全新 Postgres 部署不需要
  資料遷移**,只有「原地把跑著的 Redis 切過去」才需要。
