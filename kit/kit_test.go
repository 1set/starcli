package kit_test

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/1set/starbox"
	"github.com/1set/starcli/kit"
	"github.com/1set/starlet"
	"go.starlark.net/starlark"
	"go.uber.org/zap"
)

// lifeLoader is a trivial custom module loader used to prove the WithLoader /
// WithDynamicLoader wiring without dragging in any starpkg module.
func lifeLoader() (starlark.StringDict, error) {
	return starlark.StringDict{"answer": starlark.MakeInt(42)}, nil
}

func TestKit(t *testing.T) {
	tests := []struct {
		name   string
		opts   []kit.Option
		script string
		want   map[string]string // result key -> fmt.Sprint(value)
	}{
		{
			name:   "builtin module",
			opts:   []kit.Option{kit.WithModules("math")},
			script: `x = math.floor(3.7)`,
			want:   map[string]string{"x": "3"},
		},
		{
			name:   "inject global",
			opts:   []kit.Option{kit.WithGlobal("who", "world")},
			script: `greeting = "hi " + who`,
			want:   map[string]string{"greeting": "hi world"},
		},
		{
			name: "explicit loader",
			opts: []kit.Option{
				kit.WithLoader("life", lifeLoader),
				kit.WithModules("life"),
			},
			script: `load("life", "answer")
doubled = answer * 2`,
			want: map[string]string{"doubled": "84"},
		},
		{
			name: "dynamic loader",
			opts: []kit.Option{
				kit.WithDynamicLoader(func(name string) (starlet.ModuleLoader, error) {
					if name == "life" {
						return lifeLoader, nil
					}
					return nil, fmt.Errorf("unknown module: %s", name)
				}),
				kit.WithModules("life"),
			},
			script: `load("life", "answer")
tripled = answer * 3`,
			want: map[string]string{"tripled": "126"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := kit.Run(tt.script, tt.opts...)
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			for key, want := range tt.want {
				if got := fmt.Sprint(out[key]); got != want {
					t.Errorf("result[%q] = %q, want %q", key, got, want)
				}
			}
		})
	}
}

// TestRunFS proves a shell can ship a whole tree of .star files and run an entry
// point out of it (the embed.FS path).
func TestRunFS(t *testing.T) {
	fsys := fstest.MapFS{
		"main.star": &fstest.MapFile{Data: []byte(`load("lib.star", "double")
result = double(21)`)},
		"lib.star": &fstest.MapFile{Data: []byte(`def double(n):
    return n * 2`)},
	}
	out, err := kit.RunFS(fsys, "main.star")
	if err != nil {
		t.Fatalf("RunFS() error = %v", err)
	}
	if got := fmt.Sprint(out["result"]); got != "42" {
		t.Errorf("result = %q, want %q", got, "42")
	}
}

// TestOutputLimit proves the per-run output-entry cap is wired through Kit and
// surfaces starbox's typed error.
func TestOutputLimit(t *testing.T) {
	_, err := kit.Run(`a = 1
b = 2
c = 3`, kit.WithMaxOutputEntries(2))
	var limitErr starbox.OutputLimitExceededError
	if !errors.As(err, &limitErr) {
		t.Fatalf("error = %v, want OutputLimitExceededError", err)
	}
}

// TestRunError proves a script failure is returned, not swallowed or panicked.
func TestRunError(t *testing.T) {
	_, err := kit.Run(`fail("boom")`)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error = %v, want one mentioning boom", err)
	}
}

// TestPolicyGate proves WithPolicy installs a load gate: a module outside the
// allowlist cannot be loaded even though a loader exists for it.
func TestPolicyGate(t *testing.T) {
	allowed := starbox.Policy{Modules: starbox.ModuleAllow{Names: []string{"math"}}}
	out, err := kit.Run(`v = math.floor(9.9)`,
		kit.WithPolicy(allowed), kit.WithModules("math"))
	if err != nil {
		t.Fatalf("allowed module: Run() error = %v", err)
	}
	if got := fmt.Sprint(out["v"]); got != "9" {
		t.Errorf("v = %q, want %q", got, "9")
	}

	// json is not in the allowlist, so loading it must fail.
	if _, err := kit.Run(`x = json.encode({})`,
		kit.WithPolicy(allowed), kit.WithModules("json")); err == nil {
		t.Fatal("denied module: expected an error, got nil")
	}
}

// TestKnobsAndOptions exercises the dialect, budget, globals, print, and logger
// options together: the script reassigns a top-level global and recurses, both of
// which only resolve when those dialect knobs are enabled.
func TestKnobsAndOptions(t *testing.T) {
	var printed bytes.Buffer
	logger := zap.NewNop().Sugar()

	out, err := kit.Run(`x = 1
x = base + 1
def fact(n):
    if n <= 1:
        return 1
    return n * fact(n - 1)
result = fact(5)
print("computed")`,
		kit.WithModuleSet(starbox.EmptyModuleSet),
		kit.WithGlobals(starlet.StringAnyMap{"base": 10}),
		kit.WithGlobalReassign(true),
		kit.WithRecursion(true),
		kit.WithOutputConversion(false),
		kit.WithMaxSteps(1_000_000),
		kit.WithMaxOutputEntries(50),
		kit.WithLogger(logger),
		kit.WithPrintFunc(func(_ *starlark.Thread, msg string) { printed.WriteString(msg) }),
	)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(printed.String(), "computed") {
		t.Errorf("print output = %q, want it to contain %q", printed.String(), "computed")
	}
	if out["result"] == nil {
		t.Error("result global missing")
	}
}

// TestBoxReusable proves New(...).Box() yields a configured box the caller can
// drive directly — the seam the standard starcli builds on.
func TestBoxReusable(t *testing.T) {
	box, err := kit.New("demo", kit.WithModules("math")).Box()
	if err != nil {
		t.Fatalf("Box() error = %v", err)
	}
	out, err := box.Run(`v = math.pow(2, 10)`)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got := fmt.Sprint(out["v"]); got != "1024" {
		t.Errorf("v = %q, want %q", got, "1024")
	}
}
