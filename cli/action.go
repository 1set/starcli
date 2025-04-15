package cli

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/1set/gut/ystring"
	"github.com/1set/starbox"
	"github.com/PureMature/starcli/config"
	"github.com/PureMature/starcli/web"
	flag "github.com/spf13/pflag"
	"golang.org/x/term"
)

// runWebServer starts a web server that creates a Starbox with given code for each request.
func runWebServer(args *Args) error {
	var (
		runner        = starbox.NewRunConfig()
		webPort       = args.WebPort
		numArg        = args.NumberOfArgs
		useDirectCode = ystring.IsNotBlank(args.CodeContent)
	)

	// prepare runner
	if useDirectCode {
		// if code content is provided in flag, just use it
		runner = runner.FileName("web.star").Script(args.CodeContent)
	} else if numArg >= 1 {
		// or use the first argument as file name
		runner = runner.FileName(args.Arguments[0])
	} else {
		// no repl mode for web server, just quit if no code if provided
		return errors.New("no code to run as web server")
	}

	// attempt to build box
	opt := args.BasicBoxOpts()
	opt.scenario = scenarioWeb
	opt.name = "web"
	if _, err := BuildBox(opt); err != nil {
		return err
	}

	// start web server
	build := func() *starbox.RunnerConfig {
		b, _ := BuildBox(opt)
		return runner.Starbox(b)
	}
	return web.Start(webPort, build)
}

func runDirectCode(args *Args) error {
	// build box and runner
	opt := args.BasicBoxOpts()
	opt.scenario = scenarioDirect
	opt.name = "direct"
	opt.cmdArgs = append([]string{`-c`}, args.Arguments...)
	box, err := BuildBox(opt)
	if err != nil {
		return err
	}
	run := box.CreateRunConfig().
		FileName("direct.star").
		Script(args.CodeContent).
		InspectCond(genInspectCond(args.InteractiveMode))

	// run script
	_, err = run.Execute()
	return err
}

func runREPL(args *Args) error {
	// for build info
	stdinIsTerminal := term.IsTerminal(int(os.Stdin.Fd()))
	if stdinIsTerminal {
		config.DisplayBuildInfo()
	}

	// build box and run
	opt := args.BasicBoxOpts()
	opt.scenario = scenarioREPL
	opt.name = "repl"
	opt.cmdArgs = []string{``}
	box, err := BuildBox(opt)
	if err != nil {
		return err
	}
	err = box.REPL()

	// add extra line for better output
	if stdinIsTerminal {
		fmt.Println()
	}
	return err
}

func runScriptFile(args *Args) error {
	// load file
	fileName := args.Arguments[0]
	bs, err := ioutil.ReadFile(fileName)
	if err != nil {
		return err
	}

	// build box and runner
	name := filepath.Base(fileName)
	opt := args.BasicBoxOpts()
	opt.scenario = scenarioFile
	opt.name = name
	box, err := BuildBox(opt)
	if err != nil {
		return err
	}
	run := box.CreateRunConfig().
		FileName(name).
		Script(string(bs)).
		InspectCond(genInspectCond(args.InteractiveMode))

	// run script
	_, err = run.Execute()
	return err
}

func showVersion(args *Args) error {
	config.DisplayBuildInfo()
	return nil
}

func showHelp(args *Args) error {
	flag.Usage()
	return nil
}
