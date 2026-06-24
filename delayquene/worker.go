package delayquene

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/sirupsen/logrus"
	"github.com/syhlion/greq"
	"github.com/syhlion/gua/loghook"
	guaproto "github.com/syhlion/gua/proto"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

type Worker struct {
	logger *logrus.Logger
	//ready quene
	rpool *redis.Pool
	//group pool
	realServerName string
	urpool         *redis.Pool
	jobReplyUrl    string
	machineHost    string
	machineMac     string
	machineIp      string
	httpClient     *greq.Client
	bucketName     string
	once1          *sync.Once
	once2          *sync.Once
	once3          *sync.Once

	timers               []*time.Ticker
	bucket               *Bucket
	jobQuene             *JobQuene
	bucketNameChan       <-chan string
	closeSign            []chan int
	closeSignForJobcheck chan int
	wait                 sync.WaitGroup
}

func (t *Worker) ExecuteJob(job *guaproto.ReadyJob) (err error) {
	execTime := time.Now()
	planTime := time.Unix(job.PlanTime, 0)
	var finishTime int64
	var cmdType string
	var resp string
	defer func() {
		if err == nil {
			fTime := time.Now()
			st := fTime.Sub(planTime)
			if st > 2*time.Second {
				t.logger.WithFields(
					logrus.Fields{
						"delay_time": fmt.Sprintf("%v", st),
						"job":        fmt.Sprintf("%v", job),
					}).Warn("job-delay finsh ready quene.")
			}

			t.logger.WithFields(logrus.Fields{
				"Exectime":           execTime.Unix(),
				"FinishTime":         finishTime,
				"PlanTime":           job.PlanTime,
				"GetJobTime":         job.GetJobTime,
				"JobId":              job.Id,
				"Type":               cmdType,
				"GetJobMachineHost":  job.GetJobMachineHost,
				"GetJobMachineIp":    job.GetJobMachineIp,
				"GetJobMachineMac":   job.GetJobMachineMac,
				"ExecJobMachineHost": t.machineHost,
				"ExecJobMachineMac":  t.machineMac,
				"ExecJobMachineIp":   t.machineIp,
				"GroupName":          job.GroupName,
			}).Info("Job Send Finish")
		}
		//如沒設定 reply hook 不執行
		if t.jobReplyUrl != "" {

			payload := &loghook.Payload{
				ExecTime:           execTime.Unix(),
				FinishTime:         finishTime,
				PlanTime:           job.PlanTime,
				GetJobTime:         job.GetJobTime,
				JobId:              job.Id,
				Type:               cmdType,
				GetJobMachineHost:  job.GetJobMachineHost,
				GetJobMachineIp:    job.GetJobMachineIp,
				GetJobMachineMac:   job.GetJobMachineMac,
				ExecJobMachineHost: t.machineHost,
				ExecJobMachineMac:  t.machineMac,
				ExecJobMachineIp:   t.machineIp,
				GroupName:          job.GroupName,
			}
			if err != nil {
				payload.Error = err.Error()
			} else {
				payload.Success = resp
			}
			b, _ := json.Marshal(payload)
			br := bytes.NewReader(b)

			_, _, err = t.httpClient.PostRaw(t.jobReplyUrl, br)
			if err != nil {
				t.logger.WithError(err).Errorf("job reply reqeust err. job: %#v. payload: %#v", job, payload)
			}

		}

	}()
	ss := UrlRe.FindStringSubmatch(job.RequestUrl)

	st := execTime.Sub(planTime)
	if st > 2*time.Second {
		t.logger.WithFields(
			logrus.Fields{
				"delay_time": fmt.Sprintf("%v", st),
				"job":        fmt.Sprintf("%v", job),
			}).Warn("job-delay receive ready quene.")
	}
	cmdType = ss[1]
	env := TriggerEnvelope{
		JobId:     job.Id,
		JobName:   job.Name,
		GroupName: job.GroupName,
		PlanTime:  job.PlanTime,
		ExecTime:  execTime.Unix(),
		Payload:   job.Payload,
	}
	switch cmdType {
	case "HTTP":
		body, _ := json.Marshal(env)
		respBody, status, herr := t.httpClient.PostRaw(ss[2], bytes.NewReader(body))
		if herr != nil {
			t.logger.WithError(herr).Errorf("http callback error. job:%#v", job)
			return herr
		}
		if status >= 400 {
			t.logger.Errorf("http callback non-2xx %d. job:%#v", status, job)
			return fmt.Errorf("http callback status %d", status)
		}
		resp = string(respBody)
	case "GRPC":
		timeout := time.Duration(job.Timeout) * time.Second
		if timeout <= 0 {
			timeout = 5 * time.Second
		}
		conn, derr := grpc.Dial(ss[2], grpc.WithInsecure())
		if derr != nil {
			t.logger.WithError(derr).Errorf("grpc dial error. addr:%s. job:%#v", ss[2], job)
			return derr
		}
		defer conn.Close()
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		res, gerr := guaproto.NewGuaCallbackClient(conn).OnJobTrigger(ctx, &guaproto.JobTrigger{
			JobId:     env.JobId,
			JobName:   env.JobName,
			GroupName: env.GroupName,
			PlanTime:  env.PlanTime,
			ExecTime:  env.ExecTime,
			Payload:   env.Payload,
		})
		if gerr != nil {
			t.logger.WithError(gerr).Errorf("grpc callback error. addr:%s. job:%#v", ss[2], job)
			return gerr
		}
		if res != nil {
			resp = res.Message
			if !res.Success {
				return fmt.Errorf("grpc callback reported failure: %s", res.Message)
			}
		}
	default:
		t.logger.Errorf("unsupported request type %q. job:%#v", cmdType, job)
		return fmt.Errorf("unsupported request type %q", cmdType)
	}
	finishTime = time.Now().Unix()
	return
}

