package cli

import (
	"fmt"
	"os"

	"github.com/1set/gut/ystring"
	"github.com/1set/starbox"
	"github.com/1set/starcli/util"
)

// Run-mode exit codes. 0 is success and 1 is the catch-all failure (an
// unclassified error, or a Starlark evaluation error). The budget/structural
// kinds get distinct codes so a caller can branch on them.
const (
	exitOK             = 0
	exitError          = 1 // eval error or unclassified failure
	exitSyntax         = 2 // not valid Starlark
	exitCompile        = 3 // resolve error (undefined name, bad load, ...)
	exitModuleWithheld = 4 // load() of a module the policy/set withholds
	exitMaxSteps       = 5 // execution step budget exceeded
	exitOutputLimit    = 6 // output entry limit exceeded
)

// Process routes the parsed arguments to the desired run mode and returns the
// process exit code.
func Process(args *Args) int {
	// for basic checks
	numArg := args.NumberOfArgs
	useDirectCode := ystring.IsNotBlank(args.CodeContent)

	// determine action
	var action func(*Args) error
	switch {
	case args.ShowVersion:
		action = showVersion
	case args.Check:
		action = runCheck
	case args.WebPort > 0:
		action = runWebServer
	case useDirectCode:
		action = runDirectCode
	case numArg == 0:
		action = runREPL
	case numArg >= 1:
		action = runScriptFile
	default:
		action = showHelp
	}

	// execute action
	if err := action(args); err != nil {
		return reportError(err)
	}
	return exitOK
}

// reportError prints a failed run's error and maps it to an exit code by its
// classified kind (starbox.RunError). Budget and withheld failures get a short,
// readable banner instead of a Starlark backtrace they don't have.
func reportError(err error) int {
	switch starbox.ClassifyRunError(err).Kind {
	case starbox.RunErrorMaxSteps:
		fmt.Fprintln(os.Stderr, "Error: execution step budget exceeded (raise or drop --max-steps)")
		return exitMaxSteps
	case starbox.RunErrorOutputLimit:
		fmt.Fprintln(os.Stderr, "Error: output entry limit exceeded (raise or drop --max-output)")
		return exitOutputLimit
	case starbox.RunErrorModuleWithheld:
		util.PrintError(err)
		return exitModuleWithheld
	case starbox.RunErrorSyntax:
		util.PrintError(err)
		return exitSyntax
	case starbox.RunErrorCompile:
		util.PrintError(err)
		return exitCompile
	default: // RunErrorEval, RunErrorUnknown
		util.PrintError(err)
		return exitError
	}
}
