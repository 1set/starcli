package sys

// Characterization tests for the sys module surface.
//
// Sections:
//   - module dict (platform/arch/version/argv/host/input)
//   - input() reads and trims a line from stdin

import (
	"os"
	"runtime"
	"testing"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

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
}

func TestInput_ReadsAndTrimsLine(t *testing.T) {
	r, w, _ := os.Pipe()
	orig := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = orig }()
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
