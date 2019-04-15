package loghook

type Payload struct {
	FinishTime         int64  `json:"finish_time"`
	PlanTime           int64  `json:"plan_time"`
	ExecTime           int64  `json:"exec_time"`
	GetJobTime         int64  `json:"get_job_time"`
	JobId              string `json:"job_id"`
	Error              string `json:"error"`
	Type               string `json:"type"`
	Success            string `json:"success"`
	ExecJobMachineHost string `json:"exec_job_machine_host"`
	ExecJobMachineMac  string `json:"exec_job_machine_mac"`
	ExecJobMachineIp   string `json:"exec_job_machine_ip"`
	GetJobMachineHost  string `json:"get_job_machine_host"`
	GetJobMachineMac   string `json:"get_job_machine_mac"`
	GetJobMachineIp    string `json:"get_job_machine_ip"`
	ExecMachineHost    string `json:"exec_machine_host"`
	RemoteNodeMac      string `json:"remote_node_mac"`
	RemoteNodeIp       string `json:"remote_node_ip"`
	RemoteNodeHost     string `json:"remote_node_host"`
	GroupName          string `json:"group_name"`
}
