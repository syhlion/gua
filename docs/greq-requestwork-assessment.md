# greq + requestwork.v2 評估報告與重構計畫

> 範圍：`/home/scott/dev/greq`、`/home/scott/dev/requestwork.v2`
> 消費者：`gua`、`gua-harden`（皆 vendored，皆個人專案）
> 日期：2026-06-24

---

## 0. 結論（TL;DR）

1. **`requestwork.v2`（worker pool）是畫蛇添足，建議整包移除**，把「全域併發上限」這個唯一有價值的語意以 ~5 行 semaphore 折進 greq。
   - 決定性證據：workspace 內 **沒有任何地方直接用 `requestwork`**（`work.Execute` / `SetTransport` 0 次呼叫），它 100% 只是被 `greq.New` 吞進去的參數。一個沒有獨立使用者的抽象層 = 純indirection。
2. **`greq` 留下、但要修**：1 個 library 大忌（劫持全域 logger）、2 個真 bug（吞錯、raw debug 讀空）、1 個效能反模式（強制關 keep-alive）、大量重複碼。
3. 消費者改動極小：6 個 `greq.New(...)` 呼叫點做機械式替換，re-vendor 即可。實際用到的 greq 方法只有 `PostRaw` / `Get` / `SetBasicAuth`，相容風險低。

---

## 1. 現況架構

```
gua / gua-harden
  └─ work := requestwork.New(100)         // 開 100 個 worker goroutine + unbuffered chan
     client := greq.New(work, 60s, true)  // greq 把每個 request 丟進 worker.Execute
        └─ worker.Execute(req, handler)
             └─ run(): 再開一個 inner goroutine 做 client.Do，select{ ctx.Done / 結果 }
```

- `requestwork.v2`：用 N 個 worker goroutine 從 unbuffered channel 取 job，目的是「最多 N 個 request 同時在跑」。
- `greq`：REST 便利層（Get/Post/Put/Delete × Raw × WithOnceHeader 矩陣 + debug slow-log + gzip 處理），把 request 透過 worker 執行。

---

## 2. 問題清單

### 2.1 requestwork.v2

| # | 嚴重度 | 位置 | 問題 |
|---|---|---|---|
| R1 | 🔴 設計 | 全包 | **多此一舉**。「限制併發數」`Transport.MaxConnsPerHost`（per-host）或一個 buffered channel semaphore（全域）原生就能做，且無需每個 in-flight request 多開 goroutine。 |
| R2 | 🔴 效能 | `worker.go:33` `DisableKeepAlives:true` + greq `req.Close=true` | **每次呼叫新開 TCP+TLS、用完即關**。連線池（`DefaultMaxIdleConnPerHost=20`、`IdleConnTimeout`）全成死碼。同時付「養 goroutine pool」+「每次重做 TLS handshake」兩邊成本，標準庫好處一個沒拿到。 |
| R3 | 🟠 race | `worker.go:89-95` | timeout 走 `ctx.Done()` 就回傳 `Execute`，但 inner goroutine 的 `handler` 仍在跑 → greq 端已開始讀 `body/status`，handler 可能還在寫 → `-race` 可重現的 data race。 |
| R4 | 🟡 死碼 | `worker.go:23` | `DefaultMaxIdleConnPerHost` 宣告但無人用。 |
| R5 | 🟡 文件 | `README.md` | 範例 `a.Execute(ctx, req, ...)` 與實際簽章 `Execute(req, h)` 不符（ctx 實際從 `req.Context()` 取）。 |

### 2.2 greq

| # | 嚴重度 | 位置 | 問題 |
|---|---|---|---|
| G1 | 🔴 大忌 | `client.go:81-87` `init()` | 在 package init 改 **全域 logrus std logger**（`SetOutput/SetFormatter/SetLevel(Debug)`）。任何 import greq 的 app 都被強制 JSON + Debug level。Library 絕不該動全域 logger。`getExternalIP()` 也在 import 時做網卡列舉。 |
| G2 | 🔴 bug | `client.go:470-472` `ResolveRequest` | 參數 `e error` 被無視，body 傳的是還沒賦值的具名回傳 `err`（nil）→ 傳入的錯誤被靜默吞掉。應傳 `e`。 |
| G3 | 🟠 bug | `client.go:282` raw debug | `buf.ReadFrom(bb)` 讀的是已被 `Do` 消耗的同一 reader → `stat.Param` 永遠空。 |
| G4 | 🟠 死碼/誤導 | `client.go:344,442,503` | 手動 gzip 解碼幾乎進不來：未自設 `Accept-Encoding` 時 Go transport 自動 gzip 並解壓、且把 `Content-Encoding` header 拿掉，switch 永遠落 default。只有呼叫端手動設 Accept-Encoding 才走得到。 |
| G5 | 🟠 風險 | `client.go:351,356,449,454,510,515` | `ioutil.ReadAll` **無回應大小上限**。中間層打外部平台，巨大回應 = OOM 風險（對照 memory `api-inbound-169220804-xorcipher-oom`）。 |
| G6 | 🟡 API | `client.go:110` `CheckRedircet` | 公開方法拼錯字（Redirect→Redircet）。 |
| G7 | 🟡 重複 | `resolveRequest`/`resolveRawRequest`/`ResolveTraceRequest` | 三份 95% 相同；外加 Get/Post/Put/Delete × Raw × WithOnceHeader 方法矩陣，可收斂成一個核心 `do()` + options。 |
| G8 | 🟡 現代化 | 多處 | `ioutil.ReadAll`→`io.ReadAll`；`strings.Title`（client.go:126，Go1.18 deprecated）；README `Post(...)` 範例簽章對不上。 |
| G9 | 🟡 race | debug 模式 `t3/t4/t5` | httptrace 回呼於 http 棧 goroutine 寫，deferred func 讀；配合 R3 timeout 路徑可race（僅 debug）。 |

