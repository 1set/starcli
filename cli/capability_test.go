package cli

// Tests for the CLI-01/M2 opt-in capability load gate (default open; tightened
// via --caps / STAR_CAPS / --allow-*). Reuses captureStd / baseArgs from
// characterization_test.go (same package).
//
// Sections:
//   - grant logic (grantFromFlags + moduleAllowed) across tiers/flags
//   - allowedModules filtering
//   - open default loads everything; invalid tier errors
//   - end-to-end gating through Process under an explicit tier

import (
	"strings"
	"testing"
)

func TestModuleAllowed(t *testing.T) {
	safe := grantFromFlags("safe", false, false, false)
	network := grantFromFlags("network", false, false, false)
	allowNet := grantFromFlags("safe", true, false, false)
	allowFS := grantFromFlags("safe", false, true, false)
	full := grantFromFlags("full", false, false, false)
	fullCmd := grantFromFlags("full", false, false, true)
	allowCmd := grantFromFlags("safe", false, false, true)
	netFS := grantFromFlags("safe", true, true, false) // --allow-net --allow-fs
	open := grantFromFlags("", false, false, false)    // default/empty == open
	openX := grantFromFlags("open", false, false, false)

	cases := []struct {
		grant capGrant
		name  string
		want  bool
	}{
		// Safe tier: pure/log/process yes; net/fs/cmd no.
		{safe, "math", true}, {safe, "json", true}, {safe, "log", true},
		{safe, "runtime", true}, {safe, "sys", true}, {safe, "gum", true},
		{safe, "markdown", true},
		{safe, "http", false}, {safe, "net", false}, {safe, "web", false},
		{safe, "email", false}, {safe, "llm", false}, {safe, "s3", false},
		{safe, "file", false}, {safe, "path", false}, {safe, "sqlite", false},
		{safe, "cmd", false},
		{safe, "no-such-module", false}, // unknown -> deny

		// Network tier / --allow-net: pure-network modules yes; fs/cmd no. The
		// dual-capability modules (web/s3/sqlite) need BOTH net and fs, so a
		// net-only grant does NOT admit them.
		{network, "http", true}, {network, "email", true}, {network, "llm", true},
		{network, "web", false}, {network, "sqlite", false},
		{network, "cmd", false},
		{allowNet, "web", false}, {allowNet, "file", false},

		// --allow-fs: pure-fs modules yes; the dual modules still need net too.
		{allowFS, "file", true}, {allowFS, "path", true},
		{allowFS, "sqlite", false}, {allowFS, "web", false},
		{allowFS, "http", false}, {allowFS, "cmd", false},

		// --allow-net + --allow-fs (or --caps full): the dual modules load.
		{netFS, "web", true}, {netFS, "sqlite", true},
		{netFS, "cmd", false}, // still no exec

		// Full: net + fs, but cmd still requires --allow-cmd.
		{full, "web", true}, {full, "sqlite", true}, {full, "cmd", false},
		{fullCmd, "cmd", true},
		{allowCmd, "cmd", true}, {allowCmd, "web", false}, // allow-cmd alone doesn't grant net

		// Open (the default / empty tier): everything, including cmd.
		{open, "web", true}, {open, "sqlite", true},
		{open, "http", true}, {open, "file", true}, {open, "cmd", true},
		{openX, "cmd", true}, {openX, "web", true},
		{open, "no-such-module", false}, // an unknown name is still not a module
	}
	for _, c := range cases {
		if got := c.grant.moduleAllowed(c.name); got != c.want {
			t.Errorf("grant %+v moduleAllowed(%q)=%v want %v", c.grant, c.name, got, c.want)
		}
	}
}

func TestAllowedModules(t *testing.T) {
	in := []string{"math", "http", "cmd", "sqlite", "sys"}
	safe := grantFromFlags("safe", false, false, false).allowedModules(in)
	if got := strings.Join(safe, ","); got != "math,sys" {
		t.Errorf("safe allowedModules=%v want [math sys]", safe)
	}
	full := grantFromFlags("full", false, false, false).allowedModules(in)
	if got := strings.Join(full, ","); got != "math,http,sqlite,sys" {
		t.Errorf("full allowedModules=%v want [math http sqlite sys]", full)
	}
	fullCmd := grantFromFlags("full", false, false, true).allowedModules(in)
	if got := strings.Join(fullCmd, ","); got != "math,http,cmd,sqlite,sys" {
		t.Errorf("full+cmd allowedModules=%v want all", fullCmd)
	}
}

func TestProcess_OpenDefaultLoadsEverything(t *testing.T) {
	// The default posture (no --caps, no STAR_CAPS) is open: net/fs/cmd modules
	// load with no flag at all.
	for _, code := range []string{
		`load("http", "get"); print(type(get))`,
		`load("cmd", "run"); print(type(run))`,
		`load("sqlite", "connect"); print(type(connect))`,
	} {
		a := baseArgs() // Caps == "" -> open
		a.OutputPrinter = "stdout"
		a.CodeContent = code
		var rc int
		_, se := captureStd(t, func() { rc = Process(a) })
		if rc != 0 {
			t.Errorf("open default: %q exit=%d want 0 (stderr=%q)", code, rc, se)
		}
	}
}

func TestProcess_InvalidCapsErrors(t *testing.T) {
	// A typo'd tier must fail loudly, never silently fall open.
	a := baseArgs()
	a.Caps = "saef"
	a.CodeContent = `print(1)`
	var rc int
	_, se := captureStd(t, func() { rc = Process(a) })
	if rc == 0 {
		t.Errorf("invalid caps: expected non-zero exit")
	}
	if !strings.Contains(se, "unknown --caps value") {
		t.Errorf("invalid caps: stderr %q missing the diagnostic", se)
	}
}

func TestProcess_CapabilityGate_EndToEnd(t *testing.T) {
	type tc struct {
		name     string
		setup    func(*Args)
		code     string
		wantZero bool
		wantErr  string // substring expected in stderr when not zero
	}
	cases := []tc{
		{"safe allows sys", nil, `load("sys", "platform"); print(platform)`, true, ""},
		{"safe allows math", nil, `load("math", "floor"); print(floor(3.7))`, true, ""},
		{"safe withholds http", nil, `load("http", "get")`, false, "withheld"},
		{"safe denies web", nil, `load("web", "get")`, false, "web"},
		{"safe denies cmd", nil, `load("cmd", "run")`, false, "cmd"},
		{"allow-net opens http", func(a *Args) { a.AllowNet = true }, `load("http", "get"); print(type(get))`, true, ""},
		{"allow-cmd opens cmd", func(a *Args) { a.AllowCmd = true }, `load("cmd", "run"); print(type(run))`, true, ""},
		{"full keeps cmd denied", func(a *Args) { a.Caps = "full" }, `load("cmd", "run")`, false, "cmd"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := baseArgs()
			a.OutputPrinter = "stdout"
			a.Caps = "safe" // gate baseline; cases widen via setup
			if c.setup != nil {
				c.setup(a)
			}
			a.CodeContent = c.code
			var code int
			_, se := captureStd(t, func() { code = Process(a) })
			if c.wantZero {
				if code != 0 {
					t.Errorf("%s: exit=%d want 0 (stderr=%q)", c.name, code, se)
				}
				return
			}
			if code == 0 {
				t.Errorf("%s: expected non-zero exit, got 0", c.name)
			}
			if c.wantErr != "" && !strings.Contains(se, c.wantErr) {
				t.Errorf("%s: stderr %q missing %q", c.name, se, c.wantErr)
			}
		})
	}
}
