package delayquene

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	fmt "fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/yuin/gopher-lua/parse"

	"github.com/bwmarrin/snowflake"
	"github.com/gogo/protobuf/proto"
	"github.com/gomodule/redigo/redis"
	"github.com/pquerna/otp/totp"
	"github.com/syhlion/greq"
	"github.com/syhlion/gua/admin"
	"github.com/syhlion/gua/luacore"
	guaproto "github.com/syhlion/gua/proto"
	requestwork "github.com/syhlion/requestwork.v2"
	lua "github.com/yuin/gopher-lua"
	"google.golang.org/grpc"
)

const bucketSize = 30

// bucket-{uuid}-{[0-9]}
var bucketNamePrefix = "BUCKET-[%s]"

// job-{jobId}-{job func name}
var jobNamePrefix = "JOB-%s-%s"
var re = regexp.MustCompile(`^SERVER-(\d+)`)
var UrlRe = regexp.MustCompile(`^(HTTP|REMOTE|LUA)\@(.+)?`)

func incrServerNum(c redis.Conn) (num int, s string, err error) {
	serverNum, err := redis.Int(c.Do("INCR", "SERVER_TOTAL"))
	if err != nil {
		return
	}
	return serverNum, "SERVER-" + strconv.Itoa(serverNum), nil
}
func initName(pool *redis.Pool) (serverNum int, s string, err error) {

	now := time.Now()
	c := pool.Get()

	defer func() {
		//解鎖
		c.Do("DEL", "STARTLOCK")
		c.Close()
	}()

	//確認同時間只有一台在啟動
	for {
		//搶鎖 & 上鎖
		check, err := redis.Int(c.Do("SETNX", "STARTLOCK", 1))
		if err != nil {
			return 0, "", err
		}
		if check == 1 {
			break
		}
		time.Sleep(1 * time.Second)
	}
	replys, err := redis.Values(c.Do("keys", "SERVER-*"))
	if err != nil {

		if err == redis.ErrNil {
			return incrServerNum(c)
		}
	}
	for _, v := range replys {
		serverName, err := redis.String(v, nil)
		if err != nil {
			return 0, "", err
		}
		lastTime, err := redis.Int64(c.Do("GET", serverName))
		if err != nil {
			return 0, "", err
		}
		if now.Unix()-lastTime > 2 {
			ss := re.FindStringSubmatch(serverName)
			if len(ss) != 2 {
				return 0, "", errors.New("server name match error")
			}
			num, err := strconv.Atoi(ss[1])
			if err != nil {
				return 0, "", err
			}

			return num, serverName, nil
		}

	}
	return incrServerNum(c)

}

func New(config *Config) (quene Quene, err error) {

	// init lua pool
	lpool := luacore.New()

	//init ready quene redis pool
	rfr := redis.NewPool(func() (redis.Conn, error) {
		c, err := redis.Dial("tcp", config.RedisForReadyAddr)
		if err != nil {
			return nil, err
		}
		_, err = c.Do("SELECT", config.RedisForReadyDBNo)
		if err != nil {
			c.Close()
			return nil, err
		}
		return c, nil
	}, 10)
	rfr.MaxIdle = config.RedisForReadyMaxIdle
	rfr.MaxActive = config.RedisForReadyMaxConn
	rfrconn := rfr.Get()
	defer rfrconn.Close()

	// Test the connection
	_, err = rfrconn.Do("PING")
	if err != nil {
		return
	}

	//init delay quene redis pool
	rfd := redis.NewPool(func() (redis.Conn, error) {
		c, err := redis.Dial("tcp", config.RedisForDelayQueneAddr)
		if err != nil {
			return nil, err
		}
		_, err = c.Do("SELECT", config.RedisForDelayQueneDBNo)
		if err != nil {
			c.Close()
			return nil, err
		}
		return c, nil
	}, 10)
	rfd.MaxIdle = config.RedisForDelayQueneMaxIdle
	rfd.MaxActive = config.RedisForDelayQueneMaxConn
	rfdconn := rfr.Get()
	defer rfdconn.Close()

	// Test the connection
	_, err = rfdconn.Do("PING")
	if err != nil {
		return
	}
	num, name, err := initName(rfd)
	if err != nil {
		return
	}
	go func() {
		t := time.NewTicker(1 * time.Second)
		for {
			select {
			case <-t.C:
				conn := rfd.Get()
				conn.Do("SET", name, time.Now().Unix())
				conn.Close()
			}
		}
	}()
	bucket := &Bucket{
		rpool: rfd,
	}
	jobQuene := &JobQuene{
		rpool: rfd,
	}
	node, err := snowflake.NewNode(int64(num))
	if err != nil {
		return
	}
	work := requestwork.New(100)
	client := greq.New(work, 60*time.Second, true)
	q := &q{
		node:       node,
		num:        num,
		bucketName: fmt.Sprintf(bucketNamePrefix, name) + "-%d",
		config:     config,
		timers:     make([]*time.Ticker, bucketSize),
		lpool:      lpool,
		bucket:     bucket,
		jobQuene:   jobQuene,
		rpool:      rfr,
		once1:      &sync.Once{},
		once2:      &sync.Once{},
		httpClient: client,
	}
	q.bucketNameChan = q.generateBucketName()
	// delay quene
	go q.runForDelayQuene()
	go q.runForReadQuene()

	return q, nil
}

