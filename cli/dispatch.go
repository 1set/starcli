package cli

import (
	"github.com/1set/gut/ystring"
	"github.com/1set/starcli/util"
)

// Process routes the parsed arguments to the desired run mode and returns the
// process exit code (0 on success, 1 when the chosen action reports an error).
func Process(args *Args) int {
	// for basic checks
	numArg := args.NumberOfArgs
	useDirectCode := ystring.IsNotBlank(args.CodeContent)

	// determine action
	var action func(*Args) error
	switch {
	case args.ShowVersion:
		action = showVersion
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
	err := action(args)
	if err != nil {
		util.PrintError(err)
		return 1
	}
	return 0
}
