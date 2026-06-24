package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/syhlion/gua/delayquene"
	guaproto "github.com/syhlion/gua/proto"
)

// GuaAdmin implements the guaproto.GuaAdminServer gRPC API, mirroring the
// HTTP REST admin endpoints. It is a thin adapter over delayquene.Quene.
type GuaAdmin struct {
	quene delayquene.Quene
}

func deliveryPrefix(d guaproto.DeliveryType) (string, error) {
	switch d {
	case guaproto.DeliveryType_HTTP:
		return "HTTP", nil
	case guaproto.DeliveryType_GRPC:
		return "GRPC", nil
	default:
		return "", fmt.Errorf("unknown delivery type %v", d)
	}
}

func buildRequestURL(d guaproto.DeliveryType, target string) (string, error) {
	p, err := deliveryPrefix(d)
	if err != nil {
		return "", err
	}
	if target == "" {
		return "", errors.New("empty target")
	}
	return p + "@" + target, nil
}

func parseRequestURL(requestURL string) (guaproto.DeliveryType, string) {
	if strings.HasPrefix(requestURL, "GRPC@") {
		return guaproto.DeliveryType_GRPC, strings.TrimPrefix(requestURL, "GRPC@")
	}
	return guaproto.DeliveryType_HTTP, strings.TrimPrefix(requestURL, "HTTP@")
}

func (g *GuaAdmin) RegisterGroup(ctx context.Context, req *guaproto.GroupRequest) (*guaproto.Empty, error) {
	if err := g.quene.RegisterGroup(req.GroupName); err != nil {
		return nil, err
	}
	return &guaproto.Empty{}, nil
}

func (g *GuaAdmin) RemoveGroup(ctx context.Context, req *guaproto.GroupRequest) (*guaproto.Empty, error) {
	if err := g.quene.RemoveGroup(req.GroupName); err != nil {
		return nil, err
	}
	return &guaproto.Empty{}, nil
}

func (g *GuaAdmin) AddJob(ctx context.Context, req *guaproto.AddJobRequest) (*guaproto.AddJobResponse, error) {
	if req.Name == "" {
		return nil, errors.New("no name")
	}
	if req.ExecTime < 0 {
		return nil, errors.New("exec_time error")
	}
	if req.IntervalPattern == "" {
		return nil, errors.New("no interval_pattern")
	}
	if req.GroupName == "" {
		return nil, errors.New("no group_name")
	}
	requestURL, err := buildRequestURL(req.Delivery, req.Target)
	if err != nil {
		return nil, err
	}
	exists, err := g.quene.ExistsGroup(req.GroupName)
	if err != nil {
		return nil, err
	}
	if exists != 1 {
		return nil, errors.New("no group")
	}
	jobID := req.JobId
	if jobID == "" {
		jobID = g.quene.GenerateUID()
	}
	job := &guaproto.Job{
		Name:            req.Name,
		GroupName:       req.GroupName,
		Id:              jobID,
		Exectime:        req.ExecTime,
		Timeout:         req.Timeout,
		IntervalPattern: req.IntervalPattern,
		RequestUrl:      requestURL,
		Payload:         req.Payload,
		Active:          true,
		Memo:            req.Memo,
	}
	if err := g.quene.Push(job); err != nil {
		return nil, err
	}
	return &guaproto.AddJobResponse{JobId: jobID}, nil
}

func (g *GuaAdmin) EditJob(ctx context.Context, req *guaproto.EditJobRequest) (*guaproto.Empty, error) {
	requestURL, err := buildRequestURL(req.Delivery, req.Target)
	if err != nil {
		return nil, err
	}
	if err := g.quene.Edit(req.GroupName, req.JobId, requestURL, req.Payload); err != nil {
		return nil, err
	}
	return &guaproto.Empty{}, nil
}

func (g *GuaAdmin) DeleteJob(ctx context.Context, req *guaproto.JobRef) (*guaproto.Empty, error) {
	if err := g.quene.Delete(req.GroupName, req.JobId); err != nil {
		return nil, err
	}
	return &guaproto.Empty{}, nil
}

func (g *GuaAdmin) PauseJob(ctx context.Context, req *guaproto.JobRef) (*guaproto.Empty, error) {
	if err := g.quene.Pause(req.GroupName, req.JobId); err != nil {
		return nil, err
	}
	return &guaproto.Empty{}, nil
}

func (g *GuaAdmin) ActiveJob(ctx context.Context, req *guaproto.ActiveJobRequest) (*guaproto.Empty, error) {
	if err := g.quene.Active(req.GroupName, req.JobId, req.ExecTime); err != nil {
		return nil, err
	}
	return &guaproto.Empty{}, nil
}

func (g *GuaAdmin) ListJobs(ctx context.Context, req *guaproto.GroupRequest) (*guaproto.ListJobsResponse, error) {
	jobs, err := g.quene.List(req.GroupName)
	if err != nil {
		return nil, err
	}
	resp := &guaproto.ListJobsResponse{Jobs: make([]*guaproto.JobInfo, 0, len(jobs))}
	for _, j := range jobs {
		delivery, target := parseRequestURL(j.RequestUrl)
		resp.Jobs = append(resp.Jobs, &guaproto.JobInfo{
			Name:            j.Name,
			Id:              j.Id,
			ExecTime:        j.Exectime,
			IntervalPattern: j.IntervalPattern,
			Delivery:        delivery,
			Target:          target,
			Payload:         j.Payload,
			GroupName:       j.GroupName,
			Active:          j.Active,
			Memo:            j.Memo,
		})
	}
	return resp, nil
}
