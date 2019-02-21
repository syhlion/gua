package main

import (
	"context"
	"fmt"
	"os/exec"
	"syscall"
	"time"

	"github.com/pquerna/otp/totp"
	guaproto "github.com/syhlion/gua/proto"
	bolt "go.etcd.io/bbolt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Result struct {
	output string
	err    error
}
type Command struct {
	planTime           int64
	execTime           int64
	getJobTime         int64
	execJobMachineHost string
	execJobMachineMac  string
	execJobMachineIp   string
	getJobMachineHost  string
	getJobMachineMac   string
	getJobMachineIp    string
	cmd                string
	timeout            int
	jobId              string
}
type Node struct {
	id        string
	config    *Config
	cmdChan   chan Command
	workerNum int
	//httpClient  *greq.Client
	guaClient guaproto.GuaClient
	localdb   *bolt.DB
	otpToken  string
}

func (n *Node) RemoteCommand(ctx context.Context, req *guaproto.RemoteCommandRequest) (resp *guaproto.RemoteCommandResponse, err error) {

	var otpToken string
	err = n.localdb.View(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte("gua-node-bucket"))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		v := b.Get([]byte(req.JobId))
		otpToken = string(v)

		return nil
	})
	if err != nil {
		return
	}
	if !totp.Validate(req.OtpCode, otpToken) {
		logger.Warn("remote command auth error. request:%#v, otp-token:%s", req, n.otpToken)
		return nil, status.Errorf(codes.PermissionDenied, "otp code error")
	}
	cmd := Command{
		cmd:     string(req.ExecCmd),
		timeout: int(req.Timeout),
		jobId:   req.JobId,
	}
	select {
	case n.cmdChan <- cmd:
	default:
		logger.Warn("server busy. request:%#v", req)
		return nil, status.Errorf(codes.Aborted, "node busy")
	}

	return &guaproto.RemoteCommandResponse{}, nil
}
func (n *Node) RegisterCommand(ctx context.Context, req *guaproto.RegisterCommandRequest) (resp *guaproto.RegisterCommandReponse, err error) {
	if !totp.Validate(req.OtpCode, n.otpToken) {
		logger.Warn("register command auth error. request:%#v, otp-token:%s", req, n.otpToken)
		return nil, status.Errorf(codes.PermissionDenied, "otp code error")
	}

	err = n.localdb.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte("gua-node-bucket"))
		if err != nil {
			logger.WithError(err).Warn("local db update error. request:%#v", req)
			return fmt.Errorf("create bucket: %s", err)
		}
		b.Put([]byte(req.JobId), []byte(req.OtpToken))

		return nil
	})
	return
}
func (n *Node) exec(ctx context.Context, command string) (r Result) {
	cmd := exec.Command("/bin/bash", "-c", command)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	resultChan := make(chan Result)
	go func() {
		output, err := cmd.CombinedOutput()
		fmt.Printf("PID: %d", cmd.Process.Pid)
		resultChan <- Result{string(output), err}
	}()
	select {
	case <-ctx.Done():
		if cmd.Process.Pid > 0 {
			fmt.Println(cmd.Process.Pid)
			syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		r = Result{
			err:    ctx.Err(),
			output: "",
		}
		return
	case result := <-resultChan:
		return result
	}
}

func (n *Node) run() {
	for i := 0; i < n.workerNum; i++ {
		go n.worker()
	}
}

func (n *Node) worker() {
	var duration time.Duration
	for command := range n.cmdChan {
		if command.timeout <= 0 || time.Duration(command.timeout) > 24*time.Hour {
			duration = 24 * time.Hour
		} else {
			duration = time.Duration(command.timeout)
		}
		ctx, _ := context.WithTimeout(context.Background(), duration)
		r := n.exec(ctx, command.cmd)
		passcode, _ := totp.GenerateCode(n.otpToken, time.Now())
		ctx, _ = context.WithTimeout(context.Background(), 5*time.Second)

		replyRequest := &guaproto.JobReplyRequest{
			JobId:              command.jobId,
			OtpCode:            passcode,
			PlanTime:           command.planTime,
			ExecTime:           command.execTime,
			FinishTime:         time.Now().Unix(),
			ExecJobMachineHost: command.execJobMachineHost,
			ExecJobMachineMac:  command.execJobMachineMac,
			ExecJobMachineIp:   command.execJobMachineIp,
			GetJobMachineHost:  command.getJobMachineHost,
			GetJobMachineMac:   command.getJobMachineMac,
			GetJobMachineIp:    command.getJobMachineIp,
		}
		if r.err != nil {
			replyRequest.Error = r.err.Error()
		} else {
			replyRequest.Success = r.output
		}
		_, err := n.guaClient.JobReply(ctx, replyRequest)
		if err != nil {
			logger.Errorf("reply error: %#v, reply request:%#v", err, replyRequest)
		}
	}
}
