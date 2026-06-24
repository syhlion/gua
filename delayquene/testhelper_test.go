package delayquene

import (
	"io"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	guaproto "github.com/syhlion/gua/proto"
)

func testLogger() *logrus.Logger {
	l := logrus.New()
	if os.Getenv("GUA_TEST_LOG") != "" {
		l.SetOutput(os.Stderr)
		l.SetLevel(logrus.DebugLevel)
		return l
	}
	l.SetOutput(io.Discard)
	l.SetLevel(logrus.PanicLevel)
	return l
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
