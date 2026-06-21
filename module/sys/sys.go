// Package sys provides a Starlark module that exposes runtime information and arguments, and functions to interact with the system.
package sys

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/1set/starcli/config"
	"github.com/1set/starlet"
	"github.com/1set/starlet/dataconv"
	"go.starlark.net/starlark"
	"golang.org/x/term"
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
		"read":     starlark.NewBuiltin(ModuleName+".read", stdinRead),
		"lines":    starlark.NewBuiltin(ModuleName+".lines", stdinLinesFn),
		"isatty":   starlark.NewBuiltin(ModuleName+".isatty", stdinIsatty),
	}
	return dataconv.WrapModuleData(ModuleName, sd)
}

// stdinRead reads all of standard input (piped data) and returns it as a string.
func stdinRead(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(b.Name(), args, kwargs); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, err
	}
	return starlark.String(data), nil
}

// stdinIsatty reports whether standard input is a terminal (vs a pipe or file),
// so a script can branch on interactive vs piped input.
func stdinIsatty(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(b.Name(), args, kwargs); err != nil {
		return nil, err
	}
	return starlark.Bool(term.IsTerminal(int(os.Stdin.Fd()))), nil
}

// stdinLinesFn returns a lazy iterable over the lines of standard input, so a
// large stream is consumed line by line rather than buffered whole.
func stdinLinesFn(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(b.Name(), args, kwargs); err != nil {
		return nil, err
	}
	return stdinLines{}, nil
}

// stdinLines is a one-shot lazy iterable over os.Stdin lines (trailing CR/LF
// trimmed). Iterating it more than once yields nothing the second time, as the
// stream is already drained — the same single-stream contract as `for line in
// sys.stdin` in Python.
type stdinLines struct{}

var _ starlark.Iterable = stdinLines{}

func (stdinLines) String() string        { return "<stdin lines>" }
func (stdinLines) Type() string          { return "stdin_lines" }
func (stdinLines) Freeze()               {}
func (stdinLines) Truth() starlark.Bool  { return starlark.True }
func (stdinLines) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable type: stdin_lines") }
func (stdinLines) Iterate() starlark.Iterator {
	return &stdinLinesIter{r: bufio.NewReader(os.Stdin)}
}

type stdinLinesIter struct {
	r    *bufio.Reader
	done bool
}

func (it *stdinLinesIter) Next(p *starlark.Value) bool {
	if it.done {
		return false
	}
	line, err := it.r.ReadString('\n')
	if line == "" && err != nil {
		it.done = true
		return false
	}
	*p = starlark.String(strings.TrimRight(line, "\r\n"))
	if err != nil {
		it.done = true // final line had no trailing newline
	}
	return true
}

func (it *stdinLinesIter) Done() {}

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
