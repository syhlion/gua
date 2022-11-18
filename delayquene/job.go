package delayquene

import (
	"errors"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/gomodule/redigo/redis"
	guaproto "github.com/syhlion/gua/proto"
)

type JobQuene struct {
	rpool *redis.Pool
}

func (j *JobQuene) Get(key string) (jb *guaproto.Job, err error) {
	c := j.rpool.Get()
	defer c.Close()
	reply, err := redis.Bytes(c.Do("GET", key))
	if err != nil {
		return
	}
	jb = &guaproto.Job{}
	err = proto.Unmarshal(reply, jb)
	return
}

func (j *JobQuene) Add(key string, jb *guaproto.Job) (err error) {
	b, err := proto.Marshal(jb)
	if err != nil {
		return
	}
	c := j.rpool.Get()
	//redis lock 確保同時間 jobcheck跟 add job 不會同時進行
	defer func() {
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
		if i >= 25 {
			err = errors.New("add joblock error")
			return
		}
		if check == 1 {
			break
		}
		time.Sleep(1 * time.Second)
		i++
	}

	_, err = c.Do("SET", key, b)
	_, err = c.Do("SET", key+"-scan", time.Now().Unix())
	return
}
func (j *JobQuene) Remove(key string) (err error) {
	c := j.rpool.Get()
	//redis lock 確保同時間 jobcheck跟 remove job 不會同時進行
	defer func() {
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
		if i >= 25 {
			err = errors.New("remove joblock error")
			return
		}
		if check == 1 || i >= 20 {
			break
		}
		time.Sleep(1 * time.Second)
		i++
	}
	_, err = c.Do("DEL", key)
	_, err = c.Do("DEL", key+"-scan")
	return
}
func (j *JobQuene) List(key string) (jobs []*guaproto.Job, err error) {
	c := j.rpool.Get()
	defer c.Close()
	keys, err := RedisScan(c, key)
	if err != nil {
		return
	}
	jobs = make([]*guaproto.Job, 0)
	for _, v := range keys {
		if jobRe.MatchString(v) {
			b, err := redis.Bytes(c.Do("GET", v))
			if err != nil {
				continue
			}
			jb := &guaproto.Job{}
			err = proto.Unmarshal(b, jb)
			if err != nil {
				continue
			}
			jobs = append(jobs, jb)
		}
	}
	return
}
