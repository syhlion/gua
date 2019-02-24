package main

import "time"

type Config struct {
	CompileDate   string
	Version       string
	Hostname      string
	Mac           string
	ExternalIp    string
	MachineCode   string
	GuaAddr       string
	WorkerNum     int
	GrpcListen    string
	HttpListen    string
	OtpToken      string
	NodeId        string
	BoradcastAddr string
	StartTime     time.Time
}

func (c *Config) GetStartTime() string {
	return c.StartTime.Format("2006/01/02 15:04:05")
}
