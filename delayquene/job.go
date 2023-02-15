package delayquene

import (
	"errors"
	"time"

	"github.com/gomodule/redigo/redis"
	guaproto "github.com/syhlion/gua/proto"
	"google.golang.org/protobuf/proto"
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
func (j *JobQuene) Exist(key string) (duplicate int, err error) {

	c := j.rpool.Get()
	defer c.Close()

	return redis.Int(c.Do("EXISTS", key))
}
func (j *JobQuene) Update(key string, jb *guaproto.Job) (err error) {
	b, err := proto.Marshal(jb)
	if err != nil {
		return
	}
	c := j.rpool.Get()
	defer c.Close()
	var i = 0
	var check = 0
	t := time.NewTimer(100 * time.Millisecond)
	for {
		//確認是否有鎖
		check, err = redis.Int(c.Do("GET", "JOBCHECKLOCK"))
		if check == 0 || i >= 10 {
			break
		}
		<-t.C
		t.Reset(100 * time.Millisecond)
		i++

	}
	_, err = c.Do("SET", key, b)
	return
}
func (j *JobQuene) Add(key string, jb *guaproto.Job) (err error) {
	duplicate, err := j.Exist(key)
	if err != nil {
		return
	}
	if duplicate == 1 {
		err = errors.New("key duplicate")
		return
	}
	b, err := proto.Marshal(jb)
	if err != nil {
		return
	}
	c := j.rpool.Get()
	defer c.Close()
	var i = 0
	var check = 0
	t := time.NewTimer(100 * time.Millisecond)
	for {
		//確認是否有鎖
		check, err = redis.Int(c.Do("GET", "JOBCHECKLOCK"))
		if check == 0 || i >= 10 {
			break
		}
		<-t.C
		t.Reset(100 * time.Millisecond)
		i++

	}
	c.Send("SET", key, b)
	c.Send("SET", key+"-scan", time.Now().Unix())
	c.Flush()
	_, err = c.Receive()
	return
}
func (j *JobQuene) Remove(key string) (err error) {
	c := j.rpool.Get()
	defer c.Close()
	var i = 0
	var check = 0
	t := time.NewTimer(100 * time.Millisecond)
	for {
		//確認是否有鎖
		check, err = redis.Int(c.Do("GET", "JOBCHECKLOCK"))

		if check == 0 || i >= 10 {
			break
		}
		<-t.C
		t.Reset(100 * time.Millisecond)
		i++

	}
	c.Send("DEL", key)
	c.Send("DEL", key+"-scan")
	c.Flush()
	_, err = c.Receive()
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
