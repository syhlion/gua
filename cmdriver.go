package main

import (
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/handlers"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"github.com/syhlion/gua/delayquene"
	"github.com/syhlion/gua/httpv1"
	guaproto "github.com/syhlion/gua/proto"
	"github.com/urfave/cli"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// startRiver runs gua against the Postgres/River backend (BACKEND=river).
// It needs only GRPC_LISTEN, HTTP_LISTEN and PG_DSN — no Redis envs.
func startRiver(c *cli.Context) {
	logger = logrus.New()
	if c.String("env-file") != "" {
		if err := godotenv.Load(c.String("env-file")); err != nil {
			logger.Fatal(err)
		}
	}
	logger.SetFormatter(&logrus.JSONFormatter{})

	grpcListen := os.Getenv("GRPC_LISTEN")
	httpListen := os.Getenv("HTTP_LISTEN")
	dsn := os.Getenv("PG_DSN")
	if grpcListen == "" || httpListen == "" || dsn == "" {
		logger.Fatal("river backend requires GRPC_LISTEN, HTTP_LISTEN and PG_DSN")
	}
	host, _ := GetHostname()
	ip, _ := GetExternalIP()
	mac, _ := GetMacAddr()

	quene, err := delayquene.NewRiver(&delayquene.RiverConfig{
		DSN:         dsn,
		MachineHost: host,
		MachineIp:   ip,
		MachineMac:  mac,
		Logger:      logger,
	})
	if err != nil {
		logger.Fatal(err)
	}
	defer quene.Close()

	httpv1.SetLogger(logger)

	apiListener, err := net.Listen("tcp", grpcListen)
	if err != nil {
		logger.Fatal(err)
	}
	grpcServer := grpc.NewServer()
	guaproto.RegisterGuaAdminServer(grpcServer, &GuaAdmin{quene: quene})
	reflection.Register(grpcServer)

	httpListener, err := net.Listen("tcp", httpListen)
	if err != nil {
		logger.Fatal(err)
	}
	server := http.Server{
		ReadTimeout: 3 * time.Second,
		Handler:     handlers.CORS()(buildRouter(quene, nil)),
	}

	httpErr := make(chan error)
	grpcErr := make(chan error)
	go func() { httpErr <- server.Serve(httpListener) }()
	go func() { grpcErr <- grpcServer.Serve(apiListener) }()

	logger.Infof("gua started (river/postgres backend)  grpc=%s  http=%s", grpcListen, httpListen)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sig:
		logger.Info("receive signal")
	case err := <-grpcErr:
		logger.Error(err)
	case err := <-httpErr:
		logger.Error(err)
	}
}
