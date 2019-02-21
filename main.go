package main

import (
	"os"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

var env *string
var (
	version             string
	compileDate         string
	name                string
	listenChannelPrefix string
	cmdStart            = cli.Command{
		Name:    "start",
		Usage:   "start gua server",
		Aliases: []string{"st"},
		Action:  start,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "env-file,e",
				Usage: "import env file",
			},
			cli.BoolFlag{
				Name:  "debug,d",
				Usage: "open debug mode",
			},
		},
	}
	logger       *logrus.Logger
	guaMsgFormat = "\ngua start at \"{{.GetStartTime}}\"\tserver ip:\"{{.ExternalIp}}\"\tversion:\"{{.Version}}\"\tcomplie at \"{{.CompileDate}}\"\n" +
		"http_listen:\"{{.HttpListen}}\"\n" +
		"grpc_addr:\"{{.GrpcAddr}}\"\n" +
		"hostname:\"{{.Hostname}}\"\n" +
		"mac:\"{{.Mac}}\"\n" +
		"worker_num:\"{{.WorkerNum}}\"\n" +
		"machine_code:\"{{.MachineCode}}\"\n" +
		"redis_for_api_addr:\"{{.RedisForApiAddr}}\"\t" + "redis_for_api_dbno:\"{{.RedisForApiDBNo}}\"\n" +
		"redis_for_api_max_idle:\"{{.RedisForApiMaxIdle}}\"\n" +
		"redis_for_api_max_conn:\"{{.RedisForApiMaxConn}}\"\n" +
		"redis_for_ready_addr:\"{{.RedisForReadyAddr}}\"\t" + "redis_for_api_dbno:\"{{.RedisForReadyDBNo}}\"\n" +
		"redis_for_ready_max_idle:\"{{.RedisForReadyMaxIdle}}\"\n" +
		"redis_for_ready_max_conn:\"{{.RedisForReadyMaxConn}}\"\n" +
		"redis_for_delay_addr:\"{{.RedisForDelayAddr}}\"\t" + "redis_for_api_dbno:\"{{.RedisForDelayDBNo}}\"\n" +
		"redis_for_delay_max_idle:\"{{.RedisForDelayMaxIdle}}\"\n" +
		"redis_for_delay_max_conn:\"{{.RedisForDelayMaxConn}}\"\n\n"
)

func main() {
	cli.AppHelpTemplate += "\nWEBSITE:\n\t\thttps://github.com/syhlion/gua\n\n"
	gua := cli.NewApp()
	gua.Name = name
	gua.Author = "Scott (syhlion)"
	gua.Usage = ""
	gua.UsageText = "gua [start] [-e envfile] [-d]"
	gua.Version = version
	gua.Compiled = time.Now()
	gua.Commands = []cli.Command{
		cmdStart,
	}
	gua.Run(os.Args)
}