---

## 3. 畫蛇添足判定與方案選擇

**判定**：requestwork.v2 = 畫蛇添足。理由：(a) 無獨立消費者；(b) 它要的併發上限標準庫已提供；(c) 它的實作反而引入額外 goroutine、race、並關掉 keep-alive。

### 方案矩陣

| 方案 | 作法 | 消費者改動 | 推薦 |
|---|---|---|---|
| **A** | 直接拔 requestwork，greq 用裸 `http.Client`，**不保留**全域併發上限 | 改 6 個 New 呼叫點 | 次選（若確定不需要全域上限） |
| **B** ⭐ | **拔 requestwork**，把「全域併發上限」以 semaphore（`chan struct{}`）折進 greq；keep-alive 打開；單一 `do()` | 改 6 個 New 呼叫點 + re-vendor | **首選** |
| C | 保留 requestwork 但重寫成正確的 semaphore limiter（開 keep-alive、修 race），greq 簽章不動 | 只 re-vendor | 保守備案（churn 最小，但留一個沒人單獨用的 repo） |

> 推薦 **B**：拿掉一整個 repo 的維護負擔，保留 gua 在意的「全域 100 併發」語意，同時把 keep-alive、大小上限、單一 code path 一次到位。

---

## 4. 目標設計（方案 B）

### 4.1 新 greq.Client

```go
type Client struct {
    hc      *http.Client
    sem     chan struct{}      // 全域併發上限；nil = 不限
    timeout time.Duration
    headers map[string]string
    host    string
    lock    sync.RWMutex
    log     *logrus.Logger     // 自己的 instance，不碰全域
    debug   bool
    maxBody int64              // 回應大小上限，0 = 不限
}
```

### 4.2 新建構子（functional options）

```go
func New(opts ...Option) *Client            // 無 requestwork 依賴

// 範例
greq.New(
    greq.WithConcurrency(100),       // 取代 requestwork.New(100)
    greq.WithTimeout(60*time.Second),
    greq.WithDebug(true),
    greq.WithMaxBodyBytes(8<<20),    // 8MiB 上限
)
```

- Transport：開 keep-alive、設 `MaxIdleConnsPerHost`、移除 `DisableKeepAlives`；greq 不再 `req.Close=true`。
- 併發上限：`do()` 進入時 `sem <- struct{}{}`，defer `<-sem`（nil 則略過）。取代整個 worker pool。

### 4.3 單一核心

```go
func (c *Client) do(req *http.Request, debugParam string) (data []byte, status int, err error)
```

`Get/PostRaw/...` 全部薄薄包一層組 `*http.Request` 後呼叫 `do()`。`resolveRequest`/`resolveRawRequest`/`ResolveTraceRequest`/`ResolveRequest` 統一收斂；舊方法名保留為相容 shim（內部轉呼 `do`），避免動到消費者。

### 4.4 連帶修掉

- G1：改用 `logrus.New()` 私有 instance（或 `WithLogger` 注入），移除 init 副作用。
- G2：相容保留的 `ResolveRequest` 正確傳 `e`。
- G3/G4：debug param 改傳已知字串、移除無效 gzip 分支（或僅在自設 Accept-Encoding 時保留）。
- G5：`io.LimitReader` 包住 body 讀取。
- G6：新增正名 `CheckRedirect`，舊 `CheckRedircet` 標 `// Deprecated:` 轉呼新名。
- G8：`io.ReadAll`、移除 `strings.Title`、修 README。

---

## 5. 分階段計畫

