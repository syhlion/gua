package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/urfave/cli"
)

// Build metadata, stamped by the Makefile / Dockerfile via -ldflags -X.
var (
	version     string
	compileDate string
	name        = "gua"
)

var (
	cmdStart = cli.Command{
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
	logger *slog.Logger
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
	if err := gua.Run(os.Args); err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}
