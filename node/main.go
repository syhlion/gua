package main

import (
	"os"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

const (
	BOLTBUCKET = "gua-node-bucket"
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
		Usage:   "start gua-node server",
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
	logger        *logrus.Logger
	nodeMsgFormat = "\ngua-node start at \"{{.GetStartTime}}\"\tserver ip:\"{{.ExternalIp}}\"\tversion:\"{{.Version}}\"\tcomplie at \"{{.CompileDate}}\"\n" +
		"http_listen:\"{{.HttpListen}}\"\n" +
		"grpc_addr:\"{{.GrpcAddr}}\"\n" +
		"hostname:\"{{.Hostname}}\"\n" +
		"mac:\"{{.Mac}}\"\n" +
		"otp_token:\"{{.OtpToken}}\"\n" +
		"worker_num:\"{{.WorkerNum}}\"\n" +
		"machine_code:\"{{.MachineCode}}\"\n\n"
)

func main() {
	cli.AppHelpTemplate += "\nWEBSITE:\n\t\thttps://github.com/syhlion/gua\n\n"
	node := cli.NewApp()
	node.Name = name
	node.Author = "Scott (syhlion)"
	node.Usage = ""
	node.UsageText = "gua-node [start] [-e envfile] [-d]"
	node.Version = version
	node.Compiled = time.Now()
	node.Commands = []cli.Command{
		cmdStart,
	}
	node.Run(os.Args)
}
