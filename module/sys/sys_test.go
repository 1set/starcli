package sys

// Characterization tests for the sys module surface.
//
// Sections:
//   - module dict (platform/arch/version/argv/host/input)
//   - input() reads and trims a line from stdin
//   - shared stream, EOF, CRLF, buffering, iteration and read errors

import (
	"errors"
	"io"
	"os"
	"runtime"
	"strings"
	"testing"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// withStdin redirects os.Stdin to a pipe carrying content for the duration of f.
func withStdin(t *testing.T, content string, f func()) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stdin
	os.Stdin = r
	finished := make(chan struct{})
	defer func() { _ = r.Close(); <-finished; os.Stdin = orig }()
	go func() { defer close(finished); _, _ = io.WriteString(w, content); _ = w.Close() }()
	f()
}

func TestStdinRead(t *testing.T) {
	withStdin(t, "a\nb\nc\n", func() {
		v, err := stdinRead(nil, starlark.NewBuiltin("sys.read", stdinRead), nil, nil)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if v != starlark.String("a\nb\nc\n") {
			t.Errorf("read()=%q want %q", v, "a\nb\nc\n")
		}
	})
}

func TestStdinLines(t *testing.T) {
	withStdin(t, "foo\nbar\nbaz", func() { // no trailing newline on the last line
		it := stdinLines{}.Iterate()
		defer it.Done()
		var got []string
		var v starlark.Value
		for it.Next(&v) {
			got = append(got, string(v.(starlark.String)))
		}
		want := []string{"foo", "bar", "baz"}
		if len(got) != len(want) {
			t.Fatalf("lines()=%v want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("line %d=%q want %q", i, got[i], want[i])
			}
		}
	})
}

func TestStdinIsatty(t *testing.T) {
	withStdin(t, "data", func() {
		v, err := stdinIsatty(nil, starlark.NewBuiltin("sys.isatty", stdinIsatty), nil, nil)
		if err != nil {
			t.Fatalf("isatty: %v", err)
		}
		if v != starlark.False { // a pipe is not a terminal
			t.Errorf("isatty()=%v want False on a pipe", v)
		}
	})
}

func TestNewModule_Surface(t *testing.T) {
	// WrapModuleData returns {ModuleName: *starlarkstruct.Module{Members: ...}};
	// the members are what scripts reach via load("sys", "<member>").
	loader := NewModule([]string{"a", "b"})
	dict, err := loader()
	if err != nil {
		t.Fatalf("loader(): %v", err)
	}
	mod, ok := dict[ModuleName].(*starlarkstruct.Module)
	if !ok {
		t.Fatalf("dict[%q] is %T want *starlarkstruct.Module", ModuleName, dict[ModuleName])
	}
	m := mod.Members

	if got := m["platform"]; got != starlark.String(runtime.GOOS) {
		t.Errorf("platform=%v want %q", got, runtime.GOOS)
	}
	if got := m["arch"]; got != starlark.String(runtime.GOARCH) {
		t.Errorf("arch=%v want %q", got, runtime.GOARCH)
	}
	if got := m["version"]; got != starlark.MakeUint(starlark.CompilerVersion) {
		t.Errorf("version=%v want %v", got, starlark.MakeUint(starlark.CompilerVersion))
	}
	if _, ok := m["host"].(starlark.String); !ok {
		t.Errorf("host=%v want a starlark.String", m["host"])
	}

	argv, ok := m["argv"].(*starlark.List)
	if !ok {
		t.Fatalf("argv is %T want *starlark.List", m["argv"])
	}
	if argv.Len() != 2 || argv.Index(0) != starlark.String("a") || argv.Index(1) != starlark.String("b") {
		t.Errorf("argv=%v want [\"a\", \"b\"]", argv)
	}

	fn, ok := m["input"].(*starlark.Builtin)
	if !ok {
		t.Fatalf("input is %T want *starlark.Builtin", m["input"])
	}
	if fn.Name() != "sys.input" {
		t.Errorf("input builtin name=%q want %q", fn.Name(), "sys.input")
	}

	for _, name := range []string{"read", "lines", "isatty"} {
		if _, ok := m[name].(*starlark.Builtin); !ok {
			t.Errorf("member %q is %T, want *starlark.Builtin", name, m[name])
		}
	}
}

