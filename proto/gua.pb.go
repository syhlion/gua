// Code generated by protoc-gen-go. DO NOT EDIT.
// source: gua.proto

package guaproto

import (
	context "context"
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	grpc "google.golang.org/grpc"
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

type JobReplyRequest struct {
	JobId                string   `protobuf:"bytes,1,opt,name=job_id,json=jobId,proto3" json:"job_id,omitempty"`
	Success              string   `protobuf:"bytes,2,opt,name=success,proto3" json:"success,omitempty"`
	Error                string   `protobuf:"bytes,3,opt,name=error,proto3" json:"error,omitempty"`
	OtpCode              string   `protobuf:"bytes,4,opt,name=otp_code,json=otpCode,proto3" json:"otp_code,omitempty"`
	PlanTime             int64    `protobuf:"varint,5,opt,name=plan_time,json=planTime,proto3" json:"plan_time,omitempty"`
	ExecTime             int64    `protobuf:"varint,6,opt,name=exec_time,json=execTime,proto3" json:"exec_time,omitempty"`
	FinishTime           int64    `protobuf:"varint,7,opt,name=finish_time,json=finishTime,proto3" json:"finish_time,omitempty"`
	GetJobTime           int64    `protobuf:"varint,8,opt,name=get_job_time,json=getJobTime,proto3" json:"get_job_time,omitempty"`
	ExecJobMachineHost   string   `protobuf:"bytes,9,opt,name=exec_job_machine_host,json=execJobMachineHost,proto3" json:"exec_job_machine_host,omitempty"`
	ExecJobMachineMac    string   `protobuf:"bytes,10,opt,name=exec_job_machine_mac,json=execJobMachineMac,proto3" json:"exec_job_machine_mac,omitempty"`
	ExecJobMachineIp     string   `protobuf:"bytes,11,opt,name=exec_job_machine_ip,json=execJobMachineIp,proto3" json:"exec_job_machine_ip,omitempty"`
	GetJobMachineHost    string   `protobuf:"bytes,12,opt,name=get_job_machine_host,json=getJobMachineHost,proto3" json:"get_job_machine_host,omitempty"`
	GetJobMachineMac     string   `protobuf:"bytes,13,opt,name=get_job_machine_mac,json=getJobMachineMac,proto3" json:"get_job_machine_mac,omitempty"`
	GetJobMachineIp      string   `protobuf:"bytes,14,opt,name=get_job_machine_ip,json=getJobMachineIp,proto3" json:"get_job_machine_ip,omitempty"`
	NodeId               string   `protobuf:"bytes,15,opt,name=node_id,json=nodeId,proto3" json:"node_id,omitempty"`
	GroupName            string   `protobuf:"bytes,16,opt,name=group_name,json=groupName,proto3" json:"group_name,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *JobReplyRequest) Reset()         { *m = JobReplyRequest{} }
func (m *JobReplyRequest) String() string { return proto.CompactTextString(m) }
func (*JobReplyRequest) ProtoMessage()    {}
func (*JobReplyRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_55c0f4d918b61ad5, []int{0}
}

func (m *JobReplyRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_JobReplyRequest.Unmarshal(m, b)
}
func (m *JobReplyRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_JobReplyRequest.Marshal(b, m, deterministic)
}
func (m *JobReplyRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_JobReplyRequest.Merge(m, src)
}
func (m *JobReplyRequest) XXX_Size() int {
	return xxx_messageInfo_JobReplyRequest.Size(m)
}
func (m *JobReplyRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_JobReplyRequest.DiscardUnknown(m)
}

var xxx_messageInfo_JobReplyRequest proto.InternalMessageInfo

func (m *JobReplyRequest) GetJobId() string {
	if m != nil {
		return m.JobId
	}
	return ""
}

func (m *JobReplyRequest) GetSuccess() string {
	if m != nil {
		return m.Success
	}
	return ""
}

func (m *JobReplyRequest) GetError() string {
	if m != nil {
		return m.Error
	}
	return ""
}

func (m *JobReplyRequest) GetOtpCode() string {
	if m != nil {
		return m.OtpCode
	}
	return ""
}

func (m *JobReplyRequest) GetPlanTime() int64 {
	if m != nil {
		return m.PlanTime
	}
	return 0
}

func (m *JobReplyRequest) GetExecTime() int64 {
	if m != nil {
		return m.ExecTime
	}
	return 0
}

func (m *JobReplyRequest) GetFinishTime() int64 {
	if m != nil {
		return m.FinishTime
	}
	return 0
}

func (m *JobReplyRequest) GetGetJobTime() int64 {
	if m != nil {
		return m.GetJobTime
	}
	return 0
}

func (m *JobReplyRequest) GetExecJobMachineHost() string {
	if m != nil {
		return m.ExecJobMachineHost
	}
	return ""
}

func (m *JobReplyRequest) GetExecJobMachineMac() string {
	if m != nil {
		return m.ExecJobMachineMac
	}
	return ""
}

func (m *JobReplyRequest) GetExecJobMachineIp() string {
	if m != nil {
		return m.ExecJobMachineIp
	}
	return ""
}

func (m *JobReplyRequest) GetGetJobMachineHost() string {
	if m != nil {
		return m.GetJobMachineHost
	}
	return ""
}

func (m *JobReplyRequest) GetGetJobMachineMac() string {
	if m != nil {
		return m.GetJobMachineMac
	}
	return ""
}

func (m *JobReplyRequest) GetGetJobMachineIp() string {
	if m != nil {
		return m.GetJobMachineIp
	}
	return ""
}

func (m *JobReplyRequest) GetNodeId() string {
	if m != nil {
		return m.NodeId
	}
	return ""
}

func (m *JobReplyRequest) GetGroupName() string {
	if m != nil {
		return m.GroupName
	}
	return ""
}

type JobReplyResponse struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *JobReplyResponse) Reset()         { *m = JobReplyResponse{} }
func (m *JobReplyResponse) String() string { return proto.CompactTextString(m) }
func (*JobReplyResponse) ProtoMessage()    {}
func (*JobReplyResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_55c0f4d918b61ad5, []int{1}
}

func (m *JobReplyResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_JobReplyResponse.Unmarshal(m, b)
}
func (m *JobReplyResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_JobReplyResponse.Marshal(b, m, deterministic)
}
func (m *JobReplyResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_JobReplyResponse.Merge(m, src)
}
func (m *JobReplyResponse) XXX_Size() int {
	return xxx_messageInfo_JobReplyResponse.Size(m)
}
func (m *JobReplyResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_JobReplyResponse.DiscardUnknown(m)
}

var xxx_messageInfo_JobReplyResponse proto.InternalMessageInfo

type NodeRegisterRequest struct {
	Hostname             string   `protobuf:"bytes,1,opt,name=hostname,proto3" json:"hostname,omitempty"`
	Ip                   string   `protobuf:"bytes,2,opt,name=ip,proto3" json:"ip,omitempty"`
	Mac                  string   `protobuf:"bytes,3,opt,name=mac,proto3" json:"mac,omitempty"`
	OtpToken             string   `protobuf:"bytes,4,opt,name=otp_token,json=otpToken,proto3" json:"otp_token,omitempty"`
	BoradcastAddr        string   `protobuf:"bytes,5,opt,name=boradcast_addr,json=boradcastAddr,proto3" json:"boradcast_addr,omitempty"`
	Grpclisten           string   `protobuf:"bytes,6,opt,name=grpclisten,proto3" json:"grpclisten,omitempty"`
	MachineCode          string   `protobuf:"bytes,7,opt,name=machine_code,json=machineCode,proto3" json:"machine_code,omitempty"`
	GroupName            string   `protobuf:"bytes,8,opt,name=group_name,json=groupName,proto3" json:"group_name,omitempty"`
	OtpCode              string   `protobuf:"bytes,9,opt,name=otp_code,json=otpCode,proto3" json:"otp_code,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *NodeRegisterRequest) Reset()         { *m = NodeRegisterRequest{} }
func (m *NodeRegisterRequest) String() string { return proto.CompactTextString(m) }
func (*NodeRegisterRequest) ProtoMessage()    {}
func (*NodeRegisterRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_55c0f4d918b61ad5, []int{2}
}

func (m *NodeRegisterRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_NodeRegisterRequest.Unmarshal(m, b)
}
func (m *NodeRegisterRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_NodeRegisterRequest.Marshal(b, m, deterministic)
}
func (m *NodeRegisterRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_NodeRegisterRequest.Merge(m, src)
}
func (m *NodeRegisterRequest) XXX_Size() int {
	return xxx_messageInfo_NodeRegisterRequest.Size(m)
}
func (m *NodeRegisterRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_NodeRegisterRequest.DiscardUnknown(m)
}

var xxx_messageInfo_NodeRegisterRequest proto.InternalMessageInfo

func (m *NodeRegisterRequest) GetHostname() string {
	if m != nil {
		return m.Hostname
	}
	return ""
}

func (m *NodeRegisterRequest) GetIp() string {
	if m != nil {
		return m.Ip
	}
	return ""
}

func (m *NodeRegisterRequest) GetMac() string {
	if m != nil {
		return m.Mac
	}
	return ""
}

func (m *NodeRegisterRequest) GetOtpToken() string {
	if m != nil {
		return m.OtpToken
	}
	return ""
}

func (m *NodeRegisterRequest) GetBoradcastAddr() string {
	if m != nil {
		return m.BoradcastAddr
	}
	return ""
}

func (m *NodeRegisterRequest) GetGrpclisten() string {
	if m != nil {
		return m.Grpclisten
	}
	return ""
}

func (m *NodeRegisterRequest) GetMachineCode() string {
	if m != nil {
		return m.MachineCode
	}
	return ""
}

func (m *NodeRegisterRequest) GetGroupName() string {
	if m != nil {
		return m.GroupName
	}
	return ""
}

func (m *NodeRegisterRequest) GetOtpCode() string {
	if m != nil {
		return m.OtpCode
	}
	return ""
}

type NodeRegisterResponse struct {
	NodeId               string   `protobuf:"bytes,1,opt,name=node_id,json=nodeId,proto3" json:"node_id,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *NodeRegisterResponse) Reset()         { *m = NodeRegisterResponse{} }
func (m *NodeRegisterResponse) String() string { return proto.CompactTextString(m) }
func (*NodeRegisterResponse) ProtoMessage()    {}
func (*NodeRegisterResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_55c0f4d918b61ad5, []int{3}
}

func (m *NodeRegisterResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_NodeRegisterResponse.Unmarshal(m, b)
}
func (m *NodeRegisterResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_NodeRegisterResponse.Marshal(b, m, deterministic)
}
func (m *NodeRegisterResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_NodeRegisterResponse.Merge(m, src)
}
func (m *NodeRegisterResponse) XXX_Size() int {
	return xxx_messageInfo_NodeRegisterResponse.Size(m)
}
func (m *NodeRegisterResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_NodeRegisterResponse.DiscardUnknown(m)
}

var xxx_messageInfo_NodeRegisterResponse proto.InternalMessageInfo

func (m *NodeRegisterResponse) GetNodeId() string {
	if m != nil {
		return m.NodeId
	}
	return ""
}

type Ping struct {
	NodeId               string   `protobuf:"bytes,1,opt,name=node_id,json=nodeId,proto3" json:"node_id,omitempty"`
	GroupName            string   `protobuf:"bytes,2,opt,name=group_name,json=groupName,proto3" json:"group_name,omitempty"`
	OtpCode              string   `protobuf:"bytes,3,opt,name=otp_code,json=otpCode,proto3" json:"otp_code,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Ping) Reset()         { *m = Ping{} }
func (m *Ping) String() string { return proto.CompactTextString(m) }
func (*Ping) ProtoMessage()    {}
func (*Ping) Descriptor() ([]byte, []int) {
	return fileDescriptor_55c0f4d918b61ad5, []int{4}
}

func (m *Ping) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Ping.Unmarshal(m, b)
}
func (m *Ping) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Ping.Marshal(b, m, deterministic)
}
func (m *Ping) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Ping.Merge(m, src)
}
func (m *Ping) XXX_Size() int {
	return xxx_messageInfo_Ping.Size(m)
}
func (m *Ping) XXX_DiscardUnknown() {
	xxx_messageInfo_Ping.DiscardUnknown(m)
}

