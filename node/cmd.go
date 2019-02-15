package main

import (
	"context"
	"log"
	"net"
	"os"
	"strconv"
	"time"

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
	conf.MachineCode = os.Getenv("MACHINE_CODE")
	if conf.MachineCode == "" {
		logger.Fatal("empty env MACHINE_CODE")
	}
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
	return
}

func start(c *cli.Context) {

	conf := cmdInit(c)
	db, err := bolt.Open("gua-node.db", 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	kkey, err := totp.Generate(totp.GenerateOpts{
		Issuer:      conf.Mac,
		AccountName: conf.ExternalIp,
	})
	conf.OtpToken = kkey.Secret()
	//reply gua
	conn, err := grpc.Dial(conf.GuaAddr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	guaClient := guaproto.NewGuaClient(conn)
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	nreq := &guaproto.NodeRegisterRequest{
		Ip:          conf.ExternalIp,
		Hostname:    conf.Hostname,
		Mac:         conf.Mac,
		OtpToken:    kkey.Secret(),
		MachineCode: conf.MachineCode,
	}
	nrep, err := guaClient.NodeRegister(ctx, nreq)
	if err != nil {
		logger.Fatal("register fail", err)
		return
	}
	id := nrep.NodeId

	//server
	apiListener, err := net.Listen("tcp", conf.GrpcListen)
	if err != nil {
		log.Println(err)
		return
	}
	/*
		work := requestwork.New(5)
		client := greq.New(work, 60*time.Second, true)
	*/

	// 註冊 grpc
	sr := &Node{
		id:        id,
		cmdChan:   make(chan Command),
		workerNum: conf.WorkerNum,
		//httpClient:  client,
		guaClient: guaClient,
		otpToken:  conf.OtpToken,
	}

	grpc := grpc.NewServer()
	guaproto.RegisterGuaNodeServer(grpc, sr)

	reflection.Register(grpc)
	if err := grpc.Serve(apiListener); err != nil {
		log.Fatal(" grpc.Serve Error: ", err)
		return
	}
}