type Config struct {
	RedisForReadyAddr         string
	RedisForReadyDBNo         int
	RedisForReadyMaxIdle      int
	RedisForReadyMaxConn      int
	RedisForDelayQueneAddr    string
	RedisForDelayQueneDBNo    int
	RedisForDelayQueneMaxIdle int
	RedisForDelayQueneMaxConn int
	MachineHost               string
	MachineMac                string
	MachineIp                 string
	JobReplyUrl               string
	Logger                    *logrus.Logger
}

type Quene interface {
	GetDelayQueneRedis() (pool *redis.Pool)
	Heartbeat(nodeId string) (err error)
	GenerateUID() (s string)
	Remove(jobId string) (err error)
	Push(job *guaproto.Job) (err error)
	RegisterNode(nodeInfo *guaproto.NodeRegisterRequest) (resp *guaproto.NodeRegisterResponse, err error)
}

type q struct {
	node           *snowflake.Node
	num            int
	bucketName     string
	config         *Config
	timers         []*time.Ticker
	bucketNameChan <-chan string
	bucket         *Bucket
	jobQuene       *JobQuene
	once1          *sync.Once
	once2          *sync.Once
	rpool          *redis.Pool
	lpool          *luacore.LStatePool
	httpClient     *greq.Client
}

func (t *q) GetDelayQueneRedis() (pool *redis.Pool) {
	return t.rpool
}
func (t *q) GenerateUID() (s string) {
	return t.node.Generate().String()
}

