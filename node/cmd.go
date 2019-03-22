package main

import (
	"context"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"text/template"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/pquerna/otp/totp"
	"github.com/sirupsen/logrus"
	guaproto "github.com/syhlion/gua/proto"
	"github.com/urfave/cli"
	bolt "go.etcd.io/bbolt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func cmdInit(c *cli.Context) (conf *Config) {
	var err error
	logger = logrus.New()
	if c.String("env-file") != "" {
		envfile := c.String("env-file")
		//flag.Parse()
		err := godotenv.Load(envfile)
		if err != nil {
			logger.Fatal(err)
		}
	}
	conf = &Config{}
	conf.GuaAddr = os.Getenv("GUA_ADDR")
	if conf.GuaAddr == "" {
		logger.Fatal("empty env GUA_ADDR")
	}
	conf.WorkerNum, err = strconv.Atoi(os.Getenv("WORKER_NUM"))
	if err != nil {
		conf.WorkerNum = 100
	}
	conf.GroupName = os.Getenv("GROUP_NAME")
	if conf.GroupName == "" {
		logger.Fatal("empty env GROUP_NAME")
	}
	conf.MachineCode = os.Getenv("MACHINE_CODE")
	if conf.MachineCode == "" {
		logger.Fatal("empty env MACHINE_CODE")
	}
	//選填
	conf.BoradcastAddr = os.Getenv("BORADCAST_ADDR")

	conf.GrpcListen = os.Getenv("GRPC_LISTEN")
	if conf.GrpcListen == "" {
		logger.Fatal("empty env GRPC_LISTEN")
	}
	conf.HttpListen = os.Getenv("HTTP_LISTEN")
	if conf.HttpListen == "" {
		logger.Fatal("empty env HTTP_LISTEN")
	}
	conf.Hostname, err = GetHostname()
	if err != nil {
		logger.Fatal(err)
	}
	conf.ExternalIp, err = GetExternalIP()
	if err != nil {
		logger.Fatal(err)
	}
	conf.Mac, err = GetMacAddr()
	if err != nil {
		logger.Fatal(err)
	}
	conf.StartTime = time.Now()
	data, err := ioutil.ReadFile("./backup")
	if err != nil {
		kkey, err := totp.Generate(totp.GenerateOpts{
			Issuer:      conf.Mac,
			AccountName: conf.ExternalIp,
		})
		if err != nil {
			logger.Fatal(err)
		}
		conf.OtpToken = kkey.Secret()
	} else {
		b := Backup{}
		err := json.Unmarshal(data, &b)
		if err != nil {
			kkey, err := totp.Generate(totp.GenerateOpts{
				Issuer:      conf.Mac,
				AccountName: conf.ExternalIp,
			})
			if err != nil {
				logger.Fatal(err)
			}
			conf.OtpToken = kkey.Secret()
			return
		}
		conf.OtpToken = b.OtpToken
		conf.NodeId = b.NodeId
	}
	return
}

type Backup struct {
	OtpToken string
	NodeId   string
}

func start(c *cli.Context) {

	conf := cmdInit(c)
	db, err := bolt.Open("gua-node.db", 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	//server
	apiListener, err := net.Listen("tcp", conf.GrpcListen)
	if err != nil {
		log.Println(err)
		return
	}

	//reply gua
	conn, err := grpc.Dial(conf.GuaAddr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	guaClient := guaproto.NewGuaClient(conn)
	if conf.NodeId == "" {
		ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
		nreq := &guaproto.NodeRegisterRequest{
			Ip:            conf.ExternalIp,
			Hostname:      conf.Hostname,
			Mac:           conf.Mac,
			OtpToken:      conf.OtpToken,
			BoradcastAddr: conf.BoradcastAddr,
			Grpclisten:    conf.GrpcListen,
			MachineCode:   conf.MachineCode,
			GroupName:     conf.GroupName,
		}
		nrep, err := guaClient.NodeRegister(ctx, nreq)
		if err != nil {
			logger.Fatal("register fail", err)
			return
		}
		conf.NodeId = nrep.NodeId
		back := Backup{
			NodeId:   conf.NodeId,
			OtpToken: conf.OtpToken,
		}
		b, _ := json.Marshal(back)
		ioutil.WriteFile("./backup", b, 0644)
	}

	//heartbeat
	heartbeatErr := make(chan error)
	go func() {
		var err error
		defer func() {
			//判斷 error 是否因為 timeout 才需要刪除 backup
			if err != nil {
				if strings.Contains(err.Error(), "NO_REMOTE_NODE") {
					err = os.Remove("./backup")
					if err != nil {
						logger.Error(err)
					}
				}
			}
		}()
		t := time.NewTicker(5 * time.Second)
		for {
			select {
			case <-t.C:
				ctx, _ := context.WithTimeout(context.Background(), 1*time.Second)
				code, _ := totp.GenerateCode(conf.OtpToken, time.Now())
				req := &guaproto.Ping{
					NodeId:    conf.NodeId,
					OtpCode:   code,
					GroupName: conf.GroupName,
				}
				_, err = guaClient.Heartbeat(ctx, req)
				if err != nil {
					heartbeatErr <- err
					return
				}

			}
		}

	}()

	/*
		work := requestwork.New(5)
		client := greq.New(work, 60*time.Second, true)
	*/

	// 註冊 grpc
	sr := &Node{
		id:        conf.NodeId,
		cmdChan:   make(chan Command, 1000),
		workerNum: conf.WorkerNum,
		//httpClient:  client,
		guaClient: guaClient,
		otpToken:  conf.OtpToken,
		localdb:   db,
		config:    conf,
	}
	go sr.run()

	grpc := grpc.NewServer()
	guaproto.RegisterGuaNodeServer(grpc, sr)

	reflection.Register(grpc)
	httpErr := make(chan error)
	grpcErr := make(chan error)
	httpApiListener, err := net.Listen("tcp", conf.HttpListen)
	r := mux.NewRouter()
	r.HandleFunc("/node_id", GetNodeId(sr)).Methods("GET")
	server := http.Server{
		ReadTimeout: 3 * time.Second,
		Handler:     r,
	}
	go func() {
		httpErr <- server.Serve(httpApiListener)
	}()

	go func() {
		grpcErr <- grpc.Serve(apiListener)
	}()
	shutdow_observer := make(chan os.Signal, 1)
	t := template.Must(template.New("node start msg").Parse(nodeMsgFormat))
	t.Execute(os.Stdout, conf)
	signal.Notify(shutdow_observer, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
	select {
	case <-shutdow_observer:
		logger.Info("stop")
	case err := <-grpcErr:
		logger.Error(err)
	case err := <-httpErr:
		logger.Error(err)
	case err := <-heartbeatErr:
		logger.Error(err)
	}
	return
}