func TestInput_ReadsAndTrimsLine(t *testing.T) {
	r, w, _ := os.Pipe()
	orig := os.Stdin
	os.Stdin = r
	defer func() { _ = r.Close(); os.Stdin = orig }()
	go func() { _, _ = w.WriteString("hello world\r\n"); _ = w.Close() }()

	b := starlark.NewBuiltin("sys.input", rawStdInput)
	got, err := rawStdInput(nil, b, nil, nil)
	if err != nil {
		t.Fatalf("rawStdInput: %v", err)
	}
	if got != starlark.String("hello world") {
		t.Errorf("input=%v want %q (CR/LF trimmed)", got, "hello world")
	}
}

// Shared stdin contract: input, read, and every iterator consume one stream.
func TestStdinSharedStream(t *testing.T) {
	for _, tc := range []struct {
		name, text, code, want string
	}{
		{"EOF tail", "tail", `result = input()`, `"tail"`},
		{"continuous", "a\nb\nc", `result = [input(), input(), input()]`, `["a", "b", "c"]`},
		{"input then read", "a\nb\nc", `result = [input(), read()]`, `["a", "b\nc"]`},
		{"input then lines", "a\nb\nc", `result = [input(), list(lines())]`, `["a", ["b", "c"]]`},
		{"iterator then input", "a\nb\nc", "def take_one():\n    for line in lines():\n        return line\nresult = [take_one(), input(), read()]", `["a", "b", "c"]`},
		{"iterators share stream", "a\nb\nc", "stream = lines()\ndef take_one():\n    for line in stream:\n        return line\nresult = [take_one(), list(stream), list(stream)]", `["a", ["b", "c"], []]`},
		{"CRLF and blank", "a\r\n\r\nz", `result = [input(), input(), input()]`, `["a", "", "z"]`},
		{"trailing LF", "a\n", `result = [input(), list(lines()), read()]`, `["a", [], ""]`},
		{"empty", "", `result = [list(lines()), read()]`, `[[], ""]`},
		{"long line", strings.Repeat("x", 70000) + "\nlast", `result = [len(input()), input()]`, `[70000, "last"]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withStdin(t, tc.text, func() {
				predeclared := starlark.StringDict{
					"input": starlark.NewBuiltin("input", rawStdInput),
					"read":  starlark.NewBuiltin("read", stdinRead),
					"lines": starlark.NewBuiltin("lines", stdinLinesFn),
				}
				globals, err := starlark.ExecFile(&starlark.Thread{}, "stdin.star", tc.code, predeclared)
				if err != nil {
					t.Fatal(err)
				}
				if got := globals["result"].String(); got != tc.want {
					t.Fatalf("result = %s, want %s", got, tc.want)
				}
			})
		})
	}
}

func TestStdinErrors(t *testing.T) {
	withStdin(t, "", func() {
		if _, err := rawStdInput(nil, starlark.NewBuiltin("input", rawStdInput), nil, nil); !errors.Is(err, io.EOF) {
			t.Fatalf("empty input: %v", err)
		}
	})
	withStdin(t, "", func() {
		if err := os.Stdin.Close(); err != nil {
			t.Fatal(err)
		}
		for name, fn := range map[string]func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error){"input": rawStdInput, "read": stdinRead} {
			if _, err := fn(nil, starlark.NewBuiltin(name, fn), nil, nil); err == nil {
				t.Errorf("%s accepted closed stdin", name)
			}
		}
		it := stdinLines{}.Iterate()
		defer it.Done()
		var v starlark.Value
		if it.Next(&v) || it.Next(&v) {
			t.Error("closed stdin produced a line")
		}
	})
	for name, fn := range map[string]func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error){"input": rawStdInput, "read": stdinRead, "lines": stdinLinesFn, "isatty": stdinIsatty} {
		if _, err := fn(nil, starlark.NewBuiltin(name, fn), starlark.Tuple{starlark.None}, nil); err == nil {
			t.Errorf("%s accepted invalid args", name)
		}
	}
}
