package main

import (
	"os"
	// Embed the IANA time-zone database into the binary so named zones
	// (time.parse_time location=, in_location, is_valid_timezone) resolve even
	// when the host has no /usr/share/zoneinfo — i.e. minimal/scratch/distroless
	// containers and slimmed binaries. Costs ~450 KB; keeps the CLI portable
	// ("runs anywhere") instead of depending on host tz data.
	_ "time/tzdata"

	"github.com/1set/starcli/cli"
	"github.com/1set/starcli/config"
	"github.com/1set/starcli/module/sys"
	"github.com/1set/starcli/web"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	log *zap.SugaredLogger
)

func init() {
	// fix for Windows terminal output
	enableANSIControl()
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
	// build the root console logger (stderr), keeping diagnostics off stdout
	lvl := zapcore.InfoLevel
	if err := lvl.Set(level); err != nil {
		lvl = zapcore.InfoLevel
	}
	encCfg := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalColorLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
	core := zapcore.NewCore(zapcore.NewConsoleEncoder(encCfg), zapcore.Lock(os.Stderr), lvl)
	log = zap.New(core).Sugar().With(zap.Int("pid", os.Getpid()))

	// set log for sub-packages
	cli.SetLog(log)
	web.SetLog(log)
	sys.SetLog(log)
}
