package delayquene

import (
	"encoding/json"
	"time"

	"github.com/gomodule/redigo/redis"
)

// HistoryEntry is one execution record kept for monitoring. Stored as JSON in a
// per-group ZSET scored by exec_time, pruned to the retention window on write.
type HistoryEntry struct {
	Seq             int64  `json:"seq"`
	JobId           string `json:"job_id"`
	GroupName       string `json:"group_name"`
	Type            string `json:"type"`
	PlanTime        int64  `json:"plan_time"`
	ExecTime        int64  `json:"exec_time"`
	FinishTime      int64  `json:"finish_time"`
	Success         bool   `json:"success"`
	Message         string `json:"message,omitempty"`
	Error           string `json:"error,omitempty"`
	ExecMachineHost string `json:"exec_machine_host"`
}

func historyKey(group string) string { return "GUA-HIST-" + group }

// History returns the most recent execution records for a group, newest first.
func (t *q) History(group string, limit int) (entries []*HistoryEntry, err error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	c := t.urpool.Get()
	defer c.Close()
	vals, err := redis.ByteSlices(c.Do("ZREVRANGE", historyKey(group), 0, limit-1))
	if err != nil {
		return nil, err
	}
	entries = make([]*HistoryEntry, 0, len(vals))
	for _, v := range vals {
		e := &HistoryEntry{}
		if json.Unmarshal(v, e) == nil {
			entries = append(entries, e)
		}
	}
	return entries, nil
}

// recordHistory appends one execution record and prunes anything older than the
// retention window. No-op when retention is disabled.
func (t *Worker) recordHistory(e *HistoryEntry) {
	if t.historyTTL <= 0 {
		return
	}
	c := t.urpool.Get()
	defer c.Close()
	if seq, serr := redis.Int64(c.Do("INCR", "GUA-HIST-SEQ")); serr == nil {
		e.Seq = seq
	}
	b, merr := json.Marshal(e)
	if merr != nil {
		return
	}
	key := historyKey(e.GroupName)
	cutoff := time.Now().Unix() - int64(t.historyTTL)
	c.Send("ZADD", key, e.ExecTime, b)
	c.Send("ZREMRANGEBYSCORE", key, 0, cutoff)
	c.Send("EXPIRE", key, t.historyTTL)
	c.Flush()
	for i := 0; i < 3; i++ {
		c.Receive()
	}
}
