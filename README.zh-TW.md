# gua

> 🌐 [English](README.md) · **繁體中文**

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

## 快速開始(Docker Compose)

一鍵把 Postgres + gua 拉起來(River 開機自己跑 migration):

```
docker compose -f docker-compose/docker-compose.yml up --build
```

然後:

```
curl localhost:7777/version
# 註冊 group 並排一個 5 秒後觸發的 job:
curl -XPOST localhost:7777/v1/register/group -d '{"group_name":"demo"}'
curl -XPOST localhost:7777/v1/add/job -d "{\"group_name\":\"demo\",\"name\":\"hi\",\"exec_time\":$(( $(date +%s) + 5 )),\"request_url\":\"HTTP@https://example.com/hook\",\"interval_pattern\":\"@once\",\"payload\":\"hello\"}"
```

Console 在 <http://localhost:7777/ui>。

## 使用(binary)

```
$ ./gua start -e env.example     # 用 env 檔
$ ./gua start                    # 或直接讀 process 環境變數
```

需要一顆可連的 Postgres(`PG_DSN`);River 開機時會自己跑 migration。所有設定
(Postgres、歷史保留、logging + 滾動)見 [env.example](env.example)。

## 架構

![architecture](docs/diagrams/gua-architecture.png)

gua 節點是**無狀態**的:全部對同一顆 Postgres 用 `FOR UPDATE SKIP LOCKED` 撈
job,所以每個 job 在整個叢集只跑一次——沒有 slot 選舉、沒有 per-node bucket、
沒有去重 fence。加一台節點它就直接開始撈;某台崩潰時,它手上正在跑的 job 由
River 的 rescuer 撿回。完整說明(pipeline、HA、schema)見
[docs/ARCHITECTURE.zh-TW.md](docs/ARCHITECTURE.zh-TW.md)。

## 文件

- [docs/ARCHITECTURE.zh-TW.md](docs/ARCHITECTURE.zh-TW.md) — 架構、pipeline、HA(含圖)
- [apiv1.zh-TW.md](./apiv1.zh-TW.md) — admin REST API
- [`proto/gua.proto`](./proto/gua.proto) — gRPC `GuaAdmin` + `GuaCallback`
- [docs/MONITORING.zh-TW.md](docs/MONITORING.zh-TW.md) — `/v1/status`、`/v1/{group}/history`、`/ui`、logging
- [EVAL.zh-TW.md](./EVAL.zh-TW.md) — 取代 JobScheduler 的評估與遷移

## 測試

```
go test ./...                                          # 單元測試(cron parser)
# 整合 / 壓測需要 Postgres:
GUA_PG_DSN='postgres://user:pass@host:5432/db?sslmode=disable' go test ./delayquene/ -run TestRiver
GUA_STRESS=1 GUA_STRESS_N=2000 GUA_PG_DSN=... go test ./delayquene/ -run TestRiverStress -v
```
