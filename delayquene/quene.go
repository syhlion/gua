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
var bucketNamePrefix = "BUCKET-[%s-%s]"

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
		//先做第一次時間更新
		c.Do("SET", s, time.Now().Unix())
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

func New(config *Config, groupRedis *redis.Pool, readyRedis *redis.Pool, delayRedis *redis.Pool) (quene Quene, err error) {

	// init lua pool
	lpool := luacore.New()

	num, name, err := initName(delayRedis)
	if err != nil {
		return
	}
	go func() {
		t := time.NewTicker(1 * time.Second)
		for {
			select {
			case <-t.C:
				func() {
					conn := delayRedis.Get()
					defer conn.Close()
					conn.Do("SET", name, time.Now().Unix())
				}()
			}
		}
	}()

	conn := delayRedis.Get()
	ks, err := redis.Strings(conn.Do("keys", name+"-*"))
	if err != nil {
		conn.Close()
		return
	}
	if len(ks) > 0 {
		fmt.Println("HHHH")
		for _, v := range ks {
			kkeys := fmt.Sprintf("BUCKET-\\[%s\\]", v)
			kks, err := redis.Strings(conn.Do("keys", kkeys+"-*"))
			if err != nil {
				return nil, err
			}
			for _, vv := range kks {
				fmt.Println(vv)
				_, err := conn.Do("LPUSH", "down-server", vv)
				if err != nil {
					conn.Close()
					return nil, err
				}
			}
			conn.Do("DEL", v)
		}
	}
	conn.Close()

	bucket := &Bucket{
		rpool:  delayRedis,
		logger: config.Logger,
	}
	jobQuene := &JobQuene{
		rpool: delayRedis,
	}
	node, err := snowflake.NewNode(int64(num))
	if err != nil {
		return
	}
	work := requestwork.New(100)
	client := greq.New(work, 60*time.Second, true)
	tt := time.Now().UnixNano()
	ts := strconv.FormatInt(tt, 10)
	conn = delayRedis.Get()
	_, err = conn.Do("SET", name+"-"+ts, tt)
	if err != nil {
		conn.Close()
		return
	}
	conn.Close()
	worker := &Worker{
		bucketName:     fmt.Sprintf(bucketNamePrefix, name, ts) + "-%d",
		realServerName: name + "-" + ts,
		timers:         make([]*time.Ticker, bucketSize),
		rpool:          readyRedis,
		urpool:         groupRedis,
		once1:          &sync.Once{},
		once2:          &sync.Once{},
		httpClient:     client,
		bucket:         bucket,
		jobQuene:       jobQuene,
		jobReplyUrl:    config.JobReplyUrl,
		machineHost:    config.MachineHost,
		machineMac:     config.MachineMac,
		machineIp:      config.MachineIp,
		logger:         config.Logger,
		closeSign:      make([]chan int, bucketSize),
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
		qpool:          delayRedis,
		rpool:          readyRedis,
		urpool:         groupRedis,
		worker:         worker,
		bucketNameChan: bucketChan,
	}

	return q, nil
}

type Config struct {
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
	Heartbeat(nodeId string, groupName string) (err error)
	GenerateUID() (s string)
	Remove(jobId string) (err error)
	Edit(groupName, jobId, requestUrl, execCmd string) (err error)
	Active(groupName string, jobId string, exectime int64) (err error)
	Pause(groupName string, jobId string) (err error)
	Delete(groupName string, jobId string) (err error)
	List(groupName string) (jobs []*guaproto.Job, err error)
	Push(job *guaproto.Job) (err error)
	RegisterNode(nodeInfo *guaproto.NodeRegisterRequest) (resp *guaproto.NodeRegisterResponse, err error)
	RegisterGroup(groupName string) (otpToken string, err error)
	QueryNodes(groupName string) (nodes []*guaproto.NodeRegisterRequest, err error)
	QueryGroups() (s []string, err error)
	GroupInfo(groupName string) (s string, err error)
	Close()
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
	qpool          *redis.Pool
	urpool         *redis.Pool
	lpool          *luacore.LStatePool
	//httpClient     *greq.Client
}

func (t *q) Close() {
	t.worker.Close()
}

func (t *q) GenerateUID() (s string) {
	return t.node.Generate().String()
}

