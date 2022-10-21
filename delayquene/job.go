package delayquene

import (
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
	defer c.Close()
	_, err = c.Do("SET", key, b)
	return
}
func (j *JobQuene) Remove(key string) (err error) {
	c := j.rpool.Get()
	defer c.Close()
	_, err = c.Do("DEL", key)
	_, err = c.Do("DEL", key+"scan")
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
