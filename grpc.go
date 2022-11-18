package main

import (
	"bytes"
	"context"
	"fmt"

	"github.com/gogo/protobuf/proto"
	"github.com/gomodule/redigo/redis"
	"github.com/pquerna/otp/totp"
	"github.com/syhlion/greq"
	"github.com/syhlion/gua/delayquene"
	"github.com/syhlion/gua/loghook"
	guaproto "github.com/syhlion/gua/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Gua struct {
	config     *Config
	quene      delayquene.Quene
	httpClient *greq.Client
	rpool      *redis.Pool
}

func (g *Gua) mustEmbedUnimplementedGuaServer() {
}

func (g *Gua) NodeRegister(ctx context.Context, req *guaproto.NodeRegisterRequest) (resp *guaproto.NodeRegisterResponse, err error) {
	logger.Infof("receive node register. request:%#v", req)
	if req.MachineCode != g.config.MachineCode {
		return nil, status.Error(codes.PermissionDenied, "machine code error")
	}
	return g.quene.RegisterNode(req)
}
func (g *Gua) JobReply(ctx context.Context, req *guaproto.JobReplyRequest) (resp *guaproto.JobReplyResponse, err error) {
	conn := g.rpool.Get()
	defer conn.Close()
	logger.Infof("receive jobreply. request:%#v", req)
	remoteKey := fmt.Sprintf("REMOTE_NODE_%s_%s", req.GroupName, req.NodeId)
	b, err := redis.Bytes(conn.Do("GET", remoteKey))
	if err != nil {
		return nil, status.Error(codes.PermissionDenied, "machine code error")
	}
	nodeInfo := &guaproto.NodeRegisterRequest{}

	err = proto.Unmarshal(b, nodeInfo)
	if err != nil {
		return nil, status.Error(codes.Aborted, "nodeinfo unmarshal error")
	}
	if !totp.Validate(req.OtpCode, nodeInfo.OtpToken) {
		return nil, status.Error(codes.PermissionDenied, "otp error")
	}
	if g.config.JobReplyHook != "" {
		payload := &loghook.Payload{
			ExecTime:           req.ExecTime,
			FinishTime:         req.FinishTime,
			PlanTime:           req.PlanTime,
			GetJobTime:         req.GetJobTime,
			JobId:              req.JobId,
			Type:               "NODE",
			GetJobMachineHost:  req.GetJobMachineHost,
			GetJobMachineIp:    req.GetJobMachineIp,
			GetJobMachineMac:   req.GetJobMachineMac,
			ExecJobMachineHost: req.ExecJobMachineHost,
			ExecJobMachineMac:  req.ExecJobMachineMac,
			ExecJobMachineIp:   req.ExecJobMachineIp,
			RemoteNodeHost:     nodeInfo.Hostname,
			RemoteNodeMac:      nodeInfo.Mac,
			RemoteNodeIp:       nodeInfo.Ip,
			Error:              req.Error,
			Success:            req.Success,
		}
		b, _ := json.Marshal(payload)
		br := bytes.NewReader(b)

		_, _, err = g.httpClient.PostRaw(g.config.JobReplyHook, br)
		if err != nil {
			logger.WithError(err).Errorf("job reply reqeust err. job: %s. payload: %#v", req.JobId, payload)
		}

		return &guaproto.JobReplyResponse{}, nil
	}
	return &guaproto.JobReplyResponse{}, nil

}
func (g *Gua) Heartbeat(ctx context.Context, req *guaproto.Ping) (resp *guaproto.Pong, err error) {
	logger.Infof("receive node heartbeat. request:%#v", req)
	err = g.quene.Heartbeat(req.NodeId, req.GroupName)
	if err != nil {
		return nil, err
	}
	return &guaproto.Pong{}, nil

}
