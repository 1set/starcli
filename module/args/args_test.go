package args

// Tests for the argparse-style args module. Each case runs a small Starlark
// script with ArgumentParser/argv predeclared and inspects the resulting
// globals (or the error), exercising the full add_argument -> parse_args path.
//
// Sections:
//   - parsing: defaults / named / =form / positional / store_true / types
//   - errors: missing required / bad value / unknown option / too many
//   - argv default (argv[1:]) and the -- terminator
//   - format_help

import (
	"strings"
	"testing"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

func run(t *testing.T, argv []string, script string) (starlark.StringDict, error) {
	t.Helper()
	sd, err := NewModule(argv)()
	if err != nil {
		t.Fatalf("loader: %v", err)
	}
	mod, ok := sd[ModuleName].(*starlarkstruct.Module)
	if !ok {
		t.Fatalf("module shape: %T", sd[ModuleName])
	}
	pre := starlark.StringDict{
		"ArgumentParser": mod.Members["ArgumentParser"],
		"argv":           mod.Members["argv"],
	}
	thread := &starlark.Thread{Name: "test"}
	return starlark.ExecFile(thread, "test.star", script, pre)
}

func mustRun(t *testing.T, argv []string, script string) starlark.StringDict {
	t.Helper()
	g, err := run(t, argv, script)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return g
}

func str(t *testing.T, g starlark.StringDict, name string) string {
	t.Helper()
	s, ok := starlark.AsString(g[name])
	if !ok {
		t.Fatalf("global %q is %v, not a string", name, g[name])
	}
	return s
}

const greetSpec = `
p = ArgumentParser(description = "greet")
p.add_argument("--name", default = "World")
p.add_argument("--count", type = int, default = 1)
p.add_argument("--shout", action = "store_true")
p.add_argument("file")
ns = p.parse_args(%s)
name = ns.name
count = ns.count
shout = ns.shout
file = ns.file
`

func TestParse_Defaults(t *testing.T) {
	g := mustRun(t, nil, fmtSpec(`["in.txt"]`))
	if got := str(t, g, "name"); got != "World" {
		t.Errorf("name=%q want World", got)
	}
	if g["count"] != starlark.MakeInt(1) {
		t.Errorf("count=%v want 1", g["count"])
	}
	if g["shout"] != starlark.False {
		t.Errorf("shout=%v want False", g["shout"])
	}
	if got := str(t, g, "file"); got != "in.txt" {
		t.Errorf("file=%q want in.txt", got)
	}
}

func TestParse_NamedAndTypes(t *testing.T) {
	g := mustRun(t, nil, fmtSpec(`["--name", "Kevin", "--count", "3", "--shout", "data.csv"]`))
	if got := str(t, g, "name"); got != "Kevin" {
		t.Errorf("name=%q want Kevin", got)
	}
	if g["count"] != starlark.MakeInt(3) {
		t.Errorf("count=%v want 3", g["count"])
	}
	if g["shout"] != starlark.True {
		t.Errorf("shout=%v want True", g["shout"])
	}
}

func TestParse_EqualsForm(t *testing.T) {
	g := mustRun(t, nil, fmtSpec(`["--name=Sam", "f.txt"]`))
	if got := str(t, g, "name"); got != "Sam" {
		t.Errorf("name=%q want Sam", got)
	}
}

func TestParse_FloatType(t *testing.T) {
	g := mustRun(t, nil, `
p = ArgumentParser()
p.add_argument("--ratio", type = float, default = 0.5)
ns = p.parse_args(["--ratio", "1.25"])
ratio = ns.ratio
`)
	if g["ratio"] != starlark.Float(1.25) {
		t.Errorf("ratio=%v want 1.25", g["ratio"])
	}
}

func TestParse_TypeAsString(t *testing.T) {
	// type="int" is accepted as well as type=int.
	g := mustRun(t, nil, `
p = ArgumentParser()
p.add_argument("--n", type = "int", default = 0)
ns = p.parse_args(["--n", "7"])
n = ns.n
`)
	if g["n"] != starlark.MakeInt(7) {
		t.Errorf("n=%v want 7", g["n"])
	}
}

func TestParse_DoubleDashTerminator(t *testing.T) {
	// after --, a token that looks like an option is a positional value
	g := mustRun(t, nil, `
p = ArgumentParser()
p.add_argument("file")
ns = p.parse_args(["--", "--weird"])
file = ns.file
`)
	if got := str(t, g, "file"); got != "--weird" {
		t.Errorf("file=%q want --weird", got)
	}
}

func TestParse_DefaultArgvIsTail(t *testing.T) {
	// parse_args() with no argument uses argv[1:] (argv[0] is the script name).
	g := mustRun(t, []string{"script.star", "--name", "Ada", "x"}, fmtSpec(``))
	if got := str(t, g, "name"); got != "Ada" {
		t.Errorf("name=%q want Ada (from default argv[1:])", got)
	}
	if got := str(t, g, "file"); got != "x" {
		t.Errorf("file=%q want x", got)
	}
}

func TestParse_Errors(t *testing.T) {
	cases := []struct {
		name, argv, want string
	}{
		{"missing positional", `["--name", "X"]`, "required: file"},
		{"bad int", `["--count", "abc", "f.txt"]`, "invalid int value"},
		{"unknown option", `["--bogus", "f.txt"]`, "unrecognized argument: --bogus"},
		{"too many positionals", `["a.txt", "b.txt"]`, "unexpected positional argument: b.txt"},
		{"option needs value", `["--name"]`, "expected one value"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := run(t, nil, fmtSpec(c.argv))
			if err == nil {
				t.Fatalf("expected an error")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error %q missing %q", err.Error(), c.want)
			}
		})
	}
}

func TestParse_RequiredOption(t *testing.T) {
	_, err := run(t, nil, `
p = ArgumentParser()
p.add_argument("--token", required = True)
p.parse_args([])
`)
	if err == nil || !strings.Contains(err.Error(), "required: --token") {
		t.Errorf("required option: got %v", err)
	}
}

func TestFormatHelp(t *testing.T) {
	g := mustRun(t, nil, `
p = ArgumentParser(prog = "tool", description = "does things")
p.add_argument("--name", help = "your name")
p.add_argument("file", help = "input file")
h = p.format_help()
`)
	h := str(t, g, "h")
	for _, want := range []string{"usage: tool", "does things", "--name", "file", "your name", "input file"} {
		if !strings.Contains(h, want) {
			t.Errorf("format_help missing %q:\n%s", want, h)
		}
	}
}

func TestUnknownTypeAndAction(t *testing.T) {
	if _, err := run(t, nil, `ArgumentParser().add_argument("--x", type = "weird")`); err == nil || !strings.Contains(err.Error(), "type must be") {
		t.Errorf("unknown type: got %v", err)
	}
	if _, err := run(t, nil, `ArgumentParser().add_argument("--x", action = "store_false")`); err == nil || !strings.Contains(err.Error(), "unknown action") {
		t.Errorf("unknown action: got %v", err)
	}
}

func fmtSpec(argv string) string {
	return strings.Replace(greetSpec, "%s", argv, 1)
}
