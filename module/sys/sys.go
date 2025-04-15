// Package sys provides a Starlark module that exposes runtime information and arguments, and functions to interact with the system.
package sys

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/1set/starlet"
	"github.com/1set/starlet/dataconv"
	"github.com/PureMature/starcli/config"
	"go.starlark.net/starlark"
)

const (
	// ModuleName defines the module name.
	ModuleName = "sys"
)

// NewModule creates a new module loader for the sys module.
func NewModule(args []string) starlet.ModuleLoader {
	// get sa
	sa := make([]starlark.Value, 0, len(args))
	for _, arg := range args {
		sa = append(sa, starlark.String(arg))
	}
	// build module
	sd := starlark.StringDict{
		"platform": starlark.String(runtime.GOOS),
		"arch":     starlark.String(runtime.GOARCH),
		"version":  starlark.MakeUint(starlark.CompilerVersion),
		"argv":     starlark.NewList(sa),
		"input":    starlark.NewBuiltin(ModuleName+".input", rawStdInput),
		"host":     starlark.String(config.GetHostname()),
	}
	return dataconv.WrapModuleData(ModuleName, sd)
}

func rawStdInput(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// unpack arguments
	var prompt string
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "prompt?", &prompt); err != nil {
		return starlark.None, err
	}
	// display prompt
	if prompt != "" {
		fmt.Print(prompt)
	}
	// read input from stdin
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	// trim newline characters
	input = strings.TrimRight(input, "\r\n")
	return starlark.String(input), nil
}
