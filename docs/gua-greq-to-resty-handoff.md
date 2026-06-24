# Handoff: gua / gua-harden 的 greq → resty 遷移規格

> **STATUS: ✅ DONE.** gua-harden(`harden` 分支)與 gua(`greq-to-resty` 分支)均已完成,
> HTTP client 落在 `internal/httpclient`。本檔保留作歷史記錄。
> 給「正在重構 gua」的 session 執行。greq 與 requestwork.v2 已於 2026-06-24 封存,不再維護。
> 完整評估:`greq-requestwork-assessment.md`(同目錄)

## 0. 鐵則(先讀)

1. **這是一個獨立、atomic 的改動,不要混進結構重構。** 若你的重構會動到 HTTP 呼叫點(`cmd.go` / `delayquene/` / `node/cmd.go` / `grpc.go`),**先把重構跑綠,再做這個替換 follow-up**。混在一起 = 衝突 + 無法 review。
2. **先確認一件事再決定要不要保留全域併發上限**:gua 的 delay-queue 自己的 worker 是否已經限制併發?
   - 是 → requestwork 的 `requestwork.New(100)` 那個全域 cap 是多餘的,**直接丟,不用補 semaphore**。
   - 否 → 用 `golang.org/x/sync/semaphore`(或 `Transport.MaxConnsPerHost`)補,**不要**重建 worker pool。
3. 目標套件:**resty**(module `resty.dev/v3`)。下面 API 以 v3 為準,落地前對照 https://resty.dev 確認方法名。

## 1. 實際要替換的東西(很小)

gua/gua-harden 真正用到的 greq 方法只有三個:`PostRaw`、`Get`、`SetBasicAuth`。`PostRaw` 多半是打 webhook 回報。

**建構點(construction,改成建 resty client):**
| 檔案:行 | 現況 |
|---|---|
| `gua/cmd.go:315-316` | `work:=requestwork.New(100)` + `greq.New(work,60s,true)` |
| `gua/delayquene/quene.go:196-197` | 同上 |
| `gua/node/cmd.go:208-209` | `requestwork.New(5)` + `greq.New(...)` |
| `gua/benchmark/main.go:43-44` | 同上 |
| `gua-harden/delayquene/quene.go:186-187` | 同上 |
| `gua-harden/benchmark/main.go:43-44` | 同上 |

**型別欄位:** `gua/grpc.go:22`、`gua/delayquene/worker.go:38`、`gua-harden/delayquene/worker.go` 的 `httpClient *greq.Client` → 改 `*resty.Client`。

**方法呼叫點:** `worker.go` 的 `httpClient.PostRaw(...)` / `httpClient.Get(...)`、benchmark 的 `PostRaw`、`cmd.go:27` 的 `"set_basic_auth": client.SetBasicAuth`(注意:這是把方法當值塞進 map,簽章會變,要改 adapter)。

## 2. 設定工廠(共用、~30 行,不含任何 HTTP 邏輯)

新增 `internal/httpclient/httpclient.go`(gua 與 gua-harden 各一份,或抽共用):

```go
package httpclient

import (
	"time"
	"resty.dev/v3"
)

// New 回傳設好團隊預設的 resty client。只設定,不實作任何 HTTP 行為。
func New(timeout time.Duration, debug bool) *resty.Client {
	c := resty.New().
		SetTimeout(timeout).
		SetRetryCount(3).                       // greq 原本沒有 retry,webhook 升級點
		SetRetryWaitTime(200 * time.Millisecond).
		SetRetryMaxWaitTime(2 * time.Second).
		SetResponseBodyLimit(8 << 20)           // 8MiB,取代 greq 無上限的 OOM 風險
	if debug {
		c.SetDebug(true)        // 取代 greq 手刻的 slow-log;要時間拆解用 R().EnableTrace()
	}
	return c
}
```

> keep-alive 預設就是開的(別像 greq 那樣關掉);併發若確認需要全域上限,在這裡 `c.SetTransport(...)` 設 `MaxConnsPerHost`,或在呼叫端包 semaphore。

## 3. 方法對照(greq → resty v3)

greq 回傳 `(data []byte, status int, err error)`。resty 用 `resp.Body()` / `resp.StatusCode()`。

```go
// greq: data, status, err := client.Get(u.String(), nil)
resp, err := client.R().Get(u.String())
// data = resp.Body(); status = resp.StatusCode()

// greq: data, status, err := client.PostRaw(url, bodyReader)   // 預設 Content-Type: application/json
resp, err := client.R().
	SetHeader("Content-Type", "application/json").  // 對齊 greq PostRaw 的預設
	SetBody(bodyReader).
	Post(url)

// greq: client.SetBasicAuth(user, pass)   // 回傳 *greq.Client
client.SetBasicAuth(user, pass)            // resty 回傳 *resty.Client
```

**若想把 call site 改動降到最低**,可加一個 ~10 行 adapter,保留 `(data, status, err)` 三元組(只轉返回值,不碰 HTTP 機制),這樣 `worker.go` 的呼叫點幾乎不動:

```go
func Get(c *resty.Client, url string) ([]byte, int, error) {
	r, err := c.R().Get(url)
	if err != nil { return nil, 0, err }
	return r.Body(), r.StatusCode(), nil
}
func PostRaw(c *resty.Client, url string, body io.Reader) ([]byte, int, error) {
	r, err := c.R().SetHeader("Content-Type","application/json").SetBody(body).Post(url)
	if err != nil { return nil, 0, err }
	return r.Body(), r.StatusCode(), nil
}
```

> `cmd.go:27` 的 `"set_basic_auth": client.SetBasicAuth` 因回傳型別改變需配合調整(包成 `func(u,p string){ client.SetBasicAuth(u,p) }`)。

## 4. 收尾步驟

```bash
cd gua          # 與 gua-harden 各做一次
go get resty.dev/v3
# 改完上述呼叫點後:
go mod tidy
go mod vendor   # 兩個專案都是 vendored
go build ./...
go test -race ./...
```

- 確認 `go.mod` / `vendor/` 不再有 `github.com/syhlion/greq` 與 `.../requestwork.v2`。
- benchmark 程式同步改。

## 5. 驗收

- [ ] `go build ./...` 與 `go test -race ./...` 全綠(gua 與 gua-harden)。
- [ ] 不再 import greq / requestwork。
- [ ] webhook 回報路徑(`PostRaw`)有 retry。
- [ ] 回應有大小上限,不會無上限讀。
- [ ] keep-alive 生效(同 host 連線重用)。
- [ ] delay-queue 併發行為與替換前一致(若移除全域 cap,確認 queue 自身 worker 數已等價限流)。
