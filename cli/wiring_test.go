package cli

// Tests for the starbox v0.2.0 runtime wiring: execution budgets and
// RunError-classified exit codes (CLI-01/M2), and the --check validation mode
// (CLI-02/C-5). Reuses captureStd / baseArgs from characterization_test.go.
//
// Sections:
//   - exit-code classification (success / eval / syntax / compile)
//   - execution budgets (--max-steps / --max-output)
//   - check mode (--check: resolve without executing)

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/1set/starbox"
)

func TestProcess_ExitCodeClassification(t *testing.T) {
	cases := []struct {
		name string
		code string
		want int
	}{
		{"success", `print(1)`, exitOK},
		{"eval", `x = 1 // 0`, exitError},             // runtime error
		{"syntax", `x = =`, exitSyntax},               // not valid Starlark
		{"compile", `y = undefined_xyz`, exitCompile}, // undefined name (resolve)
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := baseArgs()
			a.CodeContent = c.code
			a.OutputPrinter = "stdout"
			var code int
			captureStd(t, func() { code = Process(a) })
			if code != c.want {
				t.Errorf("%s: exit=%d want %d", c.name, code, c.want)
			}
		})
	}
}

func TestProcess_MaxStepsBudget(t *testing.T) {
	a := baseArgs()
	a.OutputPrinter = "stdout"
	a.MaxSteps = 100
	a.CodeContent = "s = 0\nfor i in range(100000):\n    s += 1\nprint(s)\n"

	var code int
	so, se := captureStd(t, func() { code = Process(a) })
	if code != exitMaxSteps {
		t.Errorf("max-steps: exit=%d want %d", code, exitMaxSteps)
	}
	if so != "" {
		t.Errorf("max-steps: expected no stdout, got %q", so)
	}
	if !strings.Contains(se, "step budget exceeded") {
		t.Errorf("max-steps: stderr %q missing the banner", se)
	}
}

func TestProcess_MaxStepsBudget_Generous(t *testing.T) {
	// A generous budget does not interfere with a small run.
	a := baseArgs()
	a.OutputPrinter = "stdout"
	a.MaxSteps = 1_000_000
	a.CodeContent = "s = 0\nfor i in range(10):\n    s += 1\nprint(s)\n"

	var code int
	so, _ := captureStd(t, func() { code = Process(a) })
	if code != exitOK {
		t.Errorf("generous budget: exit=%d want 0", code)
	}
	if strings.TrimSpace(so) != "10" {
		t.Errorf("generous budget: stdout=%q want 10", so)
	}
}

func TestProcess_MaxOutputBudget(t *testing.T) {
	a := baseArgs()
	a.OutputPrinter = "stdout"
	a.MaxOutput = 1
	a.CodeContent = "a = 1\nb = 2\nc = 3\n"

	var code int
	_, se := captureStd(t, func() { code = Process(a) })
	if code != exitOutputLimit {
		t.Errorf("max-output: exit=%d want %d", code, exitOutputLimit)
	}
	if !strings.Contains(se, "output entry limit exceeded") {
		t.Errorf("max-output: stderr %q missing the banner", se)
	}
}

func TestBuildBox_BudgetsApplied(t *testing.T) {
	// Unit check that BuildBox actually wires the step budget into the box.
	box, err := BuildBox(&BoxOpts{
		scenario:     scenarioDirect,
		name:         "t",
		includePath:  ".",
		moduleToLoad: []string{},
		printerName:  "none",
		maxSteps:     100,
	})
	if err != nil {
		t.Fatalf("BuildBox: %v", err)
	}
	// A list comprehension is an expression (no top-level-control dialect
	// needed) that still burns far more than 100 steps.
	_, err = box.Run("data = [i * i for i in range(100000)]\n")
	if err == nil {
		t.Fatalf("expected a step-budget error, got nil")
	}
	if starbox.ClassifyRunError(err).Kind != starbox.RunErrorMaxSteps {
		t.Errorf("expected RunErrorMaxSteps, got kind %v (%v)", starbox.ClassifyRunError(err).Kind, err)
	}
}

// --- log-file routing (--log-file) ----------------------------------------

