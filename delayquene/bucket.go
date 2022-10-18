package delayquene

import (
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/sirupsen/logrus"
)

type BucketItem struct {
	Timestamp int64
	JobId     string
}

type Bucket struct {
	rpool  *redis.Pool
	logger *logrus.Logger
}

func (b *Bucket) Push(key string, timestamp int64, jobId string) (err error) {
	c := b.rpool.Get()
	defer c.Close()
	_, err = c.Do("ZADD", key, timestamp, jobId)
	return
}
func (b *Bucket) JobCheck(key string, now time.Time) (err error) {
	c := b.rpool.Get()
	defer c.Close()
	replys, err := redis.Strings(c.Do("KEYS", "JOB-*-*-scan"))
	if err != nil {
		return err
	}
	for _, v := range replys {
		lastTime, err := redis.Int64(c.Do("GET", v))
		if err != nil {
			return err
		}
		ss := jobCheckRe.FindStringSubmatch(v)

		tlastTime := time.Unix(lastTime, 0)
		if now.Sub(tlastTime) > 1*time.Minute {
			err = b.Push(key, 0, "JOB"+"-"+ss[1]+"-"+ss[2])
			if err != nil {
				b.logger.Error("job miss but auto patch job error", "JOB"+"-"+ss[1]+"-"+ss[2], err)
				return err
			}
			b.logger.Error("job miss and auto patch job ", "JOB"+"-"+ss[1]+"-"+ss[2])

		}

	}
	return
}
func (b *Bucket) Get(key string) (items []*BucketItem, err error) {
	c := b.rpool.Get()
	defer func() {
		downBucket, err := redis.String(c.Do("LPOP", "down-server"))
		if err != nil {
			c.Close()
			return
		}
		b.logger.Infof("GET New Server:%s Merge to %s Start", downBucket, key)
		_, err = c.Do("ZUNIONSTORE", key, 2, key, downBucket, "WEIGHTS", 1, 1, "AGGREGATE", "MIN")
		if err != nil {
			c.Close()
			return
		}
		c.Do("DEL", downBucket)
		c.Close()
		b.logger.Infof("GET New Server:%s Merge to %s Finish", downBucket, key)
	}()

	reply, err := redis.Values(c.Do("ZRANGE", key, 0, 30, "WITHSCORES"))
	if err != nil {
		return
	}
	items = make([]*BucketItem, 0)
	for i := 0; i < len(reply); i += 2 {
		item := &BucketItem{}
		key, err := redis.String(reply[i], nil)
		if err != nil {
			return nil, err
		}
		value, err := redis.Int64(reply[i+1], nil)
		if err != nil {
			return nil, err
		}
		item.JobId = key
		if item.JobId == "" {
			continue
		}
		item.Timestamp = value
		items = append(items, item)
	}
	return

}
func (b *Bucket) Remove(key string, jobId string) (err error) {
	c := b.rpool.Get()
	defer c.Close()
	_, err = c.Do("ZREM", key, jobId)
	return

}
