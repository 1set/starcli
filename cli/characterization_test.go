package cli

// Characterization tests: they pin starcli's CURRENT observable behavior at the
// package boundary so the structural refactor (CLI-R0/M1) can move internals
// without changing what a user sees. They are white-box (package cli) so they
// exercise the real run-mode dispatch, module wiring and printer logic and so
// count toward coverage.
//
// Sections:
//   - capture helper (swaps os.Stdout/os.Stderr around a call)
//   - printer selection & routing      (getPrinterFunc)
//   - default module set & resolution  (getDefaultModules / loadCLIModuleByName)
//   - run-mode dispatch & exit codes   (Process: direct / file / version / web / unknowns)
//
// Behaviors deliberately left for the M2 starbox-wiring PR to change (and which
// these tests therefore assert loosely, via Contains): the exact error preamble
// and the SetLogger HACK output.

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"go.starlark.net/starlark"
)

// captureStd swaps os.Stdout and os.Stderr for pipes, runs f, and returns
// whatever f wrote to each. starcli's printers and error reporting write to the
// os.Stdout/os.Stderr globals, so this is how their output is observed. Not
// parallel-safe: these tests must not call t.Parallel.
func captureStd(t *testing.T, f func()) (stdout, stderr string) {
	t.Helper()
	origOut, origErr := os.Stdout, os.Stderr
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout, os.Stderr = wOut, wErr
	outC, errC := make(chan string), make(chan string)
	go func() { var b bytes.Buffer; _, _ = io.Copy(&b, rOut); outC <- b.String() }()
	go func() { var b bytes.Buffer; _, _ = io.Copy(&b, rErr); errC <- b.String() }()
	defer func() { os.Stdout, os.Stderr = origOut, origErr }()
	f()
	_ = wOut.Close()
	_ = wErr.Close()
	return <-outC, <-errC
}

// --- printer selection & routing -------------------------------------------

func TestGetPrinterFunc_AutoResolvesPerScenario(t *testing.T) {
	// "auto" maps to a concrete printer based on the run scenario; the resolved
	// printer's routing (stdout vs stderr) is what this locks.
	cases := []struct {
		scenario       scenarioCode
		wantStdoutHit  bool // message lands on stdout
		wantStderrHit  bool // message lands on stderr
		wantNilPrinter bool // "basic" -> nil (starbox default)
	}{
		{scenarioREPL, true, false, false},   // -> stdout
		{scenarioDirect, true, false, false}, // -> stdout
		{scenarioFile, false, true, false},   // -> lineno (stderr)
		{scenarioWeb, false, false, true},    // -> basic (nil)
	}
	for _, c := range cases {
		pf, err := getPrinterFunc(c.scenario, "auto")
		if err != nil {
			t.Fatalf("scenario %d auto: unexpected error %v", c.scenario, err)
		}
		if c.wantNilPrinter {
			if pf != nil {
				t.Errorf("scenario %d auto: want nil printer (basic), got non-nil", c.scenario)
			}
			continue
		}
		so, se := captureStd(t, func() { pf(nil, "PING") })
		if got := strings.Contains(so, "PING"); got != c.wantStdoutHit {
			t.Errorf("scenario %d auto: stdout hit=%v want %v (stdout=%q)", c.scenario, got, c.wantStdoutHit, so)
		}
		if got := strings.Contains(se, "PING"); got != c.wantStderrHit {
			t.Errorf("scenario %d auto: stderr hit=%v want %v (stderr=%q)", c.scenario, got, c.wantStderrHit, se)
		}
	}
}

