package delayquene

import (
	"strings"
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
	if err != nil {
		b.logger.Errorf("bucket push error %s", err)
	}
	return
}
func (b *Bucket) JobCheck(key string, now time.Time, machineHost string) (err error) {
	c := b.rpool.Get()
	//redis lock 確保同時間只有一台執行
	defer func() {
		c.Do("DEL", "JOBCHECKLOCK")
		c.Close()
		return
	}()
	var i = 0
	var check = 0
	for {
		//搶鎖 & 上鎖
		check, err = redis.Int(c.Do("SETNX", "JOBCHECKLOCK", 1))
		if err != nil {
			return err
		}
		if check == 1 || i >= 20 {
			break
		}
		time.Sleep(1 * time.Second)
		i++
	}

	b.logger.Infof("jobcheck start from %s. worker start at %s jobecheck exec %s. repeat %d", machineHost, now, time.Now(), i)

	//先檢查是否有有JOB 但沒有scan的JOB

	replysJob, err := RedisScan(c, "JOB-*")
	for _, v := range replysJob {
		if jobRe.MatchString(v) {
			_, err := redis.Int64(c.Do("GET", v+"-scan"))
			if err == redis.ErrNil {
				_, err := c.Do("SET", v+"-scan", 0)
				if err != nil {
					b.logger.Error("redis set scan error", err)
				}
				continue
			}
		}
	}

	replys, err := RedisScan(c, "JOB-*-scan")
	if err != nil {
		return err
	}
	for _, v := range replys {
		//檢查是否有多餘的點查並且刪除除除除除除
		t := strings.Trim(v, "-scan")
		_, err := redis.Bytes(c.Do("GET", t))
		if err == redis.ErrNil {
			_, err := c.Do("DEL", v)
			if err != nil {
				b.logger.Error("job miss main job redis error:", err)
			}
			b.logger.Errorf("job miss main job %s", t)
			continue
		}
		//開始檢查是否有遺失的任務
		lastTime, err := redis.Int64(c.Do("GET", v))
		if err != nil {
			return err
		}
		ss := jobCheckRe.FindStringSubmatch(v)

		tlastTime := time.Unix(lastTime, 0)
		if now.Sub(tlastTime) > 2*time.Minute {
			err = b.Push(key, 0, "JOB"+"-"+ss[1]+"-"+ss[2])
			if err != nil {
				b.logger.Error("job miss but auto patch job error", "JOB"+"-"+ss[1]+"-"+ss[2], err)
				return err
			}
			b.logger.Error("job miss and auto patch job ", "JOB"+"-"+ss[1]+"-"+ss[2])

		}

	}
	b.logger.Infof("jobcheck finish at %s from %s", time.Now(), machineHost)
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

	reply, err := redis.Values(c.Do("ZRANGE", key, 0, -1, "WITHSCORES"))
	if err != nil {
		return
	}
	items = make([]*BucketItem, 0)
	t := time.Now().Unix()
	for i := 0; i < len(reply); i += 2 {
		item := &BucketItem{}
		key, err := redis.String(reply[i], nil)
		if err != nil {
			return nil, err
		}
		//有經過的任務，增加任務時間
		_, err = c.Do("SET", key+"-scan", t)
		if err != nil {
			return nil, err
		}
		//把任務解析成 item worker 執行
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
