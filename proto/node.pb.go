// Code generated by protoc-gen-go. DO NOT EDIT.
// source: node.proto

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

type RemoteCommandRequest struct {
	ExecCmd              []byte   `protobuf:"bytes,1,opt,name=exec_cmd,json=execCmd,proto3" json:"exec_cmd,omitempty"`
	JobId                string   `protobuf:"bytes,2,opt,name=job_id,json=jobId,proto3" json:"job_id,omitempty"`
	Timeout              int64    `protobuf:"varint,3,opt,name=timeout,proto3" json:"timeout,omitempty"`
	OtpCode              string   `protobuf:"bytes,4,opt,name=otp_code,json=otpCode,proto3" json:"otp_code,omitempty"`
	PlanTime             int64    `protobuf:"varint,5,opt,name=plan_time,json=planTime,proto3" json:"plan_time,omitempty"`
	ExecTime             int64    `protobuf:"varint,6,opt,name=exec_time,json=execTime,proto3" json:"exec_time,omitempty"`
	ExecJobMachineHost   string   `protobuf:"bytes,7,opt,name=exec_job_machine_host,json=execJobMachineHost,proto3" json:"exec_job_machine_host,omitempty"`
	ExecJobMachineMac    string   `protobuf:"bytes,8,opt,name=exec_job_machine_mac,json=execJobMachineMac,proto3" json:"exec_job_machine_mac,omitempty"`
	ExecJobMachineIp     string   `protobuf:"bytes,9,opt,name=exec_job_machine_ip,json=execJobMachineIp,proto3" json:"exec_job_machine_ip,omitempty"`
	GetJobMachineHost    string   `protobuf:"bytes,10,opt,name=get_job_machine_host,json=getJobMachineHost,proto3" json:"get_job_machine_host,omitempty"`
	GetJobMachineMac     string   `protobuf:"bytes,11,opt,name=get_job_machine_mac,json=getJobMachineMac,proto3" json:"get_job_machine_mac,omitempty"`
	GetJobMachineIp      string   `protobuf:"bytes,12,opt,name=get_job_machine_ip,json=getJobMachineIp,proto3" json:"get_job_machine_ip,omitempty"`
	GroupName            string   `protobuf:"bytes,13,opt,name=group_name,json=groupName,proto3" json:"group_name,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *RemoteCommandRequest) Reset()         { *m = RemoteCommandRequest{} }
func (m *RemoteCommandRequest) String() string { return proto.CompactTextString(m) }
func (*RemoteCommandRequest) ProtoMessage()    {}
func (*RemoteCommandRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_0c843d59d2d938e7, []int{0}
}

func (m *RemoteCommandRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_RemoteCommandRequest.Unmarshal(m, b)
}
func (m *RemoteCommandRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_RemoteCommandRequest.Marshal(b, m, deterministic)
}
func (m *RemoteCommandRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RemoteCommandRequest.Merge(m, src)
}
func (m *RemoteCommandRequest) XXX_Size() int {
	return xxx_messageInfo_RemoteCommandRequest.Size(m)
}
func (m *RemoteCommandRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_RemoteCommandRequest.DiscardUnknown(m)
}

var xxx_messageInfo_RemoteCommandRequest proto.InternalMessageInfo

func (m *RemoteCommandRequest) GetExecCmd() []byte {
	if m != nil {
		return m.ExecCmd
	}
	return nil
}

func (m *RemoteCommandRequest) GetJobId() string {
	if m != nil {
		return m.JobId
	}
	return ""
}

func (m *RemoteCommandRequest) GetTimeout() int64 {
	if m != nil {
		return m.Timeout
	}
	return 0
}

func (m *RemoteCommandRequest) GetOtpCode() string {
	if m != nil {
		return m.OtpCode
	}
	return ""
}

func (m *RemoteCommandRequest) GetPlanTime() int64 {
	if m != nil {
		return m.PlanTime
	}
	return 0
}

func (m *RemoteCommandRequest) GetExecTime() int64 {
	if m != nil {
		return m.ExecTime
	}
	return 0
}

func (m *RemoteCommandRequest) GetExecJobMachineHost() string {
	if m != nil {
		return m.ExecJobMachineHost
	}
	return ""
}

func (m *RemoteCommandRequest) GetExecJobMachineMac() string {
	if m != nil {
		return m.ExecJobMachineMac
	}
	return ""
}

func (m *RemoteCommandRequest) GetExecJobMachineIp() string {
	if m != nil {
		return m.ExecJobMachineIp
	}
	return ""
}

func (m *RemoteCommandRequest) GetGetJobMachineHost() string {
	if m != nil {
		return m.GetJobMachineHost
	}
	return ""
}

func (m *RemoteCommandRequest) GetGetJobMachineMac() string {
	if m != nil {
		return m.GetJobMachineMac
	}
	return ""
}

func (m *RemoteCommandRequest) GetGetJobMachineIp() string {
	if m != nil {
		return m.GetJobMachineIp
	}
	return ""
}

func (m *RemoteCommandRequest) GetGroupName() string {
	if m != nil {
		return m.GroupName
	}
	return ""
}

type RemoteCommandResponse struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *RemoteCommandResponse) Reset()         { *m = RemoteCommandResponse{} }
func (m *RemoteCommandResponse) String() string { return proto.CompactTextString(m) }
func (*RemoteCommandResponse) ProtoMessage()    {}
func (*RemoteCommandResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_0c843d59d2d938e7, []int{1}
}

func (m *RemoteCommandResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_RemoteCommandResponse.Unmarshal(m, b)
}
func (m *RemoteCommandResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_RemoteCommandResponse.Marshal(b, m, deterministic)
}
func (m *RemoteCommandResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RemoteCommandResponse.Merge(m, src)
}
func (m *RemoteCommandResponse) XXX_Size() int {
	return xxx_messageInfo_RemoteCommandResponse.Size(m)
}
func (m *RemoteCommandResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_RemoteCommandResponse.DiscardUnknown(m)
}

var xxx_messageInfo_RemoteCommandResponse proto.InternalMessageInfo

type RegisterCommandRequest struct {
	JobId                string   `protobuf:"bytes,1,opt,name=job_id,json=jobId,proto3" json:"job_id,omitempty"`
	OtpToken             string   `protobuf:"bytes,2,opt,name=otp_token,json=otpToken,proto3" json:"otp_token,omitempty"`
	OtpCode              string   `protobuf:"bytes,3,opt,name=otp_code,json=otpCode,proto3" json:"otp_code,omitempty"`
	GroupName            string   `protobuf:"bytes,4,opt,name=group_name,json=groupName,proto3" json:"group_name,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *RegisterCommandRequest) Reset()         { *m = RegisterCommandRequest{} }
func (m *RegisterCommandRequest) String() string { return proto.CompactTextString(m) }
func (*RegisterCommandRequest) ProtoMessage()    {}
func (*RegisterCommandRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_0c843d59d2d938e7, []int{2}
}

func (m *RegisterCommandRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_RegisterCommandRequest.Unmarshal(m, b)
}
func (m *RegisterCommandRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_RegisterCommandRequest.Marshal(b, m, deterministic)
}
func (m *RegisterCommandRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RegisterCommandRequest.Merge(m, src)
}
func (m *RegisterCommandRequest) XXX_Size() int {
	return xxx_messageInfo_RegisterCommandRequest.Size(m)
}
func (m *RegisterCommandRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_RegisterCommandRequest.DiscardUnknown(m)
}

var xxx_messageInfo_RegisterCommandRequest proto.InternalMessageInfo

func (m *RegisterCommandRequest) GetJobId() string {
	if m != nil {
		return m.JobId
	}
	return ""
}

func (m *RegisterCommandRequest) GetOtpToken() string {
	if m != nil {
		return m.OtpToken
	}
	return ""
}

func (m *RegisterCommandRequest) GetOtpCode() string {
	if m != nil {
		return m.OtpCode
	}
	return ""
}

func (m *RegisterCommandRequest) GetGroupName() string {
	if m != nil {
		return m.GroupName
	}
	return ""
}

type RegisterCommandReponse struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *RegisterCommandReponse) Reset()         { *m = RegisterCommandReponse{} }
func (m *RegisterCommandReponse) String() string { return proto.CompactTextString(m) }
func (*RegisterCommandReponse) ProtoMessage()    {}
func (*RegisterCommandReponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_0c843d59d2d938e7, []int{3}
}

