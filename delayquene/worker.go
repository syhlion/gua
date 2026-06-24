package delayquene

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"sync"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/pquerna/otp/totp"
	"github.com/sirupsen/logrus"
	"github.com/syhlion/greq"
	"github.com/syhlion/gua/loghook"
	guaproto "github.com/syhlion/gua/proto"
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
	switch cmdType {
	case "HTTP":
		u, err := url.Parse(ss[2])
		if err != nil {
			t.logger.WithError(err).Errorf("http url parse error. job:%#v", job)
			return err
		}
		q := u.Query()
		passcode, _ := totp.GenerateCode(job.OtpToken, time.Now())
		q.Set("otp_code", passcode)
		q.Set("job_id", job.Id)
		q.Set("job_name", job.Name)

		u.RawQuery = q.Encode()
		_, _, err = t.httpClient.Get(u.String(), nil)
		if err != nil {
			t.logger.WithError(err).Errorf("http get error. job:%#v", job)
			return err
		}
	default:
		t.logger.Errorf("unsupported request type %q. job:%#v", cmdType, job)
		return fmt.Errorf("unsupported request type %q", cmdType)
	}
	finishTime = time.Now().Unix()
	return
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
			var token string
			if job.OtpToken == "" {
				conn := t.urpool.Get()
				defer conn.Close()
				groupKey := fmt.Sprintf("USER_%s", job.GroupName)
				token, err = redis.String(conn.Do("GET", groupKey))
				if err != nil {
					t.bucket.Remove(realBucketName, bi.JobId)
					t.logger.WithError(err).Errorf("optToken error job %v", job)
					return
				}
			} else {
				token = job.OtpToken
			}
			rj := &guaproto.ReadyJob{
				Name:              job.Name,
				Id:                job.Id,
				OtpToken:          token,
				Timeout:           job.Timeout,
				RequestUrl:        job.RequestUrl,
				ExecCmd:           job.ExecCmd,
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