var xxx_messageInfo_Ping proto.InternalMessageInfo

func (m *Ping) GetNodeId() string {
	if m != nil {
		return m.NodeId
	}
	return ""
}

func (m *Ping) GetGroupName() string {
	if m != nil {
		return m.GroupName
	}
	return ""
}

func (m *Ping) GetOtpCode() string {
	if m != nil {
		return m.OtpCode
	}
	return ""
}

type Pong struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Pong) Reset()         { *m = Pong{} }
func (m *Pong) String() string { return proto.CompactTextString(m) }
func (*Pong) ProtoMessage()    {}
func (*Pong) Descriptor() ([]byte, []int) {
	return fileDescriptor_55c0f4d918b61ad5, []int{5}
}

func (m *Pong) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Pong.Unmarshal(m, b)
}
func (m *Pong) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Pong.Marshal(b, m, deterministic)
}
func (m *Pong) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Pong.Merge(m, src)
}
func (m *Pong) XXX_Size() int {
	return xxx_messageInfo_Pong.Size(m)
}
func (m *Pong) XXX_DiscardUnknown() {
	xxx_messageInfo_Pong.DiscardUnknown(m)
}

var xxx_messageInfo_Pong proto.InternalMessageInfo

func init() {
	proto.RegisterType((*JobReplyRequest)(nil), "guaproto.JobReplyRequest")
	proto.RegisterType((*JobReplyResponse)(nil), "guaproto.JobReplyResponse")
	proto.RegisterType((*NodeRegisterRequest)(nil), "guaproto.NodeRegisterRequest")
	proto.RegisterType((*NodeRegisterResponse)(nil), "guaproto.NodeRegisterResponse")
	proto.RegisterType((*Ping)(nil), "guaproto.Ping")
	proto.RegisterType((*Pong)(nil), "guaproto.Pong")
}