func (t *q) Remove(jobId string) (err error) {
	return t.jobQuene.Remove(jobId)
}
func (t *q) RegisterNode(nodeInfo *guaproto.NodeRegisterRequest) (resp *guaproto.NodeRegisterResponse, err error) {
	conn := t.rpool.Get()
	defer conn.Close()
	id := t.node.Generate().String()
	resp = &guaproto.NodeRegisterResponse{
		NodeId: id,
	}
	b, err := proto.Marshal(nodeInfo)
	if err != nil {
		return
	}
	remoteKey := fmt.Sprintf("REMOTE_NODE_%s", id)
	_, err = conn.Do("SET", remoteKey, b, "EX", 86400)
	return

}
func (t *q) Heartbeat(nodeId string) (err error) {
	conn := t.rpool.Get()
	defer conn.Close()
	remoteKey := fmt.Sprintf("REMOTE_NODE_%s", nodeId)
	reply, err := redis.Int(conn.Do("EXPIRE", remoteKey, 86400))
	if err != nil {
		return
	}
	if reply == 0 {
		return errors.New("NO_REMOTE_NODE")
	}
	return

}
func (t *q) Push(job *guaproto.Job) (err error) {

	cc := t.rpool.Get()
	defer cc.Close()
	ss := UrlRe.FindStringSubmatch(job.RequestUrl)
	cmdType := ss[1]
	switch cmdType {
	case "HTTP":
	case "REMOTE":
		nodeIdString := ss[2]
		nodeIds := strings.Split(nodeIdString, ",")
		for _, nodeId := range nodeIds {

			remoteKey := fmt.Sprintf("REMOTE_NODE_%s", nodeId)
			b, err := redis.Bytes(cc.Do("GET", remoteKey))
			if err != nil {
				return err
			}
			nr := guaproto.NodeRegisterRequest{}
			err = proto.Unmarshal(b, &nr)
			if err != nil {
				return err
			}
			var addr string
			if nr.BoradcastAddr != "" {
				addr = nr.BoradcastAddr
			} else {
				ss := strings.Split(nr.Grpclisten, ":")
				addr = nr.Ip + ":" + ss[1]

			}
			conn, err := grpc.Dial(addr, grpc.WithInsecure())
			if err != nil {
				return err
			}
			passcode, _ := totp.GenerateCode(nr.OtpToken, time.Now())
			nodeClient := guaproto.NewGuaNodeClient(conn)
			ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)

			cmdReq := &guaproto.RegisterCommandRequest{
				JobId:    job.Id,
				OtpToken: job.OtpToken,
				OtpCode:  passcode,
			}
			_, err = nodeClient.RegisterCommand(ctx, cmdReq)
			if err != nil {
				return err
			}
		}
	case "LUA":
		reader := bytes.NewReader(job.ExecCmd)
		_, err = parse.Parse(reader, "<string>")
		if err != nil {
			return
		}

	default:
		err = errors.New("type error")
		return
	}
	err = t.jobQuene.Add(job.Id, job)
	if err != nil {
		return
	}

	//TODO 如果是 remote 模式 要註冊 node 是否存在
	return t.bucket.Push(<-t.bucketNameChan, job.Exectime, job.Id)
}
func (t *q) executeJob(job *guaproto.ReadyJob) (err error) {
	execTime := time.Now().Unix()
	var finishTime int64
	var cmdType string
	var resp string
	defer func() {
		//如沒設定 reply hook 不執行
		if t.config.JobReplyUrl != "" {

			payload := &admin.Payload{
				ExecTime:           execTime,
				FinishTime:         finishTime,
				PlanTime:           job.PlanTime,
				GetJobTime:         job.GetJobTime,
				JobId:              job.Id,
				Type:               cmdType,
				GetJobMachineHost:  job.GetJobMachineHost,
				GetJobMachineIp:    job.GetJobMachineIp,
				GetJobMachineMac:   job.GetJobMachineMac,
				ExecJobMachineHost: t.config.MachineHost,
				ExecJobMachineMac:  t.config.MachineMac,
				ExecJobMachineIp:   t.config.MachineIp,
			}
			if err != nil {
				payload.Error = err.Error()
			} else {
				payload.Success = resp
			}
			b, _ := json.Marshal(payload)
			br := bytes.NewReader(b)

			_, _, err = t.httpClient.PostRaw(t.config.JobReplyUrl, br)
			if err != nil {
				t.config.Logger.WithError(err).Errorf("job reply reqeust err. job: %#v. payload: %#v", job, payload)
			}

		}
	}()
	ss := UrlRe.FindStringSubmatch(job.RequestUrl)
	cmdType = ss[1]
	switch cmdType {
	case "HTTP":
		u, err := url.Parse(ss[2])
		if err != nil {
			t.config.Logger.WithError(err).Errorf("http url parse error. job:%#v", job)
			return err
		}
		passcode, _ := totp.GenerateCode(job.OtpToken, time.Now())
		q := u.Query()
		q.Set("job_id", job.Id)
		q.Set("job_name", job.Name)
		q.Set("otp_code", passcode)

		u.RawQuery = q.Encode()
		_, _, err = t.httpClient.Get(u.String(), nil)
		if err != nil {
			t.config.Logger.WithError(err).Errorf("http get error. job:%#v", job)
			return err
		}
	case "REMOTE":
		nodeIdString := ss[2]
		cc := t.rpool.Get()
		defer cc.Close()
		nodeIds := strings.Split(nodeIdString, ",")
		for _, nodeId := range nodeIds {
			remoteKey := fmt.Sprintf("REMOTE_NODE_%s", nodeId)
			b, err := redis.Bytes(cc.Do("GET", remoteKey))
			if err != nil {
				t.config.Logger.WithError(err).Errorf("remote nodeinfo get error. remotekey:%s. job:%#v", remoteKey, job)
				continue
			}
			nr := guaproto.NodeRegisterRequest{}
			err = proto.Unmarshal(b, &nr)
			if err != nil {
				t.config.Logger.WithError(err).Errorf("nodeinfo unmarshal error. remotekey:%s. job:%#v", remoteKey, job)
				continue
			}
			var addr string
			if nr.BoradcastAddr != "" {
				addr = nr.BoradcastAddr
			} else {
				ss := strings.Split(nr.Grpclisten, ":")
				addr = nr.Ip + ":" + ss[1]

			}
			conn, err := grpc.Dial(addr, grpc.WithInsecure())
			if err != nil {
				t.config.Logger.WithError(err).Errorf("nodeinfo connect error. remotekey:%s. job:%#v", remoteKey, job)
				continue
			}
			passcode, _ := totp.GenerateCode(job.OtpToken, time.Now())
			nodeClient := guaproto.NewGuaNodeClient(conn)
			ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
			cmdReq := &guaproto.RemoteCommandRequest{
				ExecCmd: job.ExecCmd,
				JobId:   job.Id,
				Timeout: job.Timeout,
				OtpCode: passcode,
			}
			_, err = nodeClient.RemoteCommand(ctx, cmdReq)
			if err != nil {
				t.config.Logger.WithError(err).Errorf("nodeinfo exec error. remotekey:%s. job:%#v", remoteKey, job)
				continue
			}
		}

	case "LUA":

		l := t.lpool.Get()
		ctx, _ := context.WithTimeout(context.Background(), 60*time.Second)
		l.SetContext(ctx)
		err := l.DoString(string(job.ExecCmd))
		if err != nil {
			t.lpool.Put(l)
			t.config.Logger.WithError(err).Errorf("lua compile error.  job:%#v", job)
			return err
		}
		lfunc := l.GetGlobal(job.Name)
		switch r := lfunc.(type) {
		case *lua.LFunction:
			err := l.CallByParam(lua.P{
				Fn:      r,
				NRet:    1,
				Protect: true,
			}, lua.LNumber(job.PlanTime), lua.LNumber(execTime), lua.LString(job.Id))

			t.lpool.Put(l)
			if err != nil {
				t.config.Logger.WithError(err).Errorf("lua exec error.  job:%#v", job)
				return err
			}
		default:
			t.lpool.Put(l)
			t.config.Logger.Errorf("lua no func. job:%#v", job)
			return errors.New("lua no fuc")
		}
	}
	finishTime = time.Now().Unix()
	return
}
func (t *q) readyQueneWorker() {
	c := t.rpool.Get()
	defer c.Close()
	var queneName string
	var data []byte
	for {

		//註冊兩個工作佇列
		err := c.Send("BLPOP", "GUA-READY-JOB", 0)
		if err != nil {
			t.config.Logger.WithError(err).Errorf("redis pop error")
			continue
		}
		err = c.Flush()
		if err != nil {
			t.config.Logger.WithError(err).Errorf("redis flush error")
			continue
		}

		//這邊會block住 等收訊息
		reply, err := redis.Values(c.Receive())
		if err != nil {
			//有可能因為timeout error  重新取一再跑一次迴圈
			t.config.Logger.WithError(err).Errorf("redis receive fail")
			continue
		}
		if _, err := redis.Scan(reply, &queneName, &data); err != nil {
			t.config.Logger.WithError(err).Errorf("redis scan fail")
			continue
		}
		job := &guaproto.ReadyJob{}
		err = proto.Unmarshal(data, job)
		if err != nil {
			t.config.Logger.WithError(err).Errorf("proto unmarshal error")
			continue
		}
		err = t.executeJob(job)
		if err != nil {
			t.config.Logger.WithError(err).Errorf("exec job error")
			continue
		}

	}

}
func (t *q) runForReadQuene() {
	t.once1.Do(func() {
		for i := 0; i < bucketSize; i++ {
			go t.readyQueneWorker()
		}

	})
}

