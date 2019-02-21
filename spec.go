package main

type AddJobPayload struct {
	Name            string `json:"name"`
	Exectime        int64  `json:"exec_time"`
	RequestUrl      string `json:"request_url"`
	IntervalPattern string `json:"interval_pattern"`
	ExecCommand     string `json:"exec_command"`
	Timeout         int64  `json:"timeout"`
}
