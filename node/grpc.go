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
	cmd     string
	timeout int
	jobId   string
}
type Node struct {
	id        int64
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
		return nil, status.Errorf(codes.Aborted, "node busy")
	}

	return &guaproto.RemoteCommandResponse{}, nil
}
func (n *Node) RegisterCommand(ctx context.Context, req *guaproto.RegisterCommandRequest) (resp *guaproto.RegisterCommandReponse, err error) {
	if !totp.Validate(req.OtpCode, n.otpToken) {
		return nil, status.Errorf(codes.PermissionDenied, "otp code error")
	}

	err = n.localdb.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte("gua-node-bucket"))
		if err != nil {
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

		var output string
		if r.err != nil {
			output = r.err.Error()
		} else {
			output = r.output
		}
		replyRequest := &guaproto.JobReplyRequest{
			JobId:   command.jobId,
			Output:  output,
			OtpCode: passcode,
		}
		_, err := n.guaClient.JobReply(ctx, replyRequest)
		if err != nil {
			logger.Errorf("reply error: %#v", err)
		}
	}
}