func TestGetPrinterFunc_NamedPrinters(t *testing.T) {
	t.Run("stdout", func(t *testing.T) {
		pf, err := getPrinterFunc(scenarioDirect, "stdout")
		if err != nil || pf == nil {
			t.Fatalf("stdout: pf=%v err=%v", pf, err)
		}
		so, se := captureStd(t, func() { pf(nil, "msg") })
		if so != "msg\n" {
			t.Errorf("stdout: got stdout %q want %q", so, "msg\n")
		}
		if se != "" {
			t.Errorf("stdout: unexpected stderr %q", se)
		}
	})

	t.Run("stderr", func(t *testing.T) {
		pf, _ := getPrinterFunc(scenarioDirect, "stderr")
		so, se := captureStd(t, func() { pf(nil, "msg") })
		if se != "msg\n" || so != "" {
			t.Errorf("stderr: stdout=%q stderr=%q", so, se)
		}
	})

	for _, name := range []string{"none", "nil", "no"} {
		t.Run("silent/"+name, func(t *testing.T) {
			pf, err := getPrinterFunc(scenarioDirect, name)
			if err != nil || pf == nil {
				t.Fatalf("%s: pf=%v err=%v", name, pf, err)
			}
			so, se := captureStd(t, func() { pf(nil, "msg") })
			if so != "" || se != "" {
				t.Errorf("%s: expected no output, got stdout=%q stderr=%q", name, so, se)
			}
		})
	}

	t.Run("basic/nil", func(t *testing.T) {
		pf, err := getPrinterFunc(scenarioWeb, "basic")
		if err != nil {
			t.Fatalf("basic: err=%v", err)
		}
		if pf != nil {
			t.Errorf("basic: want nil printer (starbox default), got non-nil")
		}
	})

	t.Run("lineno", func(t *testing.T) {
		pf, _ := getPrinterFunc(scenarioFile, "lineno")
		_, se := captureStd(t, func() { pf(nil, "hello") })
		// [0001](HH:MM:SS.mmm)<emoji> hello
		re := regexp.MustCompile(`^\[0001\]\(\d{2}:\d{2}:\d{2}\.\d{3}\).* hello\n$`)
		if !re.MatchString(se) {
			t.Errorf("lineno: stderr %q does not match %v", se, re)
		}
	})

	t.Run("since", func(t *testing.T) {
		pf, _ := getPrinterFunc(scenarioFile, "since")
		_, se := captureStd(t, func() { pf(nil, "hello") })
		// [0001](S.SSS)<emoji> hello
		re := regexp.MustCompile(`^\[0001\]\(\d+\.\d{3}\).* hello\n$`)
		if !re.MatchString(se) {
			t.Errorf("since: stderr %q does not match %v", se, re)
		}
	})

	t.Run("unknown", func(t *testing.T) {
		pf, err := getPrinterFunc(scenarioDirect, "bogus")
		if pf != nil || err == nil {
			t.Fatalf("unknown printer: want (nil, err), got (%v, %v)", pf, err)
		}
		if err.Error() != "unknown printer name: bogus" {
			t.Errorf("unknown printer: got %q", err.Error())
		}
	})

	t.Run("case-and-space-insensitive", func(t *testing.T) {
		pf, err := getPrinterFunc(scenarioDirect, "  STDOUT ")
		if err != nil || pf == nil {
			t.Fatalf("normalised name: pf=%v err=%v", pf, err)
		}
		so, _ := captureStd(t, func() { pf(nil, "x") })
		if so != "x\n" {
			t.Errorf("normalised name: stdout=%q", so)
		}
	})
}

// --- default module set & resolution ---------------------------------------

func TestGetDefaultModules(t *testing.T) {
	mods := getDefaultModules()

	// sorted, no duplicates
	if !sort.StringsAreSorted(mods) {
		t.Errorf("default modules are not sorted: %v", mods)
	}
	seen := map[string]bool{}
	for _, m := range mods {
		if seen[m] {
			t.Errorf("duplicate module %q in default set", m)
		}
		seen[m] = true
	}

	// every CLI module is in the default set...
	for _, want := range []string{"args", "sys", "gum", "email", "llm", "markdown", "cmd", "sqlite", "web", "s3"} {
		if !seen[want] {
			t.Errorf("default modules missing CLI module %q", want)
		}
	}
	// ...alongside a representative sample of starlet builtins.
	for _, want := range []string{"math", "json", "time", "re", "string"} {
		if !seen[want] {
			t.Errorf("default modules missing starlet builtin %q", want)
		}
	}
}