func init() { proto.RegisterFile("gua.proto", fileDescriptor_55c0f4d918b61ad5) }

var fileDescriptor_55c0f4d918b61ad5 = []byte{
	// 591 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x7c, 0x54, 0xed, 0x6e, 0xd3, 0x40,
	0x10, 0x6c, 0x3e, 0x9a, 0xd8, 0xdb, 0x34, 0x0d, 0xd7, 0x56, 0xb8, 0x41, 0x2d, 0xc5, 0x12, 0x52,
	0x25, 0x94, 0x54, 0xc0, 0x13, 0xa0, 0xfe, 0xa0, 0x89, 0xd4, 0x52, 0x59, 0xfd, 0xc3, 0x2f, 0xeb,
	0xec, 0x3b, 0x9c, 0x2b, 0xb1, 0xef, 0xb0, 0xcf, 0x12, 0xbc, 0x07, 0x0f, 0x85, 0x78, 0x2a, 0x74,
	0x7b, 0x71, 0x62, 0xa7, 0x2d, 0xff, 0xbc, 0x3b, 0xb3, 0xab, 0xd9, 0xf1, 0xe8, 0xc0, 0x4d, 0x4a,
	0x3a, 0x55, 0xb9, 0xd4, 0x92, 0x38, 0x49, 0x49, 0xf1, 0xcb, 0xff, 0xd3, 0x85, 0x83, 0xb9, 0x8c,
	0x02, 0xae, 0x96, 0xbf, 0x02, 0xfe, 0xa3, 0xe4, 0x85, 0x26, 0xc7, 0xd0, 0x7b, 0x90, 0x51, 0x28,
	0x98, 0xd7, 0x3a, 0x6f, 0x5d, 0xb8, 0xc1, 0xee, 0x83, 0x8c, 0x66, 0x8c, 0x78, 0xd0, 0x2f, 0xca,
	0x38, 0xe6, 0x45, 0xe1, 0xb5, 0xb1, 0x5f, 0x95, 0xe4, 0x08, 0x76, 0x79, 0x9e, 0xcb, 0xdc, 0xeb,
	0x58, 0x3e, 0x16, 0xe4, 0x04, 0x1c, 0xa9, 0x55, 0x18, 0x4b, 0xc6, 0xbd, 0xae, 0x1d, 0x90, 0x5a,
	0x5d, 0x49, 0xc6, 0xc9, 0x2b, 0x70, 0xd5, 0x92, 0x66, 0xa1, 0x16, 0x29, 0xf7, 0x76, 0xcf, 0x5b,
	0x17, 0x9d, 0xc0, 0x31, 0x8d, 0x7b, 0x91, 0x22, 0xc8, 0x7f, 0xf2, 0xd8, 0x82, 0x3d, 0x0b, 0x9a,
	0x06, 0x82, 0xaf, 0x61, 0xef, 0x9b, 0xc8, 0x44, 0xb1, 0xb0, 0x70, 0x1f, 0x61, 0xb0, 0x2d, 0x24,
	0x9c, 0xc3, 0x20, 0xe1, 0x3a, 0x34, 0x07, 0x20, 0xc3, 0xb1, 0x8c, 0x84, 0xeb, 0xb9, 0x8c, 0x90,
	0xf1, 0x1e, 0x8e, 0x71, 0xbf, 0xa1, 0xa4, 0x34, 0x5e, 0x88, 0x8c, 0x87, 0x0b, 0x59, 0x68, 0xcf,
	0x45, 0x91, 0xc4, 0x80, 0x73, 0x19, 0xdd, 0x58, 0xe8, 0x5a, 0x16, 0x9a, 0x5c, 0xc2, 0xd1, 0xa3,
	0x91, 0x94, 0xc6, 0x1e, 0xe0, 0xc4, 0x8b, 0xe6, 0xc4, 0x0d, 0x8d, 0xc9, 0x04, 0x0e, 0x1f, 0x0d,
	0x08, 0xe5, 0xed, 0x21, 0x7f, 0xd4, 0xe4, 0xcf, 0x94, 0xd9, 0x5f, 0x89, 0x6e, 0x28, 0x1a, 0xd8,
	0xfd, 0x56, 0x7c, 0x5d, 0xd0, 0x04, 0x0e, 0xb7, 0x07, 0x8c, 0x9e, 0x7d, 0xbb, 0xbf, 0xc1, 0x37,
	0x72, 0xde, 0x01, 0xd9, 0xa6, 0x0b, 0xe5, 0x0d, 0x91, 0x7d, 0xd0, 0x60, 0xcf, 0x14, 0x79, 0x09,
	0xfd, 0x4c, 0x32, 0x6e, 0xfe, 0xff, 0x01, 0x32, 0x7a, 0xa6, 0x9c, 0x31, 0x72, 0x0a, 0x90, 0xe4,
	0xb2, 0x54, 0x61, 0x46, 0x53, 0xee, 0x8d, 0x10, 0x73, 0xb1, 0x73, 0x4b, 0x53, 0xee, 0x13, 0x18,
	0x6d, 0x92, 0x54, 0x28, 0x99, 0x15, 0xdc, 0xff, 0xdd, 0x86, 0xc3, 0x5b, 0xc9, 0x78, 0xc0, 0x13,
	0x51, 0x68, 0x9e, 0x57, 0x11, 0x1b, 0x83, 0x63, 0x0e, 0xc4, 0x45, 0x36, 0x64, 0xeb, 0x9a, 0x0c,
	0xa1, 0x2d, 0xd4, 0x2a, 0x62, 0x6d, 0xa1, 0xc8, 0x08, 0x3a, 0xe6, 0x36, 0x9b, 0x2d, 0xf3, 0x69,
	0x12, 0x62, 0x92, 0xa5, 0xe5, 0x77, 0x9e, 0xad, 0xa2, 0x65, 0xa2, 0x76, 0x6f, 0x6a, 0xf2, 0x16,
	0x86, 0x91, 0xcc, 0x29, 0x8b, 0x69, 0xa1, 0x43, 0xca, 0x58, 0x8e, 0x01, 0x73, 0x83, 0xfd, 0x75,
	0xf7, 0x13, 0x63, 0x39, 0x39, 0x33, 0xc7, 0xa8, 0x78, 0x69, 0x64, 0x65, 0x18, 0x33, 0x37, 0xa8,
	0x75, 0xc8, 0x1b, 0x18, 0x54, 0x56, 0x61, 0x82, 0xfb, 0xc8, 0xd8, 0x5b, 0xf5, 0x30, 0xc5, 0x4d,
	0x3f, 0x9c, 0x2d, 0x3f, 0x1a, 0xf9, 0x77, 0x1b, 0xf9, 0xf7, 0x2f, 0xe1, 0xa8, 0xe9, 0x8a, 0xb5,
	0xab, 0x6e, 0x7d, 0xab, 0x6e, 0xbd, 0xff, 0x15, 0xba, 0x77, 0x22, 0x4b, 0x9e, 0x25, 0x6c, 0x69,
	0x69, 0xff, 0x4f, 0x4b, 0xa7, 0xa9, 0xa5, 0x07, 0xdd, 0x3b, 0x99, 0x25, 0x1f, 0xfe, 0xb6, 0xa0,
	0xf3, 0xb9, 0xa4, 0xe4, 0x0b, 0x0c, 0xea, 0xda, 0xc8, 0xe9, 0xb4, 0x7a, 0x2c, 0xa6, 0x4f, 0xfc,
	0xc9, 0xf1, 0xd9, 0x73, 0xf0, 0x2a, 0x01, 0x3b, 0xe4, 0x0a, 0x9c, 0x2a, 0x17, 0xe4, 0x64, 0xc3,
	0xde, 0x7a, 0x75, 0xc6, 0xe3, 0xa7, 0xa0, 0xf5, 0x92, 0x09, 0xb8, 0xd7, 0x9c, 0xe6, 0x3a, 0xe2,
	0x54, 0x93, 0xe1, 0x86, 0x6a, 0x5c, 0x19, 0xd7, 0x6b, 0x99, 0x25, 0xfe, 0x4e, 0xd4, 0xc3, 0xea,
	0xe3, 0xbf, 0x00, 0x00, 0x00, 0xff, 0xff, 0x05, 0x40, 0x18, 0xf2, 0xf4, 0x04, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// GuaClient is the client API for Gua service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type GuaClient interface {
	NodeRegister(ctx context.Context, in *NodeRegisterRequest, opts ...grpc.CallOption) (*NodeRegisterResponse, error)
	JobReply(ctx context.Context, in *JobReplyRequest, opts ...grpc.CallOption) (*JobReplyResponse, error)
	Heartbeat(ctx context.Context, in *Ping, opts ...grpc.CallOption) (*Pong, error)
}