// TriggerEnvelope is the payload delivered to a consumer when a job fires.
// HTTP delivery sends it as a JSON POST body; gRPC delivery maps it onto
// guaproto.JobTrigger. The two transports carry identical fields.
type TriggerEnvelope struct {
	JobId     string `json:"job_id"`
	JobName   string `json:"job_name"`
	GroupName string `json:"group_name"`
	PlanTime  int64  `json:"plan_time"`
	ExecTime  int64  `json:"exec_time"`
	Payload   string `json:"payload"`
}
func (t *Worker) ReadyQueneWorker() {
	for {
		func() {
			c := t.rpool.Get()
			defer c.Close()
			var queneName string
			var data []byte

			//這邊會block住 等收訊息
			reply, err := redis.Values(c.Do("BLPOP", "GUA-READY-JOB", 60))
			if err != nil {
				if err == redis.ErrNil {
					return
				}
				//有可能因為timeout error  重新取一再跑一次迴圈
				t.logger.WithError(err).Errorf("ready quenen redis receive fail")
				return
			}
			if _, err := redis.Scan(reply, &queneName, &data); err != nil {
				t.logger.WithError(err).Errorf("redis scan fail")
				return
			}
			job := &guaproto.ReadyJob{}
			err = proto.Unmarshal(data, job)
			if err != nil {
				t.logger.WithError(err).Errorf("proto unmarshal error")
				return
			}
			go func() {
				err = t.ExecuteJob(job)
				if err != nil {
					t.logger.WithError(err).Errorf("exec job error")
					return
				}
			}()
		}()

	}

}
func (t *Worker) Close() {
	t.bucket.Close()
	t.closeSignForJobcheck <- 1
	close(t.closeSignForJobcheck)
	for _, v := range t.closeSign {
		v <- 1
		close(v)
	}
	conn := t.bucket.rpool.Get()
	conn.Do("DEL", t.realServerName)
	conn.Close()
	t.wait.Wait()
}
func (t *Worker) RunForReadQuene() {
	t.once1.Do(func() {
		for i := 0; i < bucketSize; i++ {
			go t.ReadyQueneWorker()
		}

	})
}

