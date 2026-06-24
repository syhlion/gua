package delayquene

import (
	"errors"
	fmt "fmt"
	"regexp"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/bwmarrin/snowflake"
	"github.com/gomodule/redigo/redis"
	"github.com/syhlion/gua/internal/httpclient"
	guaproto "github.com/syhlion/gua/proto"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

const bucketSize = 80

// bucket-{uuid}-{[0-9]}
var bucketNamePrefix = "BUCKET-[%s-%s]"

// JOB-{groupName}-{jobId}
var jobNamePrefix = "JOB-%s-%s"
var re = regexp.MustCompile(`^SERVER-(\d+)$`)

// var jobCheckRe = regexp.MustCompile(`^JOB-([a-zA-Z0-9_]+)-([a-zA-Z0-9_]+)-scan$`)
var jobRe = regexp.MustCompile(`^JOB-([a-zA-Z0-9_]+)-([a-zA-Z0-9_]+)$`)
var UrlRe = regexp.MustCompile(`^(HTTP|GRPC)\@(.+)?`)

// ownerKeyOf returns the fencing-token key for a server slot. The "OWN-" prefix
// keeps it out of the "SERVER-N-*" namespace that New() scans+deletes on start.
func ownerKeyOf(serverName string) string { return "OWN-" + serverName }

type servers []string

func (s servers) Len() int {
	return len(s)
}
func (s servers) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s servers) Less(i, j int) bool {
	s1 := re.FindStringSubmatch(s[i])
	s2 := re.FindStringSubmatch(s[j])
	if len(s1) == 0 || len(s2) == 0 {
		return s[i] < s[j]
	}
	a1, _ := strconv.Atoi(s1[1])
	a2, _ := strconv.Atoi(s2[1])
	return a1 < a2
}

func incrServerNum(c redis.Conn) (num int, s string, err error) {
	serverNum, err := redis.Int(c.Do("INCR", "SERVER_TOTAL"))
	if err != nil {
		return
	}
	return serverNum, "SERVER-" + strconv.Itoa(serverNum), nil
}
func initName(pool *redis.Pool) (serverNum int, s string, ownerToken string, err error) {

	now := time.Now()
	c := pool.Get()
	lockToken := newToken()
	ownerToken = newToken()

	defer func() {
		if s != "" {
			//認領這個 slot:寫入第一次心跳 + 擁有權 token。
			//reclaim 時這會覆蓋舊持有者的 token,使其下次心跳 CAS 失敗而停機。
			//擁有權 key 用 "OWN-" 前綴,避開 New() 對 "SERVER-N-*" 的清理掃描。
			c.Do("SET", s, time.Now().Unix())
			c.Do("SET", ownerKeyOf(s), ownerToken)
		}
		//解鎖(只刪自己持有的鎖)
		releaseLock(c, "STARTLOCK", lockToken)
		c.Close()
	}()

	//確認同時間只有一台在啟動(TTL 鎖:持有者崩潰也會自動釋放,不會死鎖)
	for {
		ok, lerr := acquireLock(c, "STARTLOCK", lockToken, 30000)
		if lerr != nil {
			return 0, "", ownerToken, lerr
		}
		if ok {
			break
		}
		time.Sleep(1 * time.Second)
	}
	replys, err := RedisScan(c, "SERVER-*")
	if err != nil {

		if err == redis.ErrNil {
			num, name, e := incrServerNum(c)
			return num, name, ownerToken, e
		}
	}

	//正規式篩出 SERVER-1 排除 SERVER-1-123456 與 SERVER-1-OWN
	serverlist := make([]string, 0)
	for _, v := range replys {
		if re.MatchString(v) {
			serverlist = append(serverlist, v)
		}

	}
	sort.Sort(servers(serverlist))
	for _, serverName := range serverlist {
		lastTime, err := redis.Int64(c.Do("GET", serverName))
		if err != nil {
			return 0, "", ownerToken, err
		}

		tlastTime := time.Unix(lastTime, 0)
		// 30s 接管窗口(心跳每 1s,留較大安全邊際以縮小活著的節點被誤判接管、
		// 進而 snowflake node-id 撞號的風險)。startup 由 STARTLOCK 序列化。
		if now.Sub(tlastTime) > 30*time.Second {
			ss := re.FindStringSubmatch(serverName)
			if len(ss) != 2 {
				return 0, "", ownerToken, errors.New("server name match error")
			}
			num, err := strconv.Atoi(ss[1])
			if err != nil {
				return 0, "", ownerToken, err
			}

			return num, serverName, ownerToken, nil
		}

	}
	num, name, e := incrServerNum(c)
	return num, name, ownerToken, e

}

