// Package e2e holds end-to-end tests that build the real starcli binary and run
// sample scripts through it, asserting stdout and the process exit code. This is
// the "build a binary -> run a .star -> compare stdout/exit" coverage the v0.1.0
// cost-price audit flagged as missing: it proves the wired modules actually
// *work* through the CLI, not merely that they load.
package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// binPath is the freshly built starcli binary, set up once in TestMain.
var binPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "starcli-e2e")
	if err != nil {
		panic(err)
	}
	binPath = filepath.Join(dir, "starcli")
	if isWindows() {
		binPath += ".exe"
	}
	// Build from the repo root (the parent of this e2e package directory).
	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Dir = ".."
	if out, err := build.CombinedOutput(); err != nil {
		_, _ = os.Stderr.WriteString("e2e: build failed: " + err.Error() + "\n" + string(out) + "\n")
		os.RemoveAll(dir)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func isWindows() bool { return os.PathSeparator == '\\' }

// runCLI runs the built binary with args and optional stdin, returning stdout,
// stderr, and the exit code.
func runCLI(t *testing.T, stdin string, args ...string) (stdout, stderr string, exit int) {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return so.String(), se.String(), ee.ExitCode()
		}
		t.Fatalf("run %v: %v", args, err)
	}
	return so.String(), se.String(), 0
}

func TestGolden(t *testing.T) {
	const anyNonZero = -1
	cases := []struct {
		name     string
		args     []string
		wantOut  string // exact stdout, checked when non-empty
		notOut   string // stdout must NOT contain this, checked when non-empty
		wantExit int    // exact exit code; anyNonZero (-1) means "any non-zero"
		errSub   string // stderr must contain this, checked when non-empty
	}{
		{name: "hello prints", args: []string{"-c", `print("hi", 6*7)`}, wantOut: "hi 42\n", wantExit: 0},
		// newly-wired pure domain modules actually run through the CLI:
		{name: "emoji module runs", args: []string{"-c", `load("emoji","emojize"); print(emojize("hi :wave:"))`}, wantOut: "hi \U0001F44B\n", wantExit: 0},
		{name: "yaml module runs", args: []string{"-c", `load("yaml","decode"); print(decode("n: 7")["n"])`}, wantOut: "7\n", wantExit: 0},
		{name: "qrcode module runs", args: []string{"-c", `load("qrcode","encode"); print(encode("x").size > 0)`}, wantOut: "True\n", wantExit: 0},
		// error handling + exit codes:
		{name: "fail aborts non-zero", args: []string{"-c", `fail("boom")`}, wantExit: anyNonZero, errSub: "boom"},
		// cmd execution gating through the real binary:
		{name: "cmd disabled by default", args: []string{"-c", `load("cmd","run"); run("go version")`}, wantExit: anyNonZero, errSub: "disabled"},
		{name: "allow-cmd runs a command", args: []string{"--allow-cmd", "-c", `load("cmd","run"); print(run("go version").success)`}, wantOut: "True\n", wantExit: 0},
		// capability gate:
		{name: "caps safe withholds http", args: []string{"--caps", "safe", "-c", `load("http","get")`}, wantExit: anyNonZero, errSub: "withheld"},
		// --check validates without running:
		{name: "check valid does not run", args: []string{"--check", "-c", `print("RAN")`}, notOut: "RAN", wantExit: 0},
		{name: "check invalid is non-zero", args: []string{"--check", "-c", `x =`}, wantExit: anyNonZero},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			so, se, exit := runCLI(t, "", c.args...)
			if c.wantExit == anyNonZero {
				if exit == 0 {
					t.Errorf("exit=0, want non-zero (stdout=%q stderr=%q)", so, se)
				}
			} else if exit != c.wantExit {
				t.Errorf("exit=%d, want %d (stdout=%q stderr=%q)", exit, c.wantExit, so, se)
			}
			if c.wantOut != "" && so != c.wantOut {
				t.Errorf("stdout=%q, want %q (stderr=%q)", so, c.wantOut, se)
			}
			if c.notOut != "" && strings.Contains(so, c.notOut) {
				t.Errorf("stdout=%q must not contain %q", so, c.notOut)
			}
			if c.errSub != "" && !strings.Contains(se, c.errSub) {
				t.Errorf("stderr=%q, want substring %q", se, c.errSub)
			}
		})
	}
}
