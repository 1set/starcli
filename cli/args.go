package cli

import (
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
	flag.Parse()

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
	}
}
