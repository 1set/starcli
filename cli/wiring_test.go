package cli

// Tests for the starbox v0.2.0 runtime wiring (CLI-01/M2): execution budgets
// and RunError-classified exit codes. Reuses captureStd / baseArgs from
// characterization_test.go (same package).
//
// Sections:
//   - exit-code classification (success / eval / syntax / compile)
//   - execution budgets (--max-steps / --max-output)

import (
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
