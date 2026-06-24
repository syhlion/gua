# gua harden — 計畫與進度

目標:把 gua 從 2019 個人專案,帶到可取代 AgOcean JobScheduler 的狀態。
路線:**瘦身重塑 → 可測 → 不 crash → 分散式可靠 → 可觀測 → 壓測驗收**。每階段是後一階段的安全網,不跳。

工作分支:`harden`(git worktree @ `../gua-harden`),進度推 `origin/harden`。

---

## 決策紀錄(Decisions)

- **破壞性變更可接受**:JobScheduler 使用量少,相依者改接口可行 → 評估從簡。
- **觸發模型重塑(大轉向)**:
  - **移除** REMOTE(遠端 shell / gua-node)與 LUA(in-process 腳本)兩種型別。
  - 往後只有兩種派送:**HTTP** 或 **gRPC**;註冊與 callback 都支援。
  - **gRPC callback = Push**:gua 主動 dial 消費者的 gRPC server,呼叫 `OnJobTrigger`(消費者實作),同步回應取代舊 JobReply。
  - **CRUD 雙軌**:job 建/刪/暫停/恢復/查 + group 註冊,HTTP REST 與 gRPC 等價並存。
  - Job model:`request_url` 前綴 `(HTTP|REMOTE|LUA)@` → `(HTTP|GRPC)@`;`exec_cmd` bytes → 通用 `payload` string。
  - **HTTP 升級對稱**:callback 由 GET ping 改 **POST JSON 信封**(與 gRPC `JobTrigger` 同欄位:job_id/job_name/group/plan_time/exec_time/payload)。
  - **認證先拔掉**:TOTP/OTP 全移除(含 `pquerna/otp` 依賴);group 退化成純命名空間。日後認證走傳輸層(mTLS / 共享密鑰 header),不回頭塞 app 層。
  - **callback 內嵌**:`target` 直接放位址(HTTP:url、GRPC:host:port),不做 endpoint 註冊。
  - **Reply 不走舊制**:結果來自同步回應(HTTP 2xx/body、gRPC `JobResult`),再落地 Redis TTL 歷史(= Monitor Tier 2);全域 webhook 降為可選。
- **Monitor 範圍**:快照 + 歷史 + 工程 Web UI。歷史存 Redis + TTL(env 可調,3–5 天)。UI 只給工程 / API 驗證用。
- **壓測為必交付**:驗證並發任務數 + 執行時間誤差(p50/p95/p99/max)+ 完成率。

---

## 階段與 checklist

### Phase 0 — 決策閘門 + 工作基線
- [x] 開 `harden` worktree、落地計畫、推 remote
- [x] Go/No-Go 快評:**GO(條件式)**,前提 = 接受 at-least-once + 消費端冪等

### Phase 1 — 觸發模型重塑(瘦身 + 雙軌 gRPC)〔先做,刪掉的不該浪費力氣測〕
- [x] **1a 移除 REMOTE/LUA**:刪 `node/`、`luacore/`、`luaweb/`、`apifunc.go`、func HTTP server(`LuaEntrance`)
- [x] 清依賴:`gopher-lua`、`glua-libs`、`telegram-bot-api`、`go-sql-driver/mysql`、`bbolt`(順帶清 dependabot 漏洞)
- [x] `UrlRe` 改 `^(HTTP|GRPC)@`;`ExecuteJob`/`Push`/`Edit` switch 收斂(HTTP 可跑,GRPC 派送留 1b)。build/vet 全綠
- [x] **1b 新 proto**:`GuaAdmin`(RegisterGroup/RemoveGroup/AddJob/EditJob/DeleteJob/PauseJob/ActiveJob/ListJobs)+ `GuaCallback`(`OnJobTrigger`)+ `DeliveryType` enum;刪 node.proto、舊 Gua/JobReply/Node RPC
- [x] **1b 拔認證**:移除 TOTP/OTP 全鏈路 + `pquerna/otp` 依賴;group 純命名空間
- [x] **1b HTTP callback 改 POST JSON 信封**(`TriggerEnvelope`,與 gRPC 同欄位)+ worker GRPC 派送(dial→`OnJobTrigger`)
- [x] **1c** Job model:`exec_cmd` bytes → `payload` string;`request_url` 內部仍存 `TYPE@target`,API 用 `delivery`+`target`;清掉 dead root migrate.go/group.go/spec.go
- [ ] **1b 尾巴**(留待):`benchmark/` 仍是 LUA 樣板(Phase 6 改寫);docs(README/apiv1.md/luamode.md/funclua.md)待重寫

