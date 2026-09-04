package httpv1

// EditJobPayload is the PATCH body for a job; group + job id come from the path.
type EditJobPayload struct {
	RequestUrl string `json:"request_url"`
	Payload    string `json:"payload"`
}

// AddJobPayload is the POST body for a new job; the group comes from the path.
type AddJobPayload struct {
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

// ActiveJobPayload is the activate body; group + job id come from the path.
type ActiveJobPayload struct {
	Exectime int64 `json:"exec_time"`
}

// ResponseJobList is one job as returned by GET /v1/groups/{group}/jobs.
// exec_time is the next scheduled fire (updated after every run of a
// recurring job).
type ResponseJobList struct {
	Name            string `json:"name"`
	Id              string `json:"id"`
	Exectime        int64  `json:"exec_time"`
	IntervalPattern string `json:"interval_pattern"`
	RequestUrl      string `json:"request_url"`
	Payload         string `json:"payload"`
	Timeout         int64  `json:"timeout"`
	GroupName       string `json:"group_name"`
	Active          bool   `json:"active"`
	Memo            string `json:"memo"`
}