func New(config *Config, groupRedis *redis.Pool, readyRedis *redis.Pool, delayRedis *redis.Pool) (quene Quene, err error) {

	num, name, ownerToken, err := initName(delayRedis)
	if err != nil {
		return
	}
	go func() {
		t := time.NewTicker(1 * time.Second)
		defer t.Stop()
		for range t.C {
			superseded := func() bool {
				conn := delayRedis.Get()
				defer conn.Close()
				ok, herr := redis.Int(heartbeatScript.Do(conn, ownerKeyOf(name), name, ownerToken, time.Now().Unix()))
				if herr != nil {
					// transient redis error: don't treat as supersession
					return false
				}
				return ok == 0
			}()
			if superseded {
				config.Logger.Errorf("server slot %s superseded by another node; stopping heartbeat to avoid snowflake node-id collision", name)
				if config.OnSupersede != nil {
					config.OnSupersede()
				}
				return
			}
		}
	}()

	conn := delayRedis.Get()
	ks, err := RedisScan(conn, name+"-*")
	if err != nil {
		conn.Close()
		return
	}
	if len(ks) > 0 {
		for _, v := range ks {
			kkeys := fmt.Sprintf("BUCKET-\\[%s\\]", v)
			kks, err := RedisScan(conn, kkeys+"-*")
			if err != nil {
				return nil, err
			}
			for _, vv := range kks {
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
		rpool:    delayRedis,
		logger:   config.Logger,
		lock:     &sync.RWMutex{},
		stopFlag: 0,
	}
	jobQuene := &JobQuene{
		rpool: delayRedis,
	}
	node, err := snowflake.NewNode(int64(num))
	if err != nil {
		return
	}
	client := httpclient.New(60*time.Second, false)
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
		bucketName:           fmt.Sprintf(bucketNamePrefix, name, ts) + "-%d",
		realServerName:       name + "-" + ts,
		timers:               make([]*time.Ticker, bucketSize),
		rpool:                readyRedis,
		urpool:               groupRedis,
		once1:                &sync.Once{},
		once2:                &sync.Once{},
		once3:                &sync.Once{},
		httpClient:           client,
		bucket:               bucket,
		jobQuene:             jobQuene,
		jobReplyUrl:          config.JobReplyUrl,
		historyTTL:           config.HistoryTTL,
		machineHost:          config.MachineHost,
		machineMac:           config.MachineMac,
		machineIp:            config.MachineIp,
		logger:               config.Logger,
		closeSign:            make([]chan int, bucketSize),
		closeSignForJobcheck: make(chan int, 1),
		grpcConns:            make(map[string]*grpc.ClientConn),
	}
	bucketChan := worker.GenerateBucketName()
	worker.bucketNameChan = bucketChan

	go worker.RunForDelayQuene()
	go worker.RunForReadQuene()
	go worker.RunJobCheck()
	q := &q{
		node:           node,
		num:            num,
		config:         config,
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
	HistoryTTL                int       // seconds; 0 disables execution-history recording
	OnSupersede               func()    // called when this node's slot is reclaimed by another (default: fatal exit)
	Logger                    *logrus.Logger
}

type Quene interface {
	GenerateUID() (s string)
	Remove(jobId string) (err error)
	Edit(groupName, jobId, requestUrl, payload string) (err error)
	Active(groupName string, jobId string, exectime int64) (err error)
	Pause(groupName string, jobId string) (err error)
	Delete(groupName string, jobId string) (err error)
	List(groupName string) (jobs []*guaproto.Job, err error)
	Push(job *guaproto.Job) (err error)
	RegisterGroup(groupName string) (err error)
	QueryGroups() (s []string, err error)
	GroupInfo(groupName string) (s string, err error)
	Close()
	RemoveGroup(groupName string) (err error)
	ExistsGroup(groupName string) (exists int, err error)
	Stats() (s *Stats, err error)
	History(group string, limit int) (entries []*HistoryEntry, err error)
}

// ServerStat is the health of a single cluster slot (SERVER-N).
type ServerStat struct {
	Name          string `json:"name"`
	LastHeartbeat int64  `json:"last_heartbeat"`
	Alive         bool   `json:"alive"`
}

// Stats is a read-only snapshot of cluster + queue health for monitoring.
type Stats struct {
	Now               int64        `json:"now"`
	ReadyQueueDepth   int          `json:"ready_queue_depth"`
	DownServerBacklog int          `json:"down_server_backlog"`
	Servers           []ServerStat `json:"servers"`
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
	//httpClient     *greq.Client
}

func (t *q) Close() {
	t.worker.Close()
}

// Stats returns a read-only health snapshot: ready-queue depth, the orphaned
// bucket backlog, and every cluster slot with its last heartbeat (alive if it
// beat within the 15s takeover window).
func (t *q) Stats() (s *Stats, err error) {
	now := time.Now().Unix()
	s = &Stats{Now: now, Servers: []ServerStat{}}

	rc := t.rpool.Get()
	defer rc.Close()
	if s.ReadyQueueDepth, err = redis.Int(rc.Do("LLEN", "GUA-READY-JOB")); err != nil {
		return nil, err
	}

	dc := t.qpool.Get()
	defer dc.Close()
	if s.DownServerBacklog, err = redis.Int(dc.Do("LLEN", "down-server")); err != nil {
		return nil, err
	}
	keys, err := RedisScan(dc, "SERVER-*")
	if err != nil {
		return nil, err
	}
	for _, k := range keys {
		if !re.MatchString(k) {
			continue
		}
		last, gerr := redis.Int64(dc.Do("GET", k))
		if gerr != nil {
			continue
		}
		s.Servers = append(s.Servers, ServerStat{
			Name:          k,
			LastHeartbeat: last,
			Alive:         now-last < 15,
		})
	}
	return s, nil
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
	return RedisScan(conn, "USER_*")
}
func (t *q) RegisterGroup(groupName string) (err error) {
	conn := t.urpool.Get()
	defer conn.Close()

	groupKey := fmt.Sprintf("USER_%s", groupName)
	v, err := redis.Int(conn.Do("SETNX", groupKey, "1"))
	if err != nil {
		return
	}
	if v == 0 {
		err = errors.New("duplicate key")
		return
	}
	return nil

}

func (t *q) Edit(groupName, Id, requestUrl, payload string) (err error) {
	job, err := t.jobQuene.Get(fmt.Sprintf(jobNamePrefix, groupName, Id))
	if err != nil {
		return
	}
	job.RequestUrl = requestUrl
	job.Payload = payload
	ss := UrlRe.FindStringSubmatch(job.RequestUrl)
	if len(ss) == 0 {
		return errors.New("type error")
	}
	cmdType := ss[1]
	switch cmdType {
	case "HTTP":
	case "GRPC":
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
		conn.Send("SET", jobKey+"-scan", time.Now().Unix())
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
	_, err = conn.Do("DEL", jobKey+"-scan")
	return
}
func (t *q) Push(job *guaproto.Job) (err error) {

	ss := UrlRe.FindStringSubmatch(job.RequestUrl)
	if len(ss) == 0 {
		return errors.New("type error")
	}
	cmdType := ss[1]
	switch cmdType {
	case "HTTP":
	case "GRPC":
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

func (t *q) RemoveGroup(groupName string) (err error) {
	conn := t.urpool.Get()
	defer conn.Close()

	groupKey := fmt.Sprintf("USER_%s", groupName)
	_, err = conn.Do("DEL", groupKey)
	return
}
func (t *q) ExistsGroup(groupName string) (exists int, err error) {
	conn := t.urpool.Get()
	defer conn.Close()

	groupKey := fmt.Sprintf("USER_%s", groupName)
	exists, err = redis.Int(conn.Do("EXISTS", groupKey))
	return
}
