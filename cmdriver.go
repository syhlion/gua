package main

import (
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gorilla/handlers"
	"github.com/joho/godotenv"
	"github.com/syhlion/gua/delayquene"
	"github.com/syhlion/gua/httpv1"
	guaproto "github.com/syhlion/gua/proto"
	"github.com/urfave/cli"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// startRiver runs gua against the Postgres/River backend.
// It needs only GRPC_LISTEN, HTTP_LISTEN and PG_DSN — no Redis envs.
func startRiver(c *cli.Context) {
	if c.String("env-file") != "" {
		if err := godotenv.Load(c.String("env-file")); err != nil {
			// logger not built yet (it may read LOG_* from the env file)
			os.Stderr.WriteString("load env-file failed: " + err.Error() + "\n")
			os.Exit(1)
		}
	}
	logger = setupLogger()

	grpcListen := os.Getenv("GRPC_LISTEN")
	httpListen := os.Getenv("HTTP_LISTEN")
	dsn := os.Getenv("PG_DSN")
	if grpcListen == "" || httpListen == "" || dsn == "" {
		logFatal(logger, "river backend requires GRPC_LISTEN, HTTP_LISTEN and PG_DSN")
	}
	host, _ := GetHostname()
	ip, _ := GetExternalIP()
	mac, _ := GetMacAddr()

	historyTTL := 5 * 24 * 3600
	if v := os.Getenv("GUA_HISTORY_TTL"); v != "" {
		if n, perr := strconv.Atoi(v); perr == nil {
			historyTTL = n
		}
	}
	quene, err := delayquene.NewRiver(&delayquene.RiverConfig{
		DSN:         dsn,
		MachineHost: host,
		MachineIp:   ip,
		MachineMac:  mac,
		HistoryTTL:  historyTTL,
		Logger:      logger,
	})
	if err != nil {
		logFatal(logger, "startup error", "error", err)
	}
	defer quene.Close()

	httpv1.SetLogger(logger)

	apiListener, err := net.Listen("tcp", grpcListen)
	if err != nil {
		logFatal(logger, "startup error", "error", err)
	}
	grpcServer := grpc.NewServer()
	guaproto.RegisterGuaAdminServer(grpcServer, &GuaAdmin{quene: quene})
	reflection.Register(grpcServer)

	httpListener, err := net.Listen("tcp", httpListen)
	if err != nil {
		logFatal(logger, "startup error", "error", err)
	}
	server := http.Server{
		ReadTimeout: 3 * time.Second,
		Handler:     handlers.CORS()(buildRouter(quene)),
	}

	httpErr := make(chan error)
	grpcErr := make(chan error)
	go func() { httpErr <- server.Serve(httpListener) }()
	go func() { grpcErr <- grpcServer.Serve(apiListener) }()

	logger.Info("gua started", "backend", "river/postgres", "grpc", grpcListen, "http", httpListen)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sig:
		logger.Info("receive signal")
	case err := <-grpcErr:
		logger.Error("server error", "error", err)
	case err := <-httpErr:
		logger.Error("server error", "error", err)
	}
}