func TestLoadCLIModuleByName(t *testing.T) {
	opts := &BoxOpts{cmdArgs: []string{}}

	for _, name := range cliMods {
		loader, err := loadCLIModuleByName(opts, name)
		if err != nil {
			t.Errorf("loadCLIModuleByName(%q): unexpected error %v", name, err)
			continue
		}
		if loader == nil {
			t.Errorf("loadCLIModuleByName(%q): nil loader", name)
		}
	}

	if _, err := loadCLIModuleByName(opts, "definitely-not-a-module"); err == nil {
		t.Errorf("unknown module: want error, got nil")
	} else if !strings.Contains(err.Error(), "unknown module: definitely-not-a-module") {
		t.Errorf("unknown module: got %q", err.Error())
	}
}

// --- run-mode dispatch & exit codes ----------------------------------------

// baseArgs returns Args wired the way ParseArgs would for a typical invocation,
// so Process exercises the real BuildBox + module path.
func baseArgs() *Args {
	return &Args{
		AllowGlobalReassign: true,
		ModulesToLoad:       getDefaultModules(),
		IncludePath:         ".",
		LogLevel:            "panic",
		OutputPrinter:       "auto",
	}
}

func TestProcess_DirectCode_Success(t *testing.T) {
	a := baseArgs()
	a.CodeContent = `print(1 + 2)`
	a.OutputPrinter = "stdout"

	var code int
	so, _ := captureStd(t, func() { code = Process(a) })
	if code != 0 {
		t.Errorf("direct code success: exit=%d want 0", code)
	}
	if so != "3\n" {
		t.Errorf("direct code success: stdout=%q want %q", so, "3\n")
	}
}

func TestProcess_DirectCode_LoadsModules(t *testing.T) {
	// End-to-end module wiring: load a member from the sys module and print it.
	a := baseArgs()
	a.ModulesToLoad = []string{"sys"}
	a.CodeContent = `load("sys", "platform"); print(platform)`
	a.OutputPrinter = "stdout"

	var code int
	so, _ := captureStd(t, func() { code = Process(a) })
	if code != 0 {
		t.Errorf("module load: exit=%d want 0", code)
	}
	if strings.TrimSpace(so) != runtime.GOOS {
		t.Errorf("module load: stdout=%q want platform %q", so, runtime.GOOS)
	}
}

func TestProcess_DirectCode_EvalError(t *testing.T) {
	a := baseArgs()
	a.CodeContent = `x = 1 // 0`
	a.OutputPrinter = "stdout"

	var code int
	_, se := captureStd(t, func() { code = Process(a) })
	if code != 1 {
		t.Errorf("eval error: exit=%d want 1", code)
	}
	if !strings.Contains(se, "floored division by zero") {
		t.Errorf("eval error: stderr %q missing the runtime error", se)
	}
}

func TestProcess_DirectCode_UnknownModule(t *testing.T) {
	// Under the default (open) posture there is no load gate, so an unknown -m
	// module reaches the dynamic loader and errors. (Under a restrictive tier
	// the policy instead drops it from preload — see capability_test.go.)
	a := baseArgs()
	a.ModulesToLoad = []string{"no-such-module"}
	a.CodeContent = `print(1)`

	var code int
	_, se := captureStd(t, func() { code = Process(a) })
	if code != 1 {
		t.Errorf("unknown module: exit=%d want 1", code)
	}
	if !strings.Contains(se, "unknown module: no-such-module") {
		t.Errorf("unknown module: stderr %q missing the diagnostic", se)
	}
}

