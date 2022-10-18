package delayquene

import (
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
func (j *JobQuene) ScanTime(key string, t time.Time) (err error) {
	c := j.rpool.Get()
	defer c.Close()
	_, err = c.Do("SET", key+"-scan", t.Unix())
	return
}

func (j *JobQuene) Add(key string, jb *guaproto.Job) (err error) {
	b, err := proto.Marshal(jb)
	if err != nil {
		return
	}
	c := j.rpool.Get()
	defer c.Close()
	_, err = c.Do("SET", key, b)
	return
}
func (j *JobQuene) Remove(key string) (err error) {
	c := j.rpool.Get()
	defer c.Close()
	_, err = c.Do("DEL", key)
	return
}
func (j *JobQuene) List(key string) (jobs []*guaproto.Job, err error) {
	c := j.rpool.Get()
	defer c.Close()
	keys, err := redis.Strings(c.Do("KEYS", key))
	if err != nil {
		return
	}
	jobs = make([]*guaproto.Job, 0)
	for _, v := range keys {
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
	return
}
