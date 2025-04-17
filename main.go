package main

import (
	"os"

	"bitbucket.org/neiku/hlog"
	"bitbucket.org/neiku/winornot"
	"github.com/1set/starcli/cli"
	"github.com/1set/starcli/config"
	"github.com/1set/starcli/module/sys"
	"github.com/1set/starcli/web"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var (
	log *zap.SugaredLogger
)

func init() {
	// fix for Windows terminal output
	winornot.EnableANSIControl()
}

func main() {
	// parse args
	args := cli.ParseArgs()

	// set log level
	initLogger(args.LogLevel)

	// load config
	if err := config.InitConfig(args.ConfigFile); err != nil {
		log.Fatalw("fail to load config", zap.Error(err))
	}
	log.Debugw("config loaded", "config_file", viper.ConfigFileUsed(), "host_name", config.GetHostname())

	// main
	os.Exit(cli.Process(args))
}

func initLogger(level string) {
	// build root logger
	lg := hlog.NewSimpleLogger()
	if err := lg.SetLevelString(level); err != nil {
		lg.Error(err)
	}
	log = lg.SugaredLogger.With(zap.Int("pid", os.Getpid()))

	// set log for sub-packages
	cli.SetLog(log)
	web.SetLog(log)
	sys.SetLog(log)
}
