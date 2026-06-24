package delayquene

import (
	"io"
	"log/slog"
	"os"
	"time"

	guaproto "github.com/syhlion/gua/proto"
)

func testLogger() *slog.Logger {
	if os.Getenv("GUA_TEST_LOG") != "" {
		return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

func httpJob(group, id, target, payload, interval string) *guaproto.Job {
	return &guaproto.Job{
		Name:            id,
		GroupName:       group,
		Id:              id,
		Exectime:        time.Now().Add(1 * time.Second).Unix(),
		IntervalPattern: interval,
		RequestUrl:      "HTTP@" + target,
		Payload:         payload,
		Active:          true,
	}
}