func (m *RegisterCommandReponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_RegisterCommandReponse.Unmarshal(m, b)
}
func (m *RegisterCommandReponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_RegisterCommandReponse.Marshal(b, m, deterministic)
}
func (m *RegisterCommandReponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RegisterCommandReponse.Merge(m, src)
}
func (m *RegisterCommandReponse) XXX_Size() int {
	return xxx_messageInfo_RegisterCommandReponse.Size(m)
}
func (m *RegisterCommandReponse) XXX_DiscardUnknown() {
	xxx_messageInfo_RegisterCommandReponse.DiscardUnknown(m)
}

var xxx_messageInfo_RegisterCommandReponse proto.InternalMessageInfo

func init() {
	proto.RegisterType((*RemoteCommandRequest)(nil), "guaproto.RemoteCommandRequest")
	proto.RegisterType((*RemoteCommandResponse)(nil), "guaproto.RemoteCommandResponse")
	proto.RegisterType((*RegisterCommandRequest)(nil), "guaproto.RegisterCommandRequest")
	proto.RegisterType((*RegisterCommandReponse)(nil), "guaproto.RegisterCommandReponse")
}

func init() { proto.RegisterFile("node.proto", fileDescriptor_0c843d59d2d938e7) }

var fileDescriptor_0c843d59d2d938e7 = []byte{
	// 423 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x7c, 0x52, 0xdd, 0x6e, 0xd3, 0x30,
	0x14, 0x26, 0x74, 0x6b, 0x9a, 0xc3, 0xa6, 0x81, 0xb7, 0x82, 0xd9, 0x04, 0x44, 0xbd, 0xaa, 0x84,
	0x56, 0x04, 0x3c, 0x42, 0x2f, 0xa0, 0x48, 0xdb, 0x45, 0x34, 0x89, 0xcb, 0xc8, 0x89, 0x8f, 0xb2,
	0x0c, 0x9c, 0x63, 0x1a, 0x47, 0xe2, 0x09, 0x78, 0x24, 0x1e, 0x8d, 0x6b, 0xe4, 0x93, 0x4e, 0xd4,
	0x69, 0xd9, 0x5d, 0x7c, 0xbe, 0x1f, 0x7f, 0xf1, 0x77, 0x00, 0x1a, 0xd2, 0xb8, 0xb0, 0x6b, 0x72,
	0x24, 0x26, 0x55, 0xa7, 0xf8, 0x6b, 0xf6, 0x67, 0x04, 0x67, 0x19, 0x1a, 0x72, 0xb8, 0x24, 0x63,
	0x54, 0xa3, 0x33, 0xfc, 0xd1, 0x61, 0xeb, 0xc4, 0x4b, 0x98, 0xe0, 0x4f, 0x2c, 0xf3, 0xd2, 0x68,
	0x19, 0xa5, 0xd1, 0xfc, 0x28, 0x8b, 0xfd, 0x79, 0x69, 0xb4, 0x98, 0xc2, 0xf8, 0x8e, 0x8a, 0xbc,
	0xd6, 0xf2, 0x71, 0x1a, 0xcd, 0x93, 0xec, 0xf0, 0x8e, 0x8a, 0x95, 0x16, 0x12, 0x62, 0x57, 0x1b,
	0xa4, 0xce, 0xc9, 0x51, 0x1a, 0xcd, 0x47, 0xd9, 0xfd, 0xd1, 0x7b, 0x91, 0xb3, 0x79, 0x49, 0x1a,
	0xe5, 0x01, 0x4b, 0x62, 0x72, 0x76, 0x49, 0x1a, 0xc5, 0x05, 0x24, 0xf6, 0xbb, 0x6a, 0x72, 0x4f,
	0x95, 0x87, 0x2c, 0x9b, 0xf8, 0xc1, 0x4d, 0x6d, 0x18, 0xe4, 0x0c, 0x0c, 0x8e, 0x7b, 0xd0, 0x0f,
	0x18, 0x7c, 0x0f, 0x53, 0x06, 0x7d, 0x14, 0xa3, 0xca, 0xdb, 0xba, 0xc1, 0xfc, 0x96, 0x5a, 0x27,
	0x63, 0xbe, 0x41, 0x78, 0xf0, 0x0b, 0x15, 0x57, 0x3d, 0xf4, 0x99, 0x5a, 0x27, 0xde, 0xc1, 0xd9,
	0x8e, 0xc4, 0xa8, 0x52, 0x4e, 0x58, 0xf1, 0x2c, 0x54, 0x5c, 0xa9, 0x52, 0x5c, 0xc2, 0xe9, 0x8e,
	0xa0, 0xb6, 0x32, 0x61, 0xfe, 0xd3, 0x90, 0xbf, 0xb2, 0xde, 0xbf, 0x42, 0xb7, 0x9b, 0x08, 0x7a,
	0xff, 0x0a, 0xdd, 0x20, 0xd0, 0x25, 0x9c, 0x0e, 0x05, 0x3e, 0xcf, 0x93, 0xde, 0x3f, 0xe0, 0xfb,
	0x38, 0x6f, 0x41, 0x0c, 0xe9, 0xb5, 0x95, 0x47, 0xcc, 0x3e, 0x09, 0xd8, 0x2b, 0x2b, 0x5e, 0x01,
	0x54, 0x6b, 0xea, 0x6c, 0xde, 0x28, 0x83, 0xf2, 0x98, 0x49, 0x09, 0x4f, 0xae, 0x95, 0xc1, 0xd9,
	0x0b, 0x98, 0x0e, 0x7a, 0x6f, 0x2d, 0x35, 0x2d, 0xce, 0x7e, 0x45, 0xf0, 0x3c, 0xc3, 0xaa, 0x6e,
	0x1d, 0xae, 0x07, 0x3b, 0xf1, 0xaf, 0xf8, 0x68, 0xbb, 0xf8, 0x0b, 0x48, 0x7c, 0xbd, 0x8e, 0xbe,
	0x61, 0xb3, 0x59, 0x09, 0xdf, 0xf7, 0x8d, 0x3f, 0x07, 0xdd, 0x8f, 0xc2, 0xee, 0xc3, 0x84, 0x07,
	0xc3, 0x84, 0x72, 0x4f, 0x0e, 0x8e, 0xf8, 0xe1, 0x77, 0x04, 0xf1, 0xa7, 0x4e, 0x5d, 0x7b, 0x93,
	0x0c, 0x8e, 0x83, 0xff, 0x10, 0xaf, 0x17, 0xf7, 0xcb, 0xbd, 0xd8, 0xb7, 0xd8, 0xe7, 0x6f, 0xfe,
	0x8b, 0x6f, 0x1e, 0xe0, 0x91, 0xf8, 0x0a, 0x27, 0x83, 0x9b, 0x45, 0xba, 0xad, 0xda, 0xf7, 0x38,
	0xe7, 0x0f, 0x31, 0x36, 0xc6, 0xc5, 0x98, 0xf1, 0x8f, 0x7f, 0x03, 0x00, 0x00, 0xff, 0xff, 0x81,
	0x37, 0x8e, 0x6f, 0x8c, 0x03, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// GuaNodeClient is the client API for GuaNode service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type GuaNodeClient interface {
	RemoteCommand(ctx context.Context, in *RemoteCommandRequest, opts ...grpc.CallOption) (*RemoteCommandResponse, error)
	RegisterCommand(ctx context.Context, in *RegisterCommandRequest, opts ...grpc.CallOption) (*RegisterCommandReponse, error)
}

