package delayquene

import (
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
