// Code generated by protoc-gen-go. DO NOT EDIT.
// source: job.proto

package guaproto

import (
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

type Job struct {
	Name                 string   `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Id                   string   `protobuf:"bytes,2,opt,name=Id,proto3" json:"Id,omitempty"`
	Exectime             int64    `protobuf:"varint,3,opt,name=exectime,proto3" json:"exectime,omitempty"`
	OtpToken             string   `protobuf:"bytes,4,opt,name=otp_token,json=otpToken,proto3" json:"otp_token,omitempty"`
	Timeout              int64    `protobuf:"varint,5,opt,name=timeout,proto3" json:"timeout,omitempty"`
	IntervalPattern      string   `protobuf:"bytes,6,opt,name=interval_pattern,json=intervalPattern,proto3" json:"interval_pattern,omitempty"`
	RequestUrl           string   `protobuf:"bytes,7,opt,name=request_url,json=requestUrl,proto3" json:"request_url,omitempty"`
	ExecCmd              []byte   `protobuf:"bytes,8,opt,name=exec_cmd,json=execCmd,proto3" json:"exec_cmd,omitempty"`
	GroupName            string   `protobuf:"bytes,9,opt,name=group_name,json=groupName,proto3" json:"group_name,omitempty"`
	UseOtp               bool     `protobuf:"varint,10,opt,name=use_otp,json=useOtp,proto3" json:"use_otp,omitempty"`
	DisableGroupOtp      bool     `protobuf:"varint,11,opt,name=disable_group_otp,json=disableGroupOtp,proto3" json:"disable_group_otp,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Job) Reset()         { *m = Job{} }
func (m *Job) String() string { return proto.CompactTextString(m) }
func (*Job) ProtoMessage()    {}
func (*Job) Descriptor() ([]byte, []int) {
	return fileDescriptor_f32c477d91a04ead, []int{0}
}

func (m *Job) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Job.Unmarshal(m, b)
}
func (m *Job) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Job.Marshal(b, m, deterministic)
}
func (m *Job) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Job.Merge(m, src)
}
func (m *Job) XXX_Size() int {
	return xxx_messageInfo_Job.Size(m)
}
func (m *Job) XXX_DiscardUnknown() {
	xxx_messageInfo_Job.DiscardUnknown(m)
}

var xxx_messageInfo_Job proto.InternalMessageInfo

func (m *Job) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func (m *Job) GetId() string {
	if m != nil {
		return m.Id
	}
	return ""
}

func (m *Job) GetExectime() int64 {
	if m != nil {
		return m.Exectime
	}
	return 0
}

func (m *Job) GetOtpToken() string {
	if m != nil {
		return m.OtpToken
	}
	return ""
}

func (m *Job) GetTimeout() int64 {
	if m != nil {
		return m.Timeout
	}
	return 0
}

func (m *Job) GetIntervalPattern() string {
	if m != nil {
		return m.IntervalPattern
	}
	return ""
}

func (m *Job) GetRequestUrl() string {
	if m != nil {
		return m.RequestUrl
	}
	return ""
}

func (m *Job) GetExecCmd() []byte {
	if m != nil {
		return m.ExecCmd
	}
	return nil
}

func (m *Job) GetGroupName() string {
	if m != nil {
		return m.GroupName
	}
	return ""
}

func (m *Job) GetUseOtp() bool {
	if m != nil {
		return m.UseOtp
	}
	return false
}

func (m *Job) GetDisableGroupOtp() bool {
	if m != nil {
		return m.DisableGroupOtp
	}
	return false
}

