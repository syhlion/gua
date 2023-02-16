package delayquene

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/sirupsen/logrus"
	guaproto "github.com/syhlion/gua/proto"
	"google.golang.org/protobuf/proto"
)

type BucketItem struct {
	Timestamp int64
	JobId     string
}

type Bucket struct {
	lock     *sync.RWMutex
	stopFlag int
	rpool    *redis.Pool
	logger   *logrus.Logger
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
		_, err = c.Do("DEL", "JOBCHECKLOCK")
		if err != nil {
			b.logger.WithError(err).Error("DEL JOBCHECK error")
		}
		c.Close()
		return
	}()
	var i = 0
	var check = 0
	t := time.NewTimer(1 * time.Second)
	for {
		//搶鎖 & 上鎖
		check, err = redis.Int(c.Do("SETNX", "JOBCHECKLOCK", 1))
		if err != nil {
			return err
		}
		if check == 1 || i >= 20 {
			break
		}
		<-t.C
		t.Reset(1 * time.Second)
		i++
	}
	jobCheckStart := time.Now()

	//先檢查是否有有JOB 但沒有scan的JOB

	replysJob, err := RedisScan(c, "JOB-*")
	var args []interface{}
	for _, v := range replysJob {
		if jobRe.MatchString(v) {
			//先蒐集符合的key
			args = append(args, v+"-scan")

		}
	}
	//透過MGET 一次把資料拉出來
	values, err := redis.Strings(c.Do("MGET", args...))
	for i, v := range values {
		if v != "" {
			//用pipline的方式 更新時間
			c.Send("SET", args[i], now.Unix())
		} else {
			b.logger.Errorf("job miss scan job %s", v+"-scan")
		}
	}
	c.Flush()

	replys, err := RedisScan(c, "JOB-*-scan")
	if err != nil {
		return err
	}
	var scanJob []interface{}
	var job []interface{}
	var jobName []interface{}
	//檢查是否有多餘的點查並且刪除
	for _, v := range replys {
		t := strings.TrimSuffix(v, "-scan")
		job = append(job, t)
		scanJob = append(scanJob, v)
		ss := jobCheckRe.FindStringSubmatch(v)
		jobName = append(jobName, "JOB"+"-"+ss[1]+"-"+ss[2])
	}
	jobs, err := redis.ByteSlices(c.Do("MGET", job...))
	if err != nil {
		return err
	}
	for _, v := range jobs {
		if v == nil {
			c.Send("DEL", v)
		} else {
			b.logger.Errorf("job miss main job %s", t)
		}
	}
	c.Flush()

	scanJobTime, err := redis.Int64s(c.Do("MGET", scanJob...))
	if err != nil {
		return err
	}
	jobDatas, err := redis.ByteSlices(c.Do("MGET", jobName...))
	if err != nil {
		return err
	}
	for i, v := range scanJobTime {
		tlastTime := time.Unix(v, 0)
		if now.Sub(tlastTime) > 2*time.Minute {

			jb := &guaproto.Job{}
			err = proto.Unmarshal(jobDatas[i], jb)
			if err != nil {
				b.logger.WithError(err).Error("jobCheck job unmarshal error")
				return err
			}
			//任務是Active 才進行補任務
			if jb.Active {
				err = b.Push(key, 0, jb.Id)
				if err != nil {
					b.logger.Error("job miss but auto patch job error", jb.Id, err)
					return err
				}
				b.logger.Error("job miss and auto patch job ", jb.Id)
			} else {

				t := time.Now().Add(72 * time.Hour)
				_, err = c.Do("SET", jb.Id+"-scan", t.Unix())
				if err != nil {
					b.logger.WithError(err).Error("jobcheck set job-scan error")
				}
				b.logger.Error("jobcheck is not active SET time %v", t)
			}
		}
	}

	jobCheckEnd := time.Now()
	b.logger.WithFields(logrus.Fields{
		"jobcheck_start_lock":  now,
		"jobcheck_start":       jobCheckStart,
		"jobcheck_end":         jobCheckEnd,
		"jobcheck_total":       fmt.Sprintf("duration: %v", jobCheckEnd.Sub(jobCheckStart)),
		"jobcheck_exec_host":   machineHost,
		"jobcheck_exec_repeat": i,
	}).Info("jobcheck")
	return
}
func (b *Bucket) Get(key string) (items []*BucketItem, err error) {
	c := b.rpool.Get()
	t := time.Now().Unix()
	defer func() {
		c.Close()
		b.lock.RLock()
		defer b.lock.RUnlock()
		if b.stopFlag == 1 {
			return
		}
		//檢查是否有其他server 遺落的任務 & 對於經過的任務增加檢核時間點
		go func() {
			cc := b.rpool.Get()
			defer cc.Close()
			for _, i := range items {
				c.Send("SET", i.JobId+"-scan", t)

			}
			downBucket, err := redis.String(cc.Do("LPOP", "down-server"))
			if err != nil {
				c.Close()
				return
			}
			b.logger.Infof("GET New Server:%s Merge to %s Start", downBucket, key) //經過的job 增加 job-scan 時間
			cc.Send("ZUNIONSTORE", key, 2, key, downBucket, "WEIGHTS", 1, 1, "AGGREGATE", "MIN")
			cc.Send("DEL", downBucket)
			cc.Flush()
			b.logger.Infof("GET New Server:%s Merge to %s Finish", downBucket, key)
		}()
	}()

	reply, err := redis.Values(c.Do("ZRANGE", key, 0, -1, "WITHSCORES"))
	if err != nil {
		return
	}
	items = make([]*BucketItem, 0)
	for i := 0; i < len(reply); i += 2 {

		item := &BucketItem{}
		kkey, err := redis.String(reply[i], nil)
		if err != nil {
			return nil, err
		}

		//把任務解析成 item worker 執行
		value, err := redis.Int64(reply[i+1], nil)
		if err != nil {
			return nil, err
		}
		item.JobId = kkey
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
func (b *Bucket) RemoveAndPush(removeKey string, puahKey string, jobId string, timestamp int64) (err error) {
	c := b.rpool.Get()
	defer c.Close()
	c.Send("ZREM", removeKey, jobId)
	c.Send("ZADD", puahKey, timestamp, jobId)
	c.Flush()
	_, err = c.Receive()
	if err != nil {
		b.logger.Errorf("bucket push error %s", err)
	}
	_, err = c.Receive()
	if err != nil {
		b.logger.Errorf("bucket push error %s", err)
	}
	return
}
func (b *Bucket) Close() {
	b.lock.Lock()
	b.stopFlag = 1
	b.lock.Unlock()
}
