# gua harden — 計畫與進度

目標:把 gua 從 2019 個人專案,帶到可取代 AgOcean JobScheduler 的狀態。
路線:**可測 → 不 crash → 分散式可靠 → 可觀測 → 壓測驗收**。每階段是後一階段的安全網,不跳。

工作分支:`harden`(git worktree @ `../gua-harden`),進度推 `origin/harden`。

---

## 決策紀錄(Decisions)

- **破壞性變更可接受**:目前 JobScheduler 使用量少,相依者改接口可行 → 評估(Phase 6)力道從簡。
- **Monitor 範圍**:快照 + 歷史 + 工程 Web UI。
  - 歷史存 **Redis + TTL**(env 可調,保留 3–5 天),把現有 fire-and-forget 的 `loghook.Payload` 順手落地一份。
  - Web UI 只做**工程介面**,定位 = RD 排查 + API 驗證台,不開發給 end user。
- **壓測為必交付**:驗證並發任務數 + 任務執行時間誤差(p50/p95/p99/max)+ 完成率。

---

## 階段與 checklist

### Phase 0 — 決策閘門 + 工作基線
- [x] 開 `harden` worktree、落地本計畫、推 remote
- [ ] Go/No-Go 快評(半頁,從現有事實)

### Phase 1 — 測試骨架(地基)
- [ ] 引入 miniredis + 注入式 clock
- [ ] cron parser(`parser.go`/`spec.go`)單元測試(最高 CP 值,純函式)
- [ ] 修掉讓 `node` 套件 `go test` build failed 的 vet printf 問題
- [ ] `go test ./...` 全綠、可進 CI

### Phase 2 — 止血(panic / vet / leak)
- [ ] `UrlRe.FindStringSubmatch` 全部 nil guard + `httpv1` 入口驗證(配回歸測試)
- [ ] 移除 `quene.go:430/608` debug `fmt.Println`(順手解 lock-copy)
- [ ] 修 `context.WithTimeout` cancel 洩漏(worker/quene/node/apifunc)
- [ ] 拿掉無效的 `signal.Notify(SIGKILL)`、`RunJobCheck` unreachable code
- [ ] `go vet ./...` 乾淨

### Phase 3 — delayquene 可測化
- [ ] `*redis.Pool` 收斂到薄 interface(可 mock / miniredis)
- [ ] 排程時間注入 clock
- [ ] 核心分支測試:到期推就緒、`@once` vs cron 重入桶、down-server 併桶接管、JobCheck 補單

### Phase 3.5 — ★ Monitor Tier 1:唯讀狀態 API
- [ ] 現況快照:job 狀態 / active / 下次執行時間
- [ ] 佇列深度(`LLEN GUA-READY-JOB`)、叢集槽位健康(`SERVER-*` 心跳)、delay 統計
- [ ] 純讀、不碰寫路徑;同時作為 Phase 4 的驗證工具

### Phase 4 — 分散式安全性硬化(最高風險)
- [ ] `STARTLOCK`/`JOBCHECKLOCK` 換成 TTL + owner token 鎖
- [ ] 修 `SERVER-N` 槽位 / snowflake 撞號窗口(租約 / fencing)
- [ ] ready-queue 冪等 / 去重策略
- [ ] gRPC 連線池化(去掉每次執行 Dial+Close)
- [ ] **通過壓測驗收門(見下)**

### Phase 5 — ★ Monitor Tier 2:執行歷史 + 工程 Web UI
- [ ] reply-hook payload 落地 Redis(TTL,env 可調,預設 3–5 天)
- [ ] 歷史查詢 API(成功/失敗、延遲、最後結果)
- [ ] 單頁工程 Web UI(打狀態 + 歷史 API,兼 API 驗證台)

### Phase 6 — 取代評估 + 遷移計畫(從簡)
- [ ] gua vs JobScheduler 精簡功能對位 + 缺口
- [ ] 遷移風險、相依者改接口清單、上線/回退

---

## 壓測方案(Stress / Timing Accuracy)

延伸現有 `benchmark/`(已會丟 1500 個 `@once` LUA job @ now+30s,callback 記 `actual - scheduled` timeout)。

**量測指標**
- 任務執行時間誤差 `actualExecTime - planTime` 的分佈:**p50 / p95 / p99 / max**
- 超時比例(> 1s、> 2s)
- **完成率**:漏觸發數、重複觸發數
- 佇列堆積(ready queue depth 隨時間)

**量測維度**
- 並發(同一秒到期 job 數):100 / 1k / 5k / 10k
- 系統存量(常駐 active job 總數):1k / 10k / 50k
- 觸發型別:LUA(in-process,最快)/ HTTP / REMOTE(走 gRPC,最慢)分開測
- 叢集規模:1 台 vs 3 台(驗接管、負載分散、重複觸發)

**已知誤差地板**
- 解析度受 **700ms bucket ticker** 限制:無載下單任務誤差約 `0–700ms + 執行延遲`。
- 壓測重點 = 尾端(p99/max)在壓力下會長到多大;若需 sub-second 精度,700ms ticker 是設計改點。

**驗收門(Phase 4 acceptance,基線後校準具體數字)**
- 漏觸發 = 0
- 重複觸發在冪等保護下無副作用
- p99 誤差 < 校準後門檻