func TestProcess_LogFile(t *testing.T) {
	// A script's log module output is routed to the --log-file at the
	// interpreter level (nested directory auto-created).
	dir := t.TempDir()
	// Registered after TempDir so it runs first (LIFO): release the open handle
	// before TempDir's RemoveAll, which on Windows cannot delete an open file.
	t.Cleanup(closeLogFiles)
	logPath := filepath.Join(dir, "logs", "run.log")
	a := baseArgs()
	a.LogFile = logPath
	a.CodeContent = `load("log", "info"); info("captured-line")`

	var code int
	captureStd(t, func() { code = Process(a) })
	if code != 0 {
		t.Fatalf("log-file run: exit=%d want 0", code)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if !strings.Contains(string(data), "captured-line") {
		t.Errorf("log file missing the message: %q", string(data))
	}
	if !strings.Contains(string(data), "info") {
		t.Errorf("log file missing the level: %q", string(data))
	}
}

func TestProcess_LogFile_JSON(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(closeLogFiles)
	logPath := filepath.Join(dir, "run.json")
	a := baseArgs()
	a.LogFile = logPath
	a.LogFormat = "json"
	a.CodeContent = `load("log", "info"); info("jsonline", n=7)`

	var code int
	captureStd(t, func() { code = Process(a) })
	if code != 0 {
		t.Fatalf("json log run: exit=%d want 0", code)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	for _, want := range []string{`"level":"info"`, `"msg":"jsonline"`, `"n":7`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("json log %q missing %q", string(data), want)
		}
	}
}

func TestProcess_BadLogFormat(t *testing.T) {
	a := baseArgs()
	a.LogFile = filepath.Join(t.TempDir(), "x.log")
	a.LogFormat = "yaml"
	a.CodeContent = `print(1)`
	var code int
	_, se := captureStd(t, func() { code = Process(a) })
	if code == 0 {
		t.Errorf("bad log-format: expected non-zero exit")
	}
	if !strings.Contains(se, "unknown --log-format") {
		t.Errorf("bad log-format: stderr %q missing the diagnostic", se)
	}
}

// --- session recording (--record) -----------------------------------------

func TestProcess_Record(t *testing.T) {
	recPath := filepath.Join(t.TempDir(), "rec", "session.txt")
	a := baseArgs()
	a.OutputPrinter = "stdout"
	a.Record = recPath
	a.CodeContent = `print("recorded-line"); print(6 * 7)`

	var code int
	so, _ := captureStd(t, func() { code = Process(a) })
	if code != 0 {
		t.Fatalf("record run: exit=%d want 0", code)
	}
	// The output is still shown (tee'd to the original stdout)...
	if !strings.Contains(so, "recorded-line") || !strings.Contains(so, "42") {
		t.Errorf("record: stdout %q missing the live output", so)
	}
	// ...and captured to the transcript (with a session header).
	data, err := os.ReadFile(recPath)
	if err != nil {
		t.Fatalf("read transcript: %v", err)
	}
	for _, want := range []string{"starcli session", "recorded-line", "42"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("transcript %q missing %q", string(data), want)
		}
	}
}

func TestProcess_Record_CapturesErrors(t *testing.T) {
	recPath := filepath.Join(t.TempDir(), "err.txt")
	a := baseArgs()
	a.Record = recPath
	a.CodeContent = `x = 1 // 0`

	var code int
	captureStd(t, func() { code = Process(a) })
	if code != exitError {
		t.Errorf("record error run: exit=%d want %d", code, exitError)
	}
	data, _ := os.ReadFile(recPath)
	if !strings.Contains(string(data), "floored division by zero") {
		t.Errorf("transcript %q missing the error", string(data))
	}
}

// --- check mode (--check) -------------------------------------------------

func TestProcess_Check(t *testing.T) {
	t.Run("clean code passes", func(t *testing.T) {
		a := baseArgs()
		a.Check = true
		a.CodeContent = `x = 1 + 2`
		var code int
		_, se := captureStd(t, func() { code = Process(a) })
		if code != 0 {
			t.Errorf("clean check: exit=%d want 0 (stderr=%q)", code, se)
		}
		if se != "" {
			t.Errorf("clean check: unexpected diagnostics %q", se)
		}
	})

	t.Run("clean code does NOT execute", func(t *testing.T) {
		// --check must not run the script: a print produces no stdout.
		a := baseArgs()
		a.Check = true
		a.OutputPrinter = "stdout"
		a.CodeContent = `print("SHOULD-NOT-RUN")`
		var code int
		so, _ := captureStd(t, func() { code = Process(a) })
		if code != 0 {
			t.Errorf("check no-exec: exit=%d want 0", code)
		}
		if strings.Contains(so, "SHOULD-NOT-RUN") {
			t.Errorf("check no-exec: the script executed (stdout=%q)", so)
		}
	})

	t.Run("syntax error is reported", func(t *testing.T) {
		a := baseArgs()
		a.Check = true
		a.CodeContent = `x = =`
		var code int
		_, se := captureStd(t, func() { code = Process(a) })
		if code != 1 {
			t.Errorf("syntax check: exit=%d want 1", code)
		}
		if !strings.Contains(se, "direct.star:1:") {
			t.Errorf("syntax check: stderr %q missing a file:line diagnostic", se)
		}
	})

	t.Run("undefined name is reported", func(t *testing.T) {
		a := baseArgs()
		a.Check = true
		a.CodeContent = `y = undefined_xyz`
		var code int
		_, se := captureStd(t, func() { code = Process(a) })
		if code != 1 {
			t.Errorf("undefined check: exit=%d want 1", code)
		}
		if !strings.Contains(se, "undefined: undefined_xyz") {
			t.Errorf("undefined check: stderr %q missing the diagnostic", se)
		}
	})

	t.Run("file is checked with its real name", func(t *testing.T) {
		dir := t.TempDir()
		f := filepath.Join(dir, "thing.star")
		if err := os.WriteFile(f, []byte("ok = 1\nbad = nope_name\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		a := baseArgs()
		a.Check = true
		a.Arguments = []string{f}
		a.NumberOfArgs = 1
		var code int
		_, se := captureStd(t, func() { code = Process(a) })
		if code != 1 {
			t.Errorf("file check: exit=%d want 1", code)
		}
		if !strings.Contains(se, "thing.star:2:") {
			t.Errorf("file check: stderr %q missing the real filename diagnostic", se)
		}
	})

	t.Run("nothing to check", func(t *testing.T) {
		a := baseArgs()
		a.Check = true
		var code int
		_, se := captureStd(t, func() { code = Process(a) })
		if code != 1 {
			t.Errorf("empty check: exit=%d want 1", code)
		}
		if !strings.Contains(se, "nothing to check") {
			t.Errorf("empty check: stderr %q missing the hint", se)
		}
	})
}