func (t *Worker) DelayQueneWorker(timer *time.Ticker, closeSign chan int, realBucketName string) {
	defer func() {
		conn := t.bucket.rpool.Get()
		_, err := conn.Do("LPUSH", "down-server", realBucketName)
		if err != nil {
			t.logger.Errorf("down-server error%s", realBucketName)
		}
		t.logger.Infof("down-server:%s", realBucketName)
		conn.Close()
		t.wait.Done()
	}()
	for {
		select {
		case tt := <-timer.C:
			t.DelayQueneHandler(tt, realBucketName)
		case <-closeSign:
			t.logger.Infof("down bucket:%s", realBucketName)
			return

		}
	}
}
func (t *Worker) GenerateBucketName() <-chan string {
	c := make(chan string)
	go func() {
		i := 1
		for {
			c <- fmt.Sprintf(t.bucketName, i)
			if i >= bucketSize {
				i = 1
			} else {
				i++
			}
		}
	}()

	return c
}
func (t *Worker) RunJobCheck() {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	r := 30 + rng.Intn(30)
	t.logger.Info("JobCheck gap ", r, " Second")
	t.once3.Do(func() {
		timer := time.NewTicker(time.Duration(r) * time.Second)
		for {
			select {
			case tt := <-timer.C:

				err := t.bucket.JobCheck(<-t.bucketNameChan, tt, t.machineHost)
				if err != nil {
					t.logger.Error("run job check error", err)
				}

			case <-t.closeSignForJobcheck:
				t.logger.Info("JobCheck close")
				return

			}
		}
	})
}
func (t *Worker) RunForDelayQuene() {
	t.once2.Do(func() {
		for i := 0; i < bucketSize; i++ {
			t.timers[i] = time.NewTicker(700 * time.Millisecond)
			t.closeSign[i] = make(chan int, 1)
			realBucketName := fmt.Sprintf(t.bucketName, i+1)
			t.wait.Add(1)
			go t.DelayQueneWorker(t.timers[i], t.closeSign[i], realBucketName)
		}
	})
}

func (t *Worker) DelayQueneHandler(ti time.Time, realBucketName string) (err error) {

	bis, err := t.bucket.Get(realBucketName)
	if err != nil {
		return
	}
	for _, bi := range bis {
		if bi.Timestamp > ti.Unix() {
			return
		}
		func() {
			var err error

			job, err := t.jobQuene.Get(bi.JobId)
			if err != nil {
				if err == redis.ErrNil {
					t.bucket.Remove(realBucketName, bi.JobId)
					t.jobQuene.Remove(bi.JobId)
					t.logger.WithError(err).Errorf("jobQuene get error,remove job %s", bi.JobId)
					return
				}
				t.logger.WithError(err).Error("jobQuene get error")
				t.bucket.Remove(realBucketName, bi.JobId)
				return
			}

			if !job.Active {
				t.bucket.Remove(realBucketName, bi.JobId)
				return
			}

			if job.Exectime > ti.Unix() {
				t.bucket.RemoveAndPush(realBucketName, <-t.bucketNameChan, bi.JobId, job.Exectime)

				return
			}
			rj := &guaproto.ReadyJob{
				Name:              job.Name,
				Id:                job.Id,
				Timeout:           job.Timeout,
				RequestUrl:        job.RequestUrl,
				Payload:           job.Payload,
				PlanTime:          job.Exectime,
				GetJobTime:        time.Now().Unix(),
				GetJobMachineHost: t.machineHost,
				GetJobMachineMac:  t.machineMac,
				GetJobMachineIp:   t.machineIp,
				GroupName:         job.GroupName,
			}

			//push to ready quene
			b, err := proto.Marshal(rj)
			if err != nil {
				t.bucket.Remove(realBucketName, bi.JobId)
				t.logger.WithError(err).Errorf("push to ready quene marshal error job %v", job)
				return
			}

			c := t.rpool.Get()
			_, err = c.Do("RPUSH", "GUA-READY-JOB", b)
			c.Close()
			if err != nil {
				t.logger.WithError(err).Errorf("push to ready redis error job %v", job)
				return
			}

			//check delay
			planTime := time.Unix(job.Exectime, 0)
			st := ti.Sub(planTime)
			if st > 2*time.Second {
				t.logger.WithFields(
					logrus.Fields{
						"delay_time": fmt.Sprintf("%v", st),
						"job":        fmt.Sprintf("%v", job),
					}).Warn("job-delay push ready quene.")
			}

			//remove bucket
			t.bucket.Remove(realBucketName, bi.JobId)
			if job.IntervalPattern != "@once" {
				sch, _ := Parse(job.IntervalPattern)
				job.Exectime = sch.Next(ti).Unix()
				t.jobQuene.Add(bi.JobId, job)
				t.bucket.Push(<-t.bucketNameChan, job.Exectime, bi.JobId)
				t.jobQuene.Update(bi.JobId, job)
			} else {
				t.jobQuene.Remove(bi.JobId)
			}

		}()
	}
	return
}