type ReadyJob struct {
	Name                 string   `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Id                   string   `protobuf:"bytes,2,opt,name=Id,proto3" json:"Id,omitempty"`
	OtpToken             string   `protobuf:"bytes,3,opt,name=otp_token,json=otpToken,proto3" json:"otp_token,omitempty"`
	Timeout              int64    `protobuf:"varint,4,opt,name=timeout,proto3" json:"timeout,omitempty"`
	RequestUrl           string   `protobuf:"bytes,5,opt,name=request_url,json=requestUrl,proto3" json:"request_url,omitempty"`
	ExecCmd              []byte   `protobuf:"bytes,6,opt,name=exec_cmd,json=execCmd,proto3" json:"exec_cmd,omitempty"`
	PlanTime             int64    `protobuf:"varint,7,opt,name=plan_time,json=planTime,proto3" json:"plan_time,omitempty"`
	GetJobTime           int64    `protobuf:"varint,8,opt,name=get_job_time,json=getJobTime,proto3" json:"get_job_time,omitempty"`
	GetJobMachineHost    string   `protobuf:"bytes,9,opt,name=get_job_machine_host,json=getJobMachineHost,proto3" json:"get_job_machine_host,omitempty"`
	GetJobMachineMac     string   `protobuf:"bytes,10,opt,name=get_job_machine_mac,json=getJobMachineMac,proto3" json:"get_job_machine_mac,omitempty"`
	GetJobMachineIp      string   `protobuf:"bytes,11,opt,name=get_job_machine_ip,json=getJobMachineIp,proto3" json:"get_job_machine_ip,omitempty"`
	GroupName            string   `protobuf:"bytes,12,opt,name=group_name,json=groupName,proto3" json:"group_name,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ReadyJob) Reset()         { *m = ReadyJob{} }
func (m *ReadyJob) String() string { return proto.CompactTextString(m) }
func (*ReadyJob) ProtoMessage()    {}
func (*ReadyJob) Descriptor() ([]byte, []int) {
	return fileDescriptor_f32c477d91a04ead, []int{1}
}

func (m *ReadyJob) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ReadyJob.Unmarshal(m, b)
}
func (m *ReadyJob) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ReadyJob.Marshal(b, m, deterministic)
}
func (m *ReadyJob) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ReadyJob.Merge(m, src)
}
func (m *ReadyJob) XXX_Size() int {
	return xxx_messageInfo_ReadyJob.Size(m)
}
func (m *ReadyJob) XXX_DiscardUnknown() {
	xxx_messageInfo_ReadyJob.DiscardUnknown(m)
}

var xxx_messageInfo_ReadyJob proto.InternalMessageInfo

func (m *ReadyJob) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func (m *ReadyJob) GetId() string {
	if m != nil {
		return m.Id
	}
	return ""
}

func (m *ReadyJob) GetOtpToken() string {
	if m != nil {
		return m.OtpToken
	}
	return ""
}

func (m *ReadyJob) GetTimeout() int64 {
	if m != nil {
		return m.Timeout
	}
	return 0
}

func (m *ReadyJob) GetRequestUrl() string {
	if m != nil {
		return m.RequestUrl
	}
	return ""
}

func (m *ReadyJob) GetExecCmd() []byte {
	if m != nil {
		return m.ExecCmd
	}
	return nil
}

func (m *ReadyJob) GetPlanTime() int64 {
	if m != nil {
		return m.PlanTime
	}
	return 0
}

func (m *ReadyJob) GetGetJobTime() int64 {
	if m != nil {
		return m.GetJobTime
	}
	return 0
}

func (m *ReadyJob) GetGetJobMachineHost() string {
	if m != nil {
		return m.GetJobMachineHost
	}
	return ""
}

func (m *ReadyJob) GetGetJobMachineMac() string {
	if m != nil {
		return m.GetJobMachineMac
	}
	return ""
}

func (m *ReadyJob) GetGetJobMachineIp() string {
	if m != nil {
		return m.GetJobMachineIp
	}
	return ""
}

func (m *ReadyJob) GetGroupName() string {
	if m != nil {
		return m.GroupName
	}
	return ""
}

func init() {
	proto.RegisterType((*Job)(nil), "guaproto.Job")
	proto.RegisterType((*ReadyJob)(nil), "guaproto.ReadyJob")
}

func init() { proto.RegisterFile("job.proto", fileDescriptor_f32c477d91a04ead) }

