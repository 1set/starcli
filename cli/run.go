package cli

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/1set/starbox"
	"github.com/1set/starcli/config"
	"github.com/1set/starcli/util"
	"github.com/1set/starcli/web"
	"github.com/1set/starlet"
	flag "github.com/spf13/pflag"
	"golang.org/x/term"
)

// runWebServer starts a web server that creates a Starbox with given code for each request.
func runWebServer(args *Args) error {
	var (
		runner        = starbox.NewRunConfig()
		webPort       = args.WebPort
		numArg        = args.NumberOfArgs
		useDirectCode = strings.TrimSpace(args.CodeContent) != ""
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

// runCheck resolves and type-checks the script WITHOUT executing it (CLI-02/C-5,
// reusing starbox.Check), printing each problem as "file:line:col: message". It
// builds the same box a real run would, so the check sees the configured modules
// and the capability policy. Exit is non-zero when any problem is found.
func runCheck(args *Args) error {
	// resolve the script source: -c code, or the first file argument
	var name, script string
	switch {
	case strings.TrimSpace(args.CodeContent) != "":
		name, script = "direct.star", args.CodeContent
	case args.NumberOfArgs >= 1:
		name = filepath.Base(args.Arguments[0])
		bs, err := ioutil.ReadFile(args.Arguments[0])
		if err != nil {
			return err
		}
		script = string(bs)
	default:
		return errors.New("check: nothing to check (pass -c CODE or a file)")
	}

	opt := args.BasicBoxOpts()
	opt.scenario = scenarioFile
	opt.name = name
	box, err := BuildBox(opt)
	if err != nil {
		return err
	}

	diags, err := box.Check(script)
	if err != nil {
		return err
	}
	if len(diags) == 0 {
		return nil
	}
	for _, d := range diags {
		d.File = name // Check labels diagnostics "box.star"; show the real name
		fmt.Fprintln(os.Stderr, d.String())
	}
	return fmt.Errorf("check: %d problem(s) found", len(diags))
}

func showVersion(args *Args) error {
	config.DisplayBuildInfo()
	return nil
}

func showHelp(args *Args) error {
	flag.Usage()
	return nil
}

// genInspectCond creates a function for the Starbox runner to inspect the
// result. In interactive mode it reports any error and keeps the box open;
// otherwise it never holds the box open.
func genInspectCond(inspect bool) starbox.InspectCondFunc {
	if inspect {
		return func(m starlet.StringAnyMap, err error) bool {
			if err != nil {
				util.PrintError(err)
			}
			return true
		}
	}
	return func(starlet.StringAnyMap, error) bool {
		return false
	}
}
