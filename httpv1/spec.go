package httpv1

type EditJobPayload struct {
	GroupName  string `json:"group_name"`
	Id         string `json:"id"`
	RequestUrl string `json:"request_url"`
	ExecCmd    string `json:"exec_cmd"`
}
type AddJobPayload struct {
	GroupName       string `json:"group_name"`
	JobId           string `json:"job_id"`
	Name            string `json:"name"`
	Exectime        int64  `json:"exec_time"`
	RequestUrl      string `json:"request_url"`
	IntervalPattern string `json:"interval_pattern"`
	ExecCommand     string `json:"exec_command"`
	Timeout         int64  `json:"timeout"`
	UseGroupOtp     bool   `json:"use_group_otp"`
	Memo            string `json:"memo"`
}
type RegisterGroupPayload struct {
	GroupName string `json:"group_name"`
}

type AddFuncPayload struct {
	GroupName       string `json:"group_name"`
	Name            string `json:"name"`
	UseOtp          bool   `json:"use_otp"`
	DisableGroupOtp bool   `json:"disable_group_otp"`
	LuaBody         string `json:"lua_body"`
	Memo            string `sjon:"memo"`
}

type JobControlPayload struct {
	GroupName string `json:"group_name"`
	JobId     string `json:"job_id"`
}
type ActiveJobPayload struct {
	GroupName string `json:"group_name"`
	JobId     string `json:"job_id"`
	Exectime  int64  `json:"exec_time"`
}

type GetJobsPayload struct {
	GroupName string `json:"group_name"`
}

type ResponseJobList struct {
	Name            string `json:"name"`
	Id              string `json:"id"`
	Exectime        int64  `json:"exec_time"`
	OtpToken        string `json:"otp_token"`
	IntervalPattern string `json:"interval_pattern"`
	RequestUrl      string `json:"request_url"`
	ExecCmd         string `json:"exec_cmd"`
	GroupName       string `json:"group_name"`
	Active          bool   `json:"active"`
	Memo            string `json:"memo"`
}
