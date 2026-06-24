// Package httpclient builds the team-default resty client and a tiny adapter
// that keeps greq's (data, status, err) call shape, so call sites barely change.
//
// Replaces github.com/syhlion/greq + requestwork.v2 (archived 2026-06-24).
package httpclient

import (
	"net/http"
	"time"

	"resty.dev/v3"
)

// maxConnsPerHost bounds in-flight requests to any single consumer host. The
// delay-queue spawns a goroutine per firing (unbounded), so this is what caps
// HTTP delivery concurrency (replacing requestwork's old global cap of 100).
const maxConnsPerHost = 100

// New returns a configured resty client. Configuration only — no HTTP logic.
func New(timeout time.Duration, debug bool) *resty.Client {
	transport := &http.Transport{
		MaxConnsPerHost:     maxConnsPerHost,
		MaxIdleConns:        maxConnsPerHost,
		MaxIdleConnsPerHost: maxConnsPerHost,
		IdleConnTimeout:     90 * time.Second,
		// keep-alive stays on (greq used to disable it)
	}
	c := resty.New().
		SetTimeout(timeout).
		SetTransport(transport).
		SetRetryCount(3). // greq had none; retries network failures (not 5xx)
		SetRetryWaitTime(200 * time.Millisecond).
		SetRetryMaxWaitTime(2 * time.Second).
		SetResponseBodyLimit(8 << 20) // 8MiB cap; greq read unbounded (OOM risk)
	if debug {
		c.SetDebug(true)
	}
	return c
}

// PostRaw posts body as application/json and returns (data, status, err),
// matching greq.Client.PostRaw. body is a []byte so resty can buffer/retry it
// natively (passing an io.Reader would defeat retry and add a copy).
func PostRaw(c *resty.Client, url string, body []byte) ([]byte, int, error) {
	r, err := c.R().
		SetHeader("Content-Type", "application/json").
		SetBody(body).
		Post(url)
	if err != nil {
		return nil, 0, err
	}
	return r.Bytes(), r.StatusCode(), nil
}

// Get returns (data, status, err), matching greq.Client.Get.
func Get(c *resty.Client, url string) ([]byte, int, error) {
	r, err := c.R().Get(url)
	if err != nil {
		return nil, 0, err
	}
	return r.Bytes(), r.StatusCode(), nil
}