### Phase 0 — 安全網（先做，避免回歸）
- 為 greq 補測試：Get/PostRaw 正常流、timeout、gzip 回應、>maxBody 截斷、SetBasicAuth/SetHeader、併發上限（N 個同時打、觀察 in-flight ≤ N）。
- 用 `httptest.Server` 取代現有打 `tw.yahoo.com` 的真實網路測試（requestwork 的 worker_test 同樣外網依賴，順手換掉）。
- `go test -race ./...` 設為通過門檻。

### Phase 1 — greq 內修（不動簽章，低風險先落地）
- G1 logger 劫持、G2 吞錯、G5 大小上限、G6 拼字、G8 現代化。
- 此階段消費者 re-vendor 即可，行為相容。

### Phase 2 — 折入 limiter、拔 requestwork（方案 B 主體）
- greq 內建 transport + semaphore，新增 `New(opts...)` 與 Option 集。
- 保留舊 `New(worker, timeout, debug)` 為 **deprecated 相容多載**？Go 不支援多載 → 改法：
  - 將舊 `New` 更名/保留，但其 `*requestwork.Worker` 參數無法去依賴。
  - 因此 Phase 2 必然改簽章 → 同步改 6 個消費者呼叫點（見 §6）。
- 收斂 `do()`，三個 resolve 函式合一，移除 G3/G4/G9。
- 刪除 `requestwork.v2` 依賴（go.mod、vendor）。

### Phase 3 — 消費者遷移
- `gua`、`gua-harden`：替換 6 處 `requestwork.New(N)` + `greq.New(work, …)` → `greq.New(greq.WithConcurrency(N), …)`。
- `go mod tidy && go mod vendor`，`go build ./... && go test -race ./...`。
- benchmark 程式同步更新。

### Phase 4 — requestwork.v2 收尾
- 確認無其他外部 repo 依賴後，repo README 標 archived / deprecated（指向 greq 內建 limiter）。
- 不刪 git 歷史，僅停止維護。

---

## 6. 影響面（Blast Radius）

**消費者呼叫點（全部 vendored，需 re-vendor）：**

| 檔案 | 行 |
|---|---|
| `gua/cmd.go` | 315-316 |
| `gua/delayquene/quene.go` | 196-197 |
| `gua/node/cmd.go` | 208-209 |
| `gua/benchmark/main.go` | 43-44 |
| `gua-harden/delayquene/quene.go` | 186-187 |
| `gua-harden/benchmark/main.go` | 43-44 |

**實際使用到的 greq 方法**（相容性只需保住這些）：`PostRaw`、`Get`、`SetBasicAuth`。其餘方法矩陣可安全重構/標 deprecated。

**requestwork 直接使用**：0 處（僅作為 greq.New 參數）。→ 拔除安全。

---

## 7. 風險與緩解

| 風險 | 緩解 |
|---|---|
| 打開 keep-alive 改變對端連線行為（部分老舊對端對長連線敏感） | 提供 `WithDisableKeepAlive()` 逃生開關；預設開、可關。 |
| 全域併發上限語意從「worker pool」換成「semaphore」邊界差異 | semaphore 在 `do()` 入口擋，語意等價於「最多 N 個 request 同時 in-flight」，與原意一致；用併發測試驗證。 |
| 改簽章漏改消費者 → 編譯失敗 | 只有 6 點，編譯器會抓；CI `go build ./...`。 |
| debug slow-log 行為變動 | 保留 >2s slow log 與欄位；以測試固定輸出格式。 |

---

## 8. 驗收標準

- [ ] `go test -race ./...` 在 greq、gua、gua-harden 全綠。
- [ ] greq 不再 import `requestwork.v2`；go.mod / vendor 清乾淨。
- [ ] import greq 不再改動呼叫端的全域 logrus 設定。
- [ ] 回應 > maxBody 被截斷且回明確錯誤，不 OOM。
- [ ] 併發測試：同時 100+ 請求，in-flight 數 ≤ 設定上限。
- [ ] keep-alive 生效（同 host 連續請求重用連線，可用 httptrace `GotConn.Reused` 驗）。
- [ ] gua / gua-harden 6 呼叫點遷移、build 通過。

---

## 9. 工作量粗估

| Phase | 內容 | 估時 |
|---|---|---|
| 0 | 測試安全網（httptest） | 0.5d |
| 1 | greq 內修（logger/吞錯/大小上限/現代化） | 0.5d |
| 2 | 折 limiter、拔 requestwork、收斂 do() | 1d |
| 3 | 消費者遷移 + re-vendor + 測試 | 0.5d |
| 4 | requestwork archived 收尾 | 0.2d |

> 合計約 2.5–3 人日（含測試）。Phase 1 可獨立先上，立即消除最高風險（logger 劫持、吞錯）。
