package delayquene

import (
	fmt "fmt"

	"github.com/gogo/protobuf/proto"
	"github.com/gomodule/redigo/redis"
)

type JobQuene struct {
	rpool *redis.Pool
}

func (j *JobQuene) Get(key string) (jb *Job, err error) {
	c := j.rpool.Get()
	defer c.Close()
	reply, err := redis.Bytes(c.Do("GET", fmt.Sprintf(jobNamePrefix, key)))
	if err != nil {
		return
	}
	jb = &Job{}
	err = proto.Unmarshal(reply, jb)
	return
}
func (j *JobQuene) Add(key string, jb *Job) (err error) {
	b, err := proto.Marshal(jb)
	if err != nil {
		return
	}
	c := j.rpool.Get()
	defer c.Close()
	_, err = c.Do("SET", fmt.Sprintf(jobNamePrefix, key), b)
	return
}
func (j *JobQuene) Remove(key string) (err error) {
	c := j.rpool.Get()
	defer c.Close()
	_, err = c.Do("DEL", fmt.Sprintf(jobNamePrefix, key))
	return
}