var fileDescriptor_f32c477d91a04ead = []byte{
	// 401 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x8c, 0x92, 0xd1, 0x6a, 0xd4, 0x40,
	0x14, 0x86, 0xc9, 0x66, 0xbb, 0x49, 0x4e, 0x17, 0xb7, 0x1d, 0x05, 0x47, 0x8b, 0x18, 0x7a, 0xb5,
	0x2a, 0xea, 0x85, 0x8f, 0xe0, 0x85, 0x6e, 0xa1, 0x56, 0x42, 0xbd, 0x1e, 0x26, 0xc9, 0x21, 0xcd,
	0x9a, 0x64, 0xc6, 0xe4, 0x44, 0xf4, 0x1d, 0x7c, 0x2f, 0x5f, 0x4b, 0xe6, 0xcc, 0xae, 0x90, 0x80,
	0xd2, 0xbb, 0x39, 0xff, 0xf9, 0x7f, 0xc8, 0xf9, 0xfe, 0x40, 0xb2, 0x37, 0xf9, 0x1b, 0xdb, 0x1b,
	0x32, 0x22, 0xae, 0x46, 0xcd, 0xaf, 0xcb, 0xdf, 0x0b, 0x08, 0xaf, 0x4c, 0x2e, 0x04, 0x2c, 0x3b,
	0xdd, 0xa2, 0x0c, 0xd2, 0x60, 0x9b, 0x64, 0xfc, 0x16, 0x0f, 0x60, 0xb1, 0x2b, 0xe5, 0x82, 0x95,
	0xc5, 0xae, 0x14, 0x4f, 0x21, 0xc6, 0x1f, 0x58, 0x50, 0xdd, 0xa2, 0x0c, 0xd3, 0x60, 0x1b, 0x66,
	0x7f, 0x67, 0x71, 0x01, 0x89, 0x21, 0xab, 0xc8, 0x7c, 0xc5, 0x4e, 0x2e, 0x39, 0x12, 0x1b, 0xb2,
	0xb7, 0x6e, 0x16, 0x12, 0x22, 0x67, 0x32, 0x23, 0xc9, 0x13, 0xce, 0x1d, 0x47, 0xf1, 0x02, 0xce,
	0xea, 0x8e, 0xb0, 0xff, 0xae, 0x1b, 0x65, 0x35, 0x11, 0xf6, 0x9d, 0x5c, 0x71, 0x7a, 0x73, 0xd4,
	0x3f, 0x7b, 0x59, 0x3c, 0x87, 0xd3, 0x1e, 0xbf, 0x8d, 0x38, 0x90, 0x1a, 0xfb, 0x46, 0x46, 0xec,
	0x82, 0x83, 0xf4, 0xa5, 0x6f, 0xc4, 0x13, 0xff, 0x79, 0xaa, 0x68, 0x4b, 0x19, 0xa7, 0xc1, 0x76,
	0x9d, 0x45, 0x6e, 0x7e, 0xdf, 0x96, 0xe2, 0x19, 0x40, 0xd5, 0x9b, 0xd1, 0x2a, 0xbe, 0x31, 0xe1,
	0x68, 0xc2, 0xca, 0x27, 0x77, 0xe8, 0x63, 0x88, 0xc6, 0x01, 0x95, 0x21, 0x2b, 0x21, 0x0d, 0xb6,
	0x71, 0xb6, 0x1a, 0x07, 0xbc, 0x21, 0x2b, 0x5e, 0xc2, 0x79, 0x59, 0x0f, 0x3a, 0x6f, 0x50, 0xf9,
	0xbc, 0xb3, 0x9c, 0xb2, 0x65, 0x73, 0x58, 0x7c, 0x70, 0xfa, 0x0d, 0xd9, 0xcb, 0x5f, 0x21, 0xc4,
	0x19, 0xea, 0xf2, 0xe7, 0x7d, 0x71, 0x4e, 0x90, 0x85, 0xff, 0x46, 0xb6, 0x9c, 0x22, 0x9b, 0x71,
	0x38, 0xf9, 0x2f, 0x87, 0xd5, 0x94, 0xc3, 0x05, 0x24, 0xb6, 0xd1, 0x9d, 0xe2, 0x0a, 0x23, 0x5f,
	0xa1, 0x13, 0x6e, 0x5d, 0x85, 0x29, 0xac, 0x2b, 0x24, 0xb5, 0x37, 0xb9, 0xdf, 0xc7, 0xbc, 0x87,
	0x0a, 0xe9, 0xca, 0xe4, 0xec, 0x78, 0x0b, 0x8f, 0x8e, 0x8e, 0x56, 0x17, 0x77, 0x75, 0x87, 0xea,
	0xce, 0x0c, 0x74, 0x00, 0x7a, 0xee, 0x9d, 0xd7, 0x7e, 0xf3, 0xd1, 0x0c, 0x24, 0x5e, 0xc3, 0xc3,
	0x79, 0xa0, 0xd5, 0x05, 0x43, 0x4e, 0xb2, 0xb3, 0x89, 0xff, 0x5a, 0x17, 0xe2, 0x15, 0x88, 0xb9,
	0xbd, 0xf6, 0xbc, 0x93, 0x6c, 0x33, 0x71, 0xef, 0xec, 0xac, 0xd3, 0xf5, 0xac, 0xd3, 0x7c, 0xc5,
	0xff, 0xf7, 0xbb, 0x3f, 0x01, 0x00, 0x00, 0xff, 0xff, 0x76, 0x11, 0xe0, 0x4b, 0xf6, 0x02, 0x00,
	0x00,
}
