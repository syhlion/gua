package delayquene

import (
	"bytes"
	"context"
	"errors"
	fmt "fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yuin/gopher-lua/parse"

	"github.com/bwmarrin/snowflake"
	"github.com/gogo/protobuf/proto"
	"github.com/gomodule/redigo/redis"
	"github.com/pquerna/otp/totp"
	"github.com/syhlion/gua/luacore"
	guaproto "github.com/syhlion/gua/proto"
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

func New(config Config) (quene Quene, err error) {

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
	q := &q{
		node:       node,
		num:        num,
		bucketName: fmt.Sprintf(bucketNamePrefix, name) + "-%d",
		config:     config,
		timers:     make([]*time.Ticker, 10),
		lpool:      lpool,
		bucket:     bucket,
		jobQuene:   jobQuene,
		rpool:      rfr,
		once1:      &sync.Once{},
		once2:      &sync.Once{},
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
}

type Quene interface {
	GenerateId() (s string)
	Remove(jobId string) (err error)
	Push(job *guaproto.Job) (err error)
}

type q struct {
	node           *snowflake.Node
	num            int
	bucketName     string
	config         Config
	timers         []*time.Ticker
	bucketNameChan <-chan string
	bucket         *Bucket
	jobQuene       *JobQuene
	once1          *sync.Once
	once2          *sync.Once
	rpool          *redis.Pool
	lpool          *luacore.LStatePool
}

func (t *q) GenerateId() (s string) {
	return t.node.Generate().String()
}

func (t *q) Remove(jobId string) (err error) {
	return t.jobQuene.Remove(jobId)
}
func (t *q) RegisterNode(nodeInfo *guaproto.NodeRegisterRequest) (resp *guaproto.NodeRegisterResponse, err error) {
	conn := t.rpool.Get()
	defer conn.Close()
	id := t.node.Generate()
	resp = &guaproto.NodeRegisterResponse{
		NodeId: id.Int64(),
	}
	b, err := proto.Marshal(nodeInfo)
	if err != nil {
		return
	}
	remoteKey := fmt.Sprintf("REMOTE_NODE_%s", id)
	_, err = conn.Do("SET", remoteKey, b, "EX", 86400)
	return

}
func (t *q) Hearbeat(nodeId int64) (err error) {
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
			conn, err := grpc.Dial(nr.Hostname, grpc.WithInsecure())
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
func (t *q) readyQueneWorker() {
	c := t.rpool.Get()
	defer c.Close()
	var queneName string
	var data []byte
	for {

		//註冊兩個工作佇列
		err := c.Send("BLPOP", "GUA-READY-JOB", 0)
		if err != nil {
			return
		}
		err = c.Flush()
		if err != nil {
			return
		}

		//這邊會block住 等收訊息
		reply, err := redis.Values(c.Receive())
		if err != nil {
			//有可能因為timeout error  重新取一再跑一次迴圈
			log.Printf("redis receive fail, and got error: %#v\n", err)
			continue
		}
		if _, err := redis.Scan(reply, &queneName, &data); err != nil {
			log.Printf("redis scan fail, and got error: %#v\n", err)
			continue
		}
		job := &guaproto.Job{}
		err = proto.Unmarshal(data, job)
		if err != nil {
			log.Printf("json parse data fail, and got error: %#v\n", err)
			continue
		}
		ss := UrlRe.FindStringSubmatch(job.RequestUrl)
		cmdType := ss[1]
		switch cmdType {
		case "HTTP":
			u, err := url.Parse(job.RequestUrl)
			if err != nil {
				//TODO parse錯 工作是否要放回佇列
				continue
			}
			q := u.Query()
			q.Set("job_id", job.Id)
			q.Set("job_name", job.Name)
			u.RawQuery = q.Encode()
			resp, err := http.Get(u.String())
			if err != nil {
				//TODO parse錯 工作是否要放回佇列
				continue
			}
			resp.Body.Close()
		case "REMOTE":
			func() {
				nodeIdString := ss[2]
				cc := t.rpool.Get()
				defer cc.Close()
				nodeIds := strings.Split(nodeIdString, ",")
				for _, nodeId := range nodeIds {
					remoteKey := fmt.Sprintf("REMOTE_NODE_%s", nodeId)
					b, err := redis.Bytes(cc.Do("GET", remoteKey))
					if err != nil {
						//TODO parse錯 工作是否要放回佇列
						continue
					}
					nr := guaproto.NodeRegisterRequest{}
					err = proto.Unmarshal(b, &nr)
					if err != nil {
						//TODO parse錯 工作是否要放回佇列
						continue
					}
					conn, err := grpc.Dial(nr.Hostname, grpc.WithInsecure())
					if err != nil {
						//TODO parse錯 工作是否要放回佇列
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
						//TODO parse錯 工作是否要放回佇列
						continue
					}
				}
			}()

		case "LUA":

			l := t.lpool.Get()
			ctx, _ := context.WithTimeout(context.Background(), 60*time.Second)
			l.SetContext(ctx)
			err := l.DoString(string(job.ExecCmd))
			if err != nil {
				t.lpool.Put(l)
				//fmt.Println(err)
				continue
			}
			lfunc := l.GetGlobal(job.Name)
			switch r := lfunc.(type) {
			case *lua.LFunction:
				err := l.CallByParam(lua.P{
					Fn:      r,
					NRet:    1,
					Protect: true,
				}, lua.LNumber(job.Exectime), lua.LNumber(time.Now().Unix()))

				t.lpool.Put(l)
				if err != nil {
					continue
				}
				continue
			default:
				t.lpool.Put(l)
				fmt.Println("no func")
				continue
			}
		}

	}

}
func (t *q) runForReadQuene() {
	t.once1.Do(func() {
		for i := 0; i < 100; i++ {
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

func (t *q) delayQueneHandler(ti time.Time, realBucketName string) {
	for {
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

				//push to ready quene
				b, err := proto.Marshal(job)
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

	}

}
