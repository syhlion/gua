package httpv1

type EditJobPayload struct {
	GroupName  string `json:"group_name"`
	Id         string `json:"id"`
	RequestUrl string `json:"request_url"`
	Payload    string `json:"payload"`
}
type AddJobPayload struct {
	GroupName       string `json:"group_name"`
	JobId           string `json:"job_id"`
	Name            string `json:"name"`
	Exectime        int64  `json:"exec_time"`
	RequestUrl      string `json:"request_url"`
	IntervalPattern string `json:"interval_pattern"`
	Payload         string `json:"payload"`
	Timeout         int64  `json:"timeout"`
	Memo            string `json:"memo"`
}
type RegisterGroupPayload struct {
	GroupName string `json:"group_name"`
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
	IntervalPattern string `json:"interval_pattern"`
	RequestUrl      string `json:"request_url"`
	Payload         string `json:"payload"`
	GroupName       string `json:"group_name"`
	Active          bool   `json:"active"`
	Memo            string `json:"memo"`
}
