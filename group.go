package main

import (
	"fmt"

	"github.com/gogo/protobuf/proto"
	"github.com/gomodule/redigo/redis"
	guaproto "github.com/syhlion/gua/proto"
)

type Group struct {
	apiRedis   *redis.Pool
	groupRedis *redis.Pool
	delayRedis *redis.Pool
}

func (g *Group) GetGroup(groupName string) (otpToken string, err error) {
	c := g.groupRedis.Get()
	defer c.Close()
	groupKey := fmt.Sprintf("USER_%s", groupName)
	return redis.String(c.Do("GET", groupKey))
}
func (g *Group) GetJobList(groupName string) (jobs []*guaproto.Job, err error) {
	c := g.delayRedis.Get()
	defer c.Close()
	jobKey := fmt.Sprintf("JOB-%s-*", groupName)
	keys, err := RedisScan(c, jobKey)
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
func (g *Group) GetFuncList(groupName string) (otpToken string, err error) {
	return
}
func (g *Group) GetNodeList(groupName string) (otpToken string, err error) {
	return
}
