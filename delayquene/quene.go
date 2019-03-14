package delayquene

import (
	"bytes"
	"context"
	"errors"
	fmt "fmt"
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
	"github.com/syhlion/gua/luacore"
	guaproto "github.com/syhlion/gua/proto"
	requestwork "github.com/syhlion/requestwork.v2"
	"google.golang.org/grpc"
)

const bucketSize = 30

// bucket-{uuid}-{[0-9]}
var bucketNamePrefix = "BUCKET-[%s]"

// JOB-{groupName}-{jobId}
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
	//init user redis pool
	urpool := redis.NewPool(func() (redis.Conn, error) {
		c, err := redis.Dial("tcp", config.RedisForGroupAddr)
		if err != nil {
			return nil, err
		}
		_, err = c.Do("SELECT", config.RedisForGroupDBNo)
		if err != nil {
			c.Close()
			return nil, err
		}
		return c, nil
	}, 10)
	urpool.MaxIdle = config.RedisForGroupMaxIdle
	urpool.MaxActive = config.RedisForGroupMaxConn
	urpconn := urpool.Get()
	defer urpconn.Close()

	// Test the connection
	_, err = urpconn.Do("PING")
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
	rfdconn := rfd.Get()
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
	worker := &Worker{
		bucketName:  fmt.Sprintf(bucketNamePrefix, name) + "-%d",
		timers:      make([]*time.Ticker, bucketSize),
		rpool:       rfr,
		urpool:      urpool,
		once1:       &sync.Once{},
		once2:       &sync.Once{},
		httpClient:  client,
		bucket:      bucket,
		jobQuene:    jobQuene,
		jobReplyUrl: config.JobReplyUrl,
		machineHost: config.MachineHost,
		machineMac:  config.MachineMac,
		machineIp:   config.MachineIp,
		logger:      config.Logger,
	}
	bucketChan := worker.GenerateBucketName()
	worker.bucketNameChan = bucketChan

	go worker.RunForDelayQuene()
	go worker.RunForReadQuene()
	q := &q{
		node:           node,
		num:            num,
		config:         config,
		lpool:          lpool,
		bucket:         bucket,
		jobQuene:       jobQuene,
		rpool:          rfr,
		urpool:         urpool,
		worker:         worker,
		bucketNameChan: bucketChan,
	}
	// delay quene
	//go q.runForDelayQuene()
	//go q.runForReadQuene()

	return q, nil
}

type Config struct {
	RedisForGroupAddr         string
	RedisForGroupDBNo         int
	RedisForGroupMaxIdle      int
	RedisForGroupMaxConn      int
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
	Heartbeat(nodeId string, groupName string) (err error)
	GenerateUID() (s string)
	Remove(jobId string) (err error)
	Push(job *guaproto.Job) (err error)
	RegisterNode(nodeInfo *guaproto.NodeRegisterRequest) (resp *guaproto.NodeRegisterResponse, err error)
	RegisterGroup(groupName string) (otpToken string, err error)
	QueryNode(nodeId string, groupName string) (resp *guaproto.NodeRegisterRequest, err error)
}

type q struct {
	bucketNameChan <-chan string
	worker         *Worker
	node           *snowflake.Node
	num            int
	config         *Config
	bucket         *Bucket
	jobQuene       *JobQuene
	rpool          *redis.Pool
	urpool         *redis.Pool
	lpool          *luacore.LStatePool
	//httpClient     *greq.Client
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
func (t *q) RegisterGroup(groupName string) (otpToken string, err error) {
	conn := t.urpool.Get()
	defer conn.Close()
	kkey, err := totp.Generate(totp.GenerateOpts{
		Issuer:      groupName,
		AccountName: t.worker.bucketName,
	})

	groupKey := fmt.Sprintf("USER_%s", groupName)
	v, err := redis.Int(conn.Do("SETNX", groupKey, kkey.Secret()))
	if err != nil {
		return
	}
	if v == 0 {
		err = errors.New("duplicate key")
		return
	}
	return kkey.Secret(), nil

}

func (t *q) QueryNode(nodeId string, groupName string) (resp *guaproto.NodeRegisterRequest, err error) {
	conn := t.urpool.Get()
	defer conn.Close()

	remoteKey := fmt.Sprintf("REMOTE_NODE_%s_%s", groupName, nodeId)
	b, err := redis.Bytes(conn.Do("GET", remoteKey))
	if err != nil {
		return
	}
	resp = &guaproto.NodeRegisterRequest{}
	err = proto.Unmarshal(b, resp)
	return

}
func (t *q) RegisterNode(nodeInfo *guaproto.NodeRegisterRequest) (resp *guaproto.NodeRegisterResponse, err error) {
	conn := t.rpool.Get()
	defer conn.Close()

	uconn := t.urpool.Get()
	defer uconn.Close()
	groupKey := fmt.Sprintf("USER_%s", nodeInfo.GroupName)
	token, err := redis.String(uconn.Do("GET", groupKey))
	if err != nil {
		return
	}
	if token == "" {
		err = errors.New("NO GROUP")
		return
	}
	id := t.node.Generate().String()
	nodeId := fmt.Sprintf("%s@%s@%s", id, nodeInfo.Ip, nodeInfo.Hostname)
	resp = &guaproto.NodeRegisterResponse{
		NodeId: nodeId,
	}
	b, err := proto.Marshal(nodeInfo)
	if err != nil {
		return
	}
	remoteKey := fmt.Sprintf("REMOTE_NODE_%s_%s", nodeInfo.GroupName, nodeId)
	_, err = conn.Do("SET", remoteKey, b, "EX", 86400)
	return

}
func (t *q) Heartbeat(nodeId string, groupName string) (err error) {
	conn := t.rpool.Get()
	defer conn.Close()
	remoteKey := fmt.Sprintf("REMOTE_NODE_%s_%s", groupName, nodeId)

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
		err = func() (err error) {
			nodeIdString := ss[2]
			nodeIds := strings.Split(nodeIdString, ",")
			for _, nodeId := range nodeIds {

				remoteKey := fmt.Sprintf("REMOTE_NODE_%s_%s", job.GroupName, nodeId)
				b, err := redis.Bytes(cc.Do("GET", remoteKey))
				if err != nil {
					t.config.Logger.Warnf("NO REMOTE NODE:%s\n", err)
					return errors.New("NO REMOTE NODE")
				}

				nodeInfo := guaproto.NodeRegisterRequest{}
				err = proto.Unmarshal(b, &nodeInfo)
				if err != nil {
					return err
				}
				var addr string
				if nodeInfo.BoradcastAddr != "" {
					addr = nodeInfo.BoradcastAddr
				} else {
					ss := strings.Split(nodeInfo.Grpclisten, ":")
					addr = nodeInfo.Ip + ":" + ss[1]

				}
				conn, err := grpc.Dial(addr, grpc.WithInsecure())
				if err != nil {
					return err
				}
				defer conn.Close()
				passcode, _ := totp.GenerateCode(nodeInfo.OtpToken, time.Now())
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
			return
		}()
		if err != nil {
			return
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
	err = t.jobQuene.Add(fmt.Sprintf(jobNamePrefix, job.GroupName, job.Id), job)
	if err != nil {
		return
	}

	return t.bucket.Push(<-t.bucketNameChan, job.Exectime, fmt.Sprintf(jobNamePrefix, job.GroupName, job.Id))
}