type guaClient struct {
	cc *grpc.ClientConn
}

func NewGuaClient(cc *grpc.ClientConn) GuaClient {
	return &guaClient{cc}
}

func (c *guaClient) NodeRegister(ctx context.Context, in *NodeRegisterRequest, opts ...grpc.CallOption) (*NodeRegisterResponse, error) {
	out := new(NodeRegisterResponse)
	err := c.cc.Invoke(ctx, "/guaproto.Gua/NodeRegister", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *guaClient) JobReply(ctx context.Context, in *JobReplyRequest, opts ...grpc.CallOption) (*JobReplyResponse, error) {
	out := new(JobReplyResponse)
	err := c.cc.Invoke(ctx, "/guaproto.Gua/JobReply", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *guaClient) Heartbeat(ctx context.Context, in *Ping, opts ...grpc.CallOption) (*Pong, error) {
	out := new(Pong)
	err := c.cc.Invoke(ctx, "/guaproto.Gua/Heartbeat", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// GuaServer is the server API for Gua service.
type GuaServer interface {
	NodeRegister(context.Context, *NodeRegisterRequest) (*NodeRegisterResponse, error)
	JobReply(context.Context, *JobReplyRequest) (*JobReplyResponse, error)
	Heartbeat(context.Context, *Ping) (*Pong, error)
}

func RegisterGuaServer(s *grpc.Server, srv GuaServer) {
	s.RegisterService(&_Gua_serviceDesc, srv)
}

func _Gua_NodeRegister_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(NodeRegisterRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GuaServer).NodeRegister(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/guaproto.Gua/NodeRegister",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GuaServer).NodeRegister(ctx, req.(*NodeRegisterRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Gua_JobReply_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(JobReplyRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GuaServer).JobReply(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/guaproto.Gua/JobReply",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GuaServer).JobReply(ctx, req.(*JobReplyRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Gua_Heartbeat_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Ping)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GuaServer).Heartbeat(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/guaproto.Gua/Heartbeat",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GuaServer).Heartbeat(ctx, req.(*Ping))
	}
	return interceptor(ctx, in, info, handler)
}

var _Gua_serviceDesc = grpc.ServiceDesc{
	ServiceName: "guaproto.Gua",
	HandlerType: (*GuaServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "NodeRegister",
			Handler:    _Gua_NodeRegister_Handler,
		},
		{
			MethodName: "JobReply",
			Handler:    _Gua_JobReply_Handler,
		},
		{
			MethodName: "Heartbeat",
			Handler:    _Gua_Heartbeat_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "gua.proto",
}
