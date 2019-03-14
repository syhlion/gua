package main

import "time"

type Config struct {
	CompileDate               string
	Version                   string
	Hostname                  string
	Mac                       string
	ExternalIp                string
	WorkerNum                 int
	GrpcListen                string
	HttpListen                string
	RedisForApiAddr           string
	RedisForApiDBNo           int
	RedisForApiMaxIdle        int
	RedisForApiMaxConn        int
	RedisForReadyAddr         string
	RedisForReadyDBNo         int
	RedisForReadyMaxIdle      int
	RedisForReadyMaxConn      int
	RedisForDelayQueneAddr    string
	RedisForDelayQueneDBNo    int
	RedisForDelayQueneMaxIdle int
	RedisForDelayQueneMaxConn int
	RedisForGroupAddr         string
	RedisForGroupDBNo         int
	RedisForGroupMaxIdle      int
	RedisForGroupMaxConn      int
	JobReplyHook              string
	MachineCode               string
	StartTime                 time.Time
}

func (c *Config) GetStartTime() string {
	return c.StartTime.Format("2006/01/02 15:04:05")
}