type guaNodeClient struct {
	cc *grpc.ClientConn
}

func NewGuaNodeClient(cc *grpc.ClientConn) GuaNodeClient {
	return &guaNodeClient{cc}
}

func (c *guaNodeClient) RemoteCommand(ctx context.Context, in *RemoteCommandRequest, opts ...grpc.CallOption) (*RemoteCommandResponse, error) {
	out := new(RemoteCommandResponse)
	err := c.cc.Invoke(ctx, "/guaproto.GuaNode/RemoteCommand", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *guaNodeClient) RegisterCommand(ctx context.Context, in *RegisterCommandRequest, opts ...grpc.CallOption) (*RegisterCommandReponse, error) {
	out := new(RegisterCommandReponse)
	err := c.cc.Invoke(ctx, "/guaproto.GuaNode/RegisterCommand", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// GuaNodeServer is the server API for GuaNode service.
type GuaNodeServer interface {
	RemoteCommand(context.Context, *RemoteCommandRequest) (*RemoteCommandResponse, error)
	RegisterCommand(context.Context, *RegisterCommandRequest) (*RegisterCommandReponse, error)
}

func RegisterGuaNodeServer(s *grpc.Server, srv GuaNodeServer) {
	s.RegisterService(&_GuaNode_serviceDesc, srv)
}

func _GuaNode_RemoteCommand_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RemoteCommandRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GuaNodeServer).RemoteCommand(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/guaproto.GuaNode/RemoteCommand",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GuaNodeServer).RemoteCommand(ctx, req.(*RemoteCommandRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _GuaNode_RegisterCommand_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RegisterCommandRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GuaNodeServer).RegisterCommand(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/guaproto.GuaNode/RegisterCommand",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GuaNodeServer).RegisterCommand(ctx, req.(*RegisterCommandRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _GuaNode_serviceDesc = grpc.ServiceDesc{
	ServiceName: "guaproto.GuaNode",
	HandlerType: (*GuaNodeServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "RemoteCommand",
			Handler:    _GuaNode_RemoteCommand_Handler,
		},
		{
			MethodName: "RegisterCommand",
			Handler:    _GuaNode_RegisterCommand_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "node.proto",
}