func TestProcess_ScriptFile(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "hello.star")
	if err := os.WriteFile(script, []byte("print('from file')\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	a := baseArgs()
	a.IncludePath = dir
	a.Arguments = []string{script}
	a.NumberOfArgs = 1
	a.OutputPrinter = "stdout"

	var code int
	so, _ := captureStd(t, func() { code = Process(a) })
	if code != 0 {
		t.Errorf("script file: exit=%d want 0", code)
	}
	if so != "from file\n" {
		t.Errorf("script file: stdout=%q want %q", so, "from file\n")
	}
}

func TestProcess_ScriptFile_NotFound(t *testing.T) {
	a := baseArgs()
	a.Arguments = []string{filepath.Join(t.TempDir(), "missing.star")}
	a.NumberOfArgs = 1

	var code int
	captureStd(t, func() { code = Process(a) })
	if code != 1 {
		t.Errorf("missing file: exit=%d want 1", code)
	}
}

func TestProcess_Version(t *testing.T) {
	a := baseArgs()
	a.ShowVersion = true

	var code int
	captureStd(t, func() { code = Process(a) })
	if code != 0 {
		t.Errorf("version: exit=%d want 0", code)
	}
}

func TestProcess_WebNoCode(t *testing.T) {
	// Web mode with neither -c nor a file errors out before binding a port.
	a := baseArgs()
	a.WebPort = 8080

	var code int
	_, se := captureStd(t, func() { code = Process(a) })
	if code != 1 {
		t.Errorf("web no code: exit=%d want 1", code)
	}
	if !strings.Contains(se, "no code to run as web server") {
		t.Errorf("web no code: stderr %q missing the diagnostic", se)
	}
}

func TestProcess_UnknownPrinterIsError(t *testing.T) {
	a := baseArgs()
	a.CodeContent = `print(1)`
	a.OutputPrinter = "bogus"

	var code int
	_, se := captureStd(t, func() { code = Process(a) })
	if code != 1 {
		t.Errorf("unknown printer: exit=%d want 1", code)
	}
	if !strings.Contains(se, "unknown printer name: bogus") {
		t.Errorf("unknown printer: stderr %q missing the diagnostic", se)
	}
}

func TestProcess_DirectCode_Interactive(t *testing.T) {
	// InteractiveMode wires the inspect callback (genInspectCond(true)); a
	// successful script still exits 0.
	a := baseArgs()
	a.CodeContent = `print("hi")`
	a.OutputPrinter = "stdout"
	a.InteractiveMode = true

	var code int
	so, _ := captureStd(t, func() { code = Process(a) })
	if code != 0 {
		t.Errorf("interactive direct: exit=%d want 0", code)
	}
	if !strings.Contains(so, "hi") {
		t.Errorf("interactive direct: stdout=%q missing output", so)
	}
}

// NOTE: REPL (runREPL) is not characterized in-process: starbox's REPL reads
// via the chzyer/readline library from its own terminal fd, not the os.Stdin
// global, so an injected pipe never reaches it. The refactor only relocates
// runREPL unchanged.

func TestBuildBox_Toggles(t *testing.T) {
	// Both recursion/global-reassign toggles build a usable box.
	for _, tc := range []struct {
		name           string
		recursion      bool
		globalReassign bool
	}{
		{"recursion+reassign", true, true},
		{"strict", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			box, err := BuildBox(&BoxOpts{
				scenario:       scenarioDirect,
				name:           "t",
				includePath:    ".",
				moduleToLoad:   []string{"sys"},
				printerName:    "stdout",
				recursion:      tc.recursion,
				globalReassign: tc.globalReassign,
			})
			if err != nil {
				t.Fatalf("BuildBox: %v", err)
			}
			if box == nil {
				t.Fatalf("BuildBox returned nil box")
			}
		})
	}
}

// guard: starlark import is used (keeps the PrintFunc signature honest if the
// printer call sites change during the refactor).
var _ = starlark.Thread{}
