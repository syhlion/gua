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
  - Job model:`request_url` 前綴 `(HTTP|REMOTE|LUA)@` → `(HTTP|GRPC)@`;`exec_cmd` → 通用 `payload`。
- **Monitor 範圍**:快照 + 歷史 + 工程 Web UI。歷史存 Redis + TTL(env 可調,3–5 天)。UI 只給工程 / API 驗證用。
- **壓測為必交付**:驗證並發任務數 + 執行時間誤差(p50/p95/p99/max)+ 完成率。

---

## 階段與 checklist

### Phase 0 — 決策閘門 + 工作基線
- [x] 開 `harden` worktree、落地計畫、推 remote
- [x] Go/No-Go 快評:**GO(條件式)**,前提 = 接受 at-least-once + 消費端冪等

### Phase 1 — 觸發模型重塑(瘦身 + 雙軌 gRPC)〔先做,刪掉的不該浪費力氣測〕
- [ ] **1a 移除 REMOTE/LUA**:刪 `node/`、`luacore/`、`luaweb/`、`apifunc.go`、func HTTP server(`LuaEntrance`)
- [ ] 清依賴:`gopher-lua`、`glua-libs`、`telegram-bot-api`、`go-sql-driver/mysql`(順帶清 dependabot 漏洞)
- [ ] `ExecuteJob` switch 收斂為 `HTTP` / `GRPC`;`UrlRe` 改 `^(HTTP|GRPC)@`
- [ ] **1b 新 proto**:
  - [ ] Admin/CRUD gRPC service(鏡像 HTTP:RegisterGroup / AddJob / Remove / Pause / Active / Edit / List / RegisterCallbackEndpoint)
  - [ ] Trigger gRPC service(gua→消費者 Push:`OnJobTrigger(job_id,name,group,plan_time,exec_time,payload,otp_code)→result`)
  - [ ] 移除舊 node 專屬 RPC(NodeRegister / RemoteCommand / JobReply / node Heartbeat)
- [ ] **1c** Job model:`exec_cmd` → `payload` 語意調整;callback endpoint 註冊(位址 + OTP)存 Redis(取代 REMOTE_NODE_*)

### Phase 2 — 測試骨架(地基)
- [ ] miniredis + 注入式 clock
- [ ] cron parser(`parser.go`/`spec.go`)單元測試
- [ ] `go test ./...` 全綠、可進 CI

### Phase 3 — 止血(panic / vet / leak)〔刪除後大幅縮水〕
- [ ] 殘餘 `FindStringSubmatch` nil guard + 入口驗證
- [ ] `context.WithTimeout` cancel 洩漏、`signal.Notify(SIGKILL)`、unreachable code
- [ ] `go vet ./...` 乾淨

### Phase 4 — delayquene 可測化
- [ ] `*redis.Pool` 收斂到薄 interface(可 mock / miniredis)
- [ ] 排程時間注入 clock
- [ ] 核心分支測試:到期推就緒、`@once` vs cron 重入桶、down-server 併桶接管、JobCheck 補單

### Phase 5 — ★ Monitor Tier 1:唯讀狀態 API
- [ ] 現況快照:job 狀態 / active / 下次執行時間
- [ ] 佇列深度(`LLEN GUA-READY-JOB`)、叢集槽位健康(`SERVER-*`)、delay 統計
- [ ] 純讀;同時作為 Phase 6 硬化的驗證工具

### Phase 6 — 分散式安全性硬化(最高風險)+ 壓測驗收
- [ ] `STARTLOCK`/`JOBCHECKLOCK` 換 TTL + owner token 鎖
- [ ] 修 `SERVER-N` 槽位 / snowflake 撞號窗口(租約 / fencing)
- [ ] ready-queue 冪等 / 去重
- [ ] gRPC 連線池化
- [ ] **通過壓測驗收門**

### Phase 7 — ★ Monitor Tier 2:執行歷史 + 工程 Web UI
- [ ] reply payload 落地 Redis(TTL,env 可調)
- [ ] 歷史查詢 API + 單頁工程 Web UI(兼 API 驗證台)

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