func (t *q) Remove(jobId string) (err error) {
	return t.jobQuene.Remove(jobId)
}
func (t *q) GroupInfo(groupName string) (s string, err error) {
	conn := t.urpool.Get()
	defer conn.Close()
	groupKey := fmt.Sprintf("USER_%s", groupName)
	return redis.String(conn.Do("GET", groupKey))
}
func (t *q) QueryGroups() (groups []string, err error) {
	conn := t.urpool.Get()
	defer conn.Close()
	return redis.Strings(conn.Do("KEYS", "USER_*"))

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

func (t *q) QueryNodes(groupName string) (nodes []*guaproto.NodeRegisterRequest, err error) {
	conn := t.urpool.Get()
	defer conn.Close()

	remoteKey := fmt.Sprintf("REMOTE_NODE_%s_*", groupName)
	keys, err := redis.Strings(conn.Do("KEYS", remoteKey))
	if err != nil {
		return
	}
	nodes = make([]*guaproto.NodeRegisterRequest, 0)
	for _, v := range keys {
		b, err := redis.Bytes(conn.Do("GET", v))
		if err != nil {
			continue
		}
		node := &guaproto.NodeRegisterRequest{}
		err = proto.Unmarshal(b, node)
		if err != nil {
			continue
		}
		nodes = append(nodes, node)
	}
	return

}
func (t *q) RegisterNode(nodeInfo *guaproto.NodeRegisterRequest) (resp *guaproto.NodeRegisterResponse, err error) {

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
	_, err = uconn.Do("SET", remoteKey, b, "EX", 86400)
	return

}
func (t *q) Edit(groupName, Id, requestUrl, execCmd string) (err error) {
	job, err := t.jobQuene.Get(fmt.Sprintf(jobNamePrefix, groupName, Id))
	if err != nil {
		return
	}
	job.RequestUrl = requestUrl
	job.ExecCmd = []byte(execCmd)
	cc := t.urpool.Get()
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
				fmt.Println(nodeInfo)
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
	return
}
func (t *q) Active(groupName string, jobId string, exectime int64) (err error) {
	conn := t.qpool.Get()
	defer conn.Close()
	for {
		jobKey := fmt.Sprintf(jobNamePrefix, groupName, jobId)
		_, err := conn.Do("WATCH", jobKey)
		if err != nil {
			return err
		}
		b, err := redis.Bytes(conn.Do("GET", jobKey))
		if err != nil {
			return err
		}
		job := &guaproto.Job{}
		err = proto.Unmarshal(b, job)
		if err != nil {
			return err
		}
		job.Active = true
		job.Exectime = exectime
		bb, err := proto.Marshal(job)
		if err != nil {
			return err
		}
		err = conn.Send("MULTI")
		if err != nil {
			return err
		}
		conn.Send("SET", jobKey, bb)
		reply, err := conn.Do("EXEC")
		if err == nil && reply != nil {
			t.bucket.Push(<-t.bucketNameChan, exectime, fmt.Sprintf(jobNamePrefix, job.GroupName, job.Id))
			break
		}
	}
	return

}
func (t *q) Pause(groupName string, jobId string) (err error) {
	conn := t.qpool.Get()
	defer conn.Close()
	for {
		jobKey := fmt.Sprintf(jobNamePrefix, groupName, jobId)
		_, err := conn.Do("WATCH", jobKey)
		if err != nil {
			fmt.Println("gg", jobKey)
			return err
		}
		b, err := redis.Bytes(conn.Do("GET", jobKey))
		if err != nil {
			return err
		}
		job := &guaproto.Job{}
		err = proto.Unmarshal(b, job)
		if err != nil {
			return err
		}
		job.Active = false
		bb, err := proto.Marshal(job)
		if err != nil {
			return err
		}
		err = conn.Send("MULTI")
		if err != nil {
			return err
		}
		conn.Send("SET", jobKey, bb)
		reply, err := conn.Do("EXEC")
		if err == nil && reply != nil {
			break
		}
	}
	return

}
func (t *q) List(groupName string) (jobs []*guaproto.Job, err error) {
	jobKey := fmt.Sprintf(jobNamePrefix, groupName, "*")
	return t.jobQuene.List(jobKey)
}
func (t *q) Delete(groupName, jobId string) (err error) {
	conn := t.qpool.Get()
	defer conn.Close()
	jobKey := fmt.Sprintf(jobNamePrefix, groupName, jobId)
	_, err = conn.Do("DEL", jobKey)
	return
}
func (t *q) Heartbeat(nodeId string, groupName string) (err error) {
	conn := t.urpool.Get()
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

	cc := t.urpool.Get()
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
				fmt.Println(nodeInfo)
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
