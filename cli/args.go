package cli

import (
	"os"

	flag "github.com/spf13/pflag"
)

// Args is the command line arguments for starcli.
type Args struct {
	AllowRecursion      bool
	AllowGlobalReassign bool
	ModulesToLoad       []string
	IncludePath         string
	FileName            string
	CodeContent         string
	WebPort             uint16
	NumberOfArgs        int
	Arguments           []string
	LogLevel            string
	ShowVersion         bool
	InteractiveMode     bool
	OutputPrinter       string
	ConfigFile          string
	MaxSteps            uint64
	MaxOutput           uint
	Caps                string
	AllowNet            bool
	AllowFS             bool
	AllowCmd            bool
	Check               bool
	LogFile             string
	LogFormat           string
	Record              string
}

// ParseArgs parses command line arguments and returns the Args object.
func ParseArgs() *Args {
	args := &Args{}

	// parse command line arguments
	flag.BoolVarP(&args.AllowRecursion, "recursion", "r", false, "allow recursion in Starlark code")
	flag.BoolVarP(&args.AllowGlobalReassign, "globalreassign", "g", true, "allow reassigning global variables in Starlark code")
	flag.StringSliceVarP(&args.ModulesToLoad, "module", "m", getDefaultModules(), "allowed modules to preload and load")
	flag.StringVarP(&args.IncludePath, "include", "I", ".", "include path for Starlark code to load modules from")
	flag.StringVarP(&args.CodeContent, "code", "c", "", "Starlark code to execute")
	flag.Uint16VarP(&args.WebPort, "web", "w", 0, "run web server on specified port, it provides request and response structs for Starlark code to use")
	flag.StringVarP(&args.LogLevel, "log", "l", "info", "log level: debug, info, warn, error, dpanic, panic, fatal")
	flag.BoolVarP(&args.ShowVersion, "version", "V", false, "print version & build information")
	flag.BoolVarP(&args.InteractiveMode, "interactive", "i", false, "enter interactive mode after executing")
	flag.StringVarP(&args.OutputPrinter, "output", "o", "auto", "output printer: none,stdout,stderr,basic,lineno,since,auto")
	flag.StringVarP(&args.ConfigFile, "config", "C", "", "config file to load")
	flag.Uint64Var(&args.MaxSteps, "max-steps", 0, "max Starlark execution steps per run, guards runaway loops (0=unlimited)")
	flag.UintVar(&args.MaxOutput, "max-output", 0, "max top-level output entries per run (0=unlimited)")
	flag.StringVar(&args.Caps, "caps", "", "capability tier: open (default, everything) | full | network | safe; or env STAR_CAPS")
	flag.BoolVar(&args.AllowNet, "allow-net", false, "widen a restrictive tier with network modules (http, net, email, llm)")
	flag.BoolVar(&args.AllowFS, "allow-fs", false, "widen a restrictive tier with filesystem modules (file, path)")
	flag.BoolVar(&args.AllowCmd, "allow-cmd", false, "widen a restrictive tier with the cmd module (host command execution)")
	flag.BoolVar(&args.Check, "check", false, "syntax/resolve check the script (-c or file) without running it")
	flag.StringVar(&args.LogFile, "log-file", "", "append the script's log module output to this file")
	flag.StringVar(&args.LogFormat, "log-format", "console", "log file format: console (human) or json (machine)")
	flag.StringVar(&args.Record, "record", "", "record the complete session output (stdout+stderr) to this transcript file")
	flag.Parse()

	// Capability tier resolution: an explicit --caps wins; otherwise fall back
	// to the STAR_CAPS env var; otherwise empty, which BuildBox treats as the
	// default open tier.
	if args.Caps == "" {
		args.Caps = os.Getenv("STAR_CAPS")
	}

	// keep the rest of arguments
	args.NumberOfArgs = flag.NArg()
	args.Arguments = flag.Args()
	return args
}

// BasicBoxOpts returns the basic BoxOpts object based on the command line arguments.
func (a *Args) BasicBoxOpts() *BoxOpts {
	return &BoxOpts{
		cmdArgs:        a.Arguments,
		includePath:    a.IncludePath,
		moduleToLoad:   a.ModulesToLoad,
		printerName:    a.OutputPrinter,
		recursion:      a.AllowRecursion,
		globalReassign: a.AllowGlobalReassign,
		logFile:        a.LogFile,
		logFormat:      a.LogFormat,
		maxSteps:       a.MaxSteps,
		maxOutput:      a.MaxOutput,
		caps:           a.Caps,
		allowNet:       a.AllowNet,
		allowFS:        a.AllowFS,
		allowCmd:       a.AllowCmd,
	}
}
