# gua

[![Stars](https://img.shields.io/github/stars/syhlion/gua.svg)](https://github.com/syhlion/gua)
[![Build Status](https://drone.syhlion.tw/api/badges/syhlion/gua/status.svg)](https://drone.syhlion.tw/syhlion/gua)
[![Go](https://img.shields.io/github/go-mod/go-version/syhlion/gua.svg)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Backed by PostgreSQL](https://img.shields.io/badge/backed%20by-PostgreSQL-336791.svg)](https://www.postgresql.org)
[![docs English](https://img.shields.io/badge/docs-English-lightgrey.svg)](README.md)
[![docs 繁體中文](https://img.shields.io/badge/docs-%E7%B9%81%E9%AB%94%E4%B8%AD%E6%96%87-blue.svg)](README.zh-TW.md)

以 Go 實作的分散式 crontab 風格排程器,後端為 **PostgreSQL**(透過
[River](https://riverqueue.com))。

Job 註冊在某個 group 底下,在排定時間觸發(一次性 `@once`、cron、或 `@every`)。
當 job 觸發時,gua 會把一份 **trigger 信封** 透過兩種傳輸之一送給消費者:

- **HTTP** — 對 `<target>` 發 `POST`,body 為 JSON
- **gRPC** — `OnJobTrigger`(Push:gua 主動 dial 消費者的 gRPC server)

兩者帶相同欄位:`job_id`、`job_name`、`group_name`、`plan_time`、`exec_time`、
`payload`。消費者的 HTTP `2xx` / gRPC `JobResult` 即為執行結果,會記進短期保留的
歷史供監控查詢。

> 這是 PostgreSQL 線。Redis 版保留在 `harden` 分支;後端為什麼、怎麼搬到
> Postgres,見 [docs/pg-migration.zh-TW.md](docs/pg-migration.zh-TW.md)。

## 架構

![architecture](docs/diagrams/gua-architecture.png)

- **註冊 / CRUD**(消費者 → gua)——`RegisterGroup`、`AddJob`、`EditJob`、
  `PauseJob`、`ActiveJob`、`DeleteJob`、`ListJobs`,走 HTTP REST(`/v1/...`)或
  對應的 gRPC `GuaAdmin`。
- **派送**(gua → 消費者,job 觸發時)——把 trigger 信封以 JSON `POST`(HTTP)或
  `GuaCallback.OnJobTrigger`(gRPC Push)送出。
- **不需 NATS**——gua 節點是**無狀態**的:全部對同一顆 Postgres 用
  `FOR UPDATE SKIP LOCKED` 撈 job,所以每個 job 在整個叢集只跑一次——沒有 slot
  選舉、沒有 per-node bucket、沒有去重 fence。

加一台節點它就直接開始撈;某台崩潰時,它手上正在跑的 job 由 River 的 rescuer
撿回。完整說明(pipeline、HA、schema)見 [docs/ARCHITECTURE.zh-TW.md](docs/ARCHITECTURE.zh-TW.md)。

## 環境需求

- **PostgreSQL**(唯一的後端)——以 `PG_DSN` 連線;River 開機時會自己跑 migration,
  不需手動建 schema。
- **Go** 1.x 以從原始碼編譯(版本見 [go.mod](go.mod))。

## 快速開始(Docker Compose)

一鍵把 Postgres + gua 拉起來(River 開機自己跑 migration),免設定:

```
docker compose -f docker-compose/docker-compose.yml up --build
```

**驗證效果** — 註冊一個 group、排一個約 5 秒後觸發的一次性 job,再看它觸發:

```
curl localhost:7777/version
# 1. 註冊 group
curl -XPOST localhost:7777/v1/groups -d '{"group_name":"demo"}'
# 2. 排一個約 5 秒後 POST 出 trigger 信封的 job。把 request_url 指到你自己的接收端
#    (例如去 https://webhook.site 拿一個 URL)就能看到信封送達:
curl -XPOST localhost:7777/v1/groups/demo/jobs -d "{\"name\":\"hi\",\"exec_time\":$(( $(date +%s) + 5 )),\"request_url\":\"HTTP@https://webhook.site/your-id\",\"interval_pattern\":\"@once\",\"payload\":\"hello\"}"
# 3. 觸發後,從執行歷史確認
curl localhost:7777/v1/groups/demo/history
```

或開 console <http://localhost:7777/ui>,直接在介面註冊、排程並即時看歷史。

## 執行方式(binary)

```
$ ./gua start -e env.example     # 用 env 檔
$ ./gua start                    # 或直接讀 process 環境變數
```

需要一顆可連的 Postgres(`PG_DSN`);River 開機時會自己跑 migration。所有設定
(Postgres、歷史保留、logging)見 [env.example](env.example)。

## 維運

- **健康檢查**:`GET /version`(build/版本)· web console `GET /ui`。
- **監控**:`GET /v1/status`(佇列健康)· `GET /v1/groups/{group}/history`(近期執行)。
  完整參照見 [docs/MONITORING.zh-TW.md](docs/MONITORING.zh-TW.md)。

## 日誌

輸出方式可選,並支援輪替(rotation),透過環境變數設定:

| 環境變數 | 可選值 |
|---|---|
| `LOG_OUTPUT` | `stdout`(預設)/ `file` / `both` |
| `LOG_FILE` | 路徑(用於 `file` / `both`) |
| `LOG_FORMAT` | `json`(預設)/ `text` |
| `LOG_LEVEL` | `debug` / `info`(預設)/ `warn` / `error` |
| `LOG_ROTATE_MAX_SIZE_MB` / `_MAX_BACKUPS` / `_MAX_AGE_DAYS` / `_COMPRESS` | 輪替設定 |

## 文件

- [docs/ARCHITECTURE.zh-TW.md](docs/ARCHITECTURE.zh-TW.md) — 架構、pipeline、HA(含圖)
- [docs/apiv1.zh-TW.md](docs/apiv1.zh-TW.md) — admin REST API
- [`proto/gua.proto`](./proto/gua.proto) — gRPC `GuaAdmin` + `GuaCallback`
- [docs/MONITORING.zh-TW.md](docs/MONITORING.zh-TW.md) — `/v1/status`、`/v1/groups/{group}/history`、`/ui`
- [docs/EVAL.zh-TW.md](docs/EVAL.zh-TW.md) — 取代 JobScheduler 的評估與遷移

## 測試

```
go test ./...                                          # 單元測試(cron parser)
# 整合 / 壓測需要 Postgres:
GUA_PG_DSN='postgres://user:pass@host:5432/db?sslmode=disable' go test ./delayquene/ -run TestRiver
GUA_STRESS=1 GUA_STRESS_N=2000 GUA_PG_DSN=... go test ./delayquene/ -run TestRiverStress -v
```