func (t *q) delayQueneWorker(timer *time.Ticker, realBucketName string) {
	for {
		select {
		case tt := <-timer.C:
			t.delayQueneHandler(tt, realBucketName)

		}
	}
}
func (t *q) generateBucketName() <-chan string {
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

func (t *q) runForDelayQuene() {
	t.once2.Do(func() {
		for i := 0; i < bucketSize; i++ {
			t.timers[i] = time.NewTicker(1 * time.Second)
			realBucketName := fmt.Sprintf(t.bucketName, i+1)
			go t.delayQueneWorker(t.timers[i], realBucketName)
		}
	})
}

func (t *q) delayQueneHandler(ti time.Time, realBucketName string) (err error) {

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
			defer func() {
				if err != nil {
					t.bucket.Remove(realBucketName, bi.JobId)
				}
			}()
			job, err := t.jobQuene.Get(bi.JobId)
			if err != nil {
				t.bucket.Remove(realBucketName, bi.JobId)
				t.jobQuene.Remove(bi.JobId)
				return
			}

			if job.Exectime > ti.Unix() {
				t.bucket.Remove(realBucketName, bi.JobId)
				t.bucket.Push(<-t.bucketNameChan, job.Exectime, bi.JobId)
				return
			}
			rj := &guaproto.ReadyJob{
				Name:              job.Name,
				Id:                job.Id,
				OtpToken:          job.OtpToken,
				Timeout:           job.Timeout,
				RequestUrl:        job.RequestUrl,
				ExecCmd:           job.ExecCmd,
				PlanTime:          job.Exectime,
				GetJobTime:        time.Now().Unix(),
				GetJobMachineHost: t.config.MachineHost,
				GetJobMachineMac:  t.config.MachineMac,
				GetJobMachineIp:   t.config.MachineIp,
			}

			//push to ready quene
			b, err := proto.Marshal(rj)
			if err != nil {
				//fmt.Println(err)
				return
			}
			c := t.rpool.Get()
			c.Do("RPUSH", "GUA-READY-JOB", b)
			c.Close()
			//remove bucket

			t.bucket.Remove(realBucketName, bi.JobId)
			if job.IntervalPattern != "@once" {
				sch, _ := Parse(job.IntervalPattern)
				job.Exectime = sch.Next(time.Now()).Unix()
				t.jobQuene.Add(bi.JobId, job)
				t.bucket.Push(<-t.bucketNameChan, job.Exectime, bi.JobId)
			}
		}()
	}

	return
}
