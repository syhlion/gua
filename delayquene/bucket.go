package delayquene

import (
	"github.com/gomodule/redigo/redis"
)

type BucketItem struct {
	Timestamp int64
	JobId     string
}

type Bucket struct {
	rpool *redis.Pool
}

func (b *Bucket) Push(key string, timestamp int64, jobId string) (err error) {
	c := b.rpool.Get()
	defer c.Close()
	_, err = c.Do("ZADD", key, timestamp, jobId)
	return
}
func (b *Bucket) Get(key string) (items []*BucketItem, err error) {
	c := b.rpool.Get()
	defer c.Close()
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