### Phase 2 — 測試骨架(地基)— ✅ 完成
- [x] miniredis 整合測試骨架(`newTestQuene`,真 TCP redis,免抽象 interface)
- [x] cron parser 單元測試(`parser_test.go`)
- [x] 生命週期整合測試:HTTP POST 信封、gRPC Push、recurring、pause、list/delete
- [x] `go test ./...` 全綠

### Phase 3 — 止血(panic / vet / leak)— ✅ 完成
- [x] `FindStringSubmatch` nil guard(Push/Edit/worker)+ 入口驗證
- [x] `context.WithTimeout` cancel 洩漏修正、移除 `signal.Notify(SIGKILL)`、unreachable code
- [x] `go vet ./...` 乾淨

### Phase 4 — delayquene 可測化 — ✅ 由 Phase 2 達成
- [x] 改用 miniredis 真實整合測試,取代原訂 redis interface 抽象(改動更小、更貼近真實)
- [x] cron `Next(t)` 本就吃時間參數、可測,免 clock 注入
- [x] 核心分支已覆蓋:到期推就緒、`@once`/cron 重入桶、pause、CRUD
- [ ] (補)down-server 併桶接管、JobCheck 補單 專項測試 → 隨 Phase 6 補

### Phase 5 — ★ Monitor Tier 1:唯讀狀態 API — ✅ 完成
- [x] `Quene.Stats()` + `GET /v1/status`:ready 佇列深度、down-server backlog、叢集槽位健康(SERVER-N + 心跳 + alive)
- [x] job 狀態/active/下次執行時間 → 既有 `GET /v1/{group}/job/list`
- [x] 純讀;TestStats 覆蓋;作為 Phase 6 硬化的驗證工具

### Phase 6 — 分散式安全性硬化 + 壓測驗收 — ✅ 完成
- [x] `STARTLOCK`/`JOBCHECKLOCK` 換 TTL + owner token 鎖(Lua CAS 釋放);JobCheck 不再無鎖硬跑
- [x] ready-queue dedup fence(`FENCE-<job>-<exectime>`,SET NX PX);reclaim/JobCheck 不重推
- [x] gRPC 連線池化(per-addr 快取,免每次 Dial)
- [~] `SERVER-N` 撞號窗口:接管門檻 15s→30s 縮小窗口 + startup 由 STARTLOCK 序列化(完整 fencing 列殘留風險)
- [x] **壓測驗收通過**:N=500/2000/5000 同秒,0 漏觸發 0 重複,誤差 < 700ms ticker 地板(p99<300ms@5k)

### Phase 7 — ★ Monitor Tier 2:執行歷史 + 工程 Web UI — ✅ 完成
- [x] 執行結果落地 Redis ZSET(score=exec_time,寫入時 prune 到 retention;`GUA_HISTORY_TTL` env,預設 5 天)
- [x] `GET /v1/{group}/history?limit=` 查詢 API + `GET /ui` 單頁工程 console(status/jobs/history/auto-refresh)
- [x] 整合測試 TestHistoryRecorded + 真 redis 端到端 smoke 驗證(register→add→fire→history→ui)

### Phase 8 — 取代評估 + 遷移計畫(從簡)
- [ ] gua vs JobScheduler 精簡對位 + 缺口、相依者改接口清單、上線/回退

---

## 壓測方案(Stress / Timing Accuracy)

延伸 `benchmark/`(現有:丟 1500 個 `@once` job,callback 記 `actual - scheduled`)。
改測 HTTP / GRPC 兩型別(LUA/REMOTE 已移除)。

**指標**:誤差 `actualExecTime - planTime` 的 p50/p95/p99/max;超時比例(>1s/>2s);完成率(漏觸發 / 重複觸發);ready queue 堆積。
**維度**:並發(同秒到期 100/1k/5k/10k)× 存量(active 1k/10k/50k)× 型別(HTTP / GRPC)× 叢集(1 vs 3 台)。
**誤差地板**:受 **700ms bucket ticker** 限制,無載約 `0–700ms + 執行延遲`;測尾端在壓力下多大。需 sub-second 則 ticker 是改點。
**驗收門(Phase 6)**:漏觸發 = 0;重複觸發在冪等保護下無副作用;p99 誤差 < 基線校準後門檻。
