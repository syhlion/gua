package main

import "time"

type Config struct {
	Hostname    string
	Mac         string
	ExternalIp  string
	MachineCode string
	GuaAddr     string
	WorkerNum   int
	GrpcListen  string
	HttpListen  string
	OtpToken    string
	NodeId      int64
	StartTime   time.Time
}

func (c *Config) GetStartTime() string {
	return c.StartTime.Format("2006/01/02 15:04:05")
}
