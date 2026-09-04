// Package httpclient builds the resty client used for HTTP deliveries and a
// tiny (data, status, err) adapter around it.
package httpclient

import (
	"context"
	"net/http"
	"time"

	"resty.dev/v3"
)

// maxConnsPerHost bounds in-flight requests to any single consumer host, so a
// burst of deliveries (River dispatches them concurrently) can't open unbounded
// connections to one consumer.
const maxConnsPerHost = 100

// New returns a configured resty client. maxTimeout is the hard cap on any
// request; the per-delivery timeout comes from the context passed to PostRaw.
//
// No resty-level retry is configured: resty never retries a POST (non
// idempotent) and River already retries a failed delivery with backoff.
func New(maxTimeout time.Duration, debug bool) *resty.Client {
	transport := &http.Transport{
		MaxConnsPerHost:     maxConnsPerHost,
		MaxIdleConns:        maxConnsPerHost,
		MaxIdleConnsPerHost: maxConnsPerHost,
		IdleConnTimeout:     90 * time.Second,
	}
	c := resty.New().
		SetTimeout(maxTimeout).
		SetTransport(transport).
		SetResponseBodyLimit(8 << 20) // 8MiB cap on what we read back
	if debug {
		c.SetDebug(true)
	}
	return c
}

// PostRaw posts body as application/json and returns (data, status, err). The
// request is bound to ctx, so the caller's deadline/cancellation applies.
func PostRaw(ctx context.Context, c *resty.Client, url string, body []byte) ([]byte, int, error) {
	r, err := c.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(body).
		Post(url)
	if err != nil {
		return nil, 0, err
	}
	return r.Bytes(), r.StatusCode(), nil
}
