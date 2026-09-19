// Package e2e holds end-to-end tests that build the real starcli binary and run
// sample scripts through it, asserting stdout and the process exit code. This is
// the "build a binary -> run a .star -> compare stdout/exit" coverage the v0.1.0
// cost-price audit flagged as missing: it proves the wired modules actually
// *work* through the CLI, not merely that they load.
//
// Sections: local imports, golden scripts, domain modules, stdin, containers.
package e2e

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// binPath is a freshly built binary or the installed release candidate.
var binPath string

func TestMain(m *testing.M) {
	if candidate := os.Getenv("STARCLI_TEST_BINARY"); candidate != "" {
		var err error
		binPath, err = filepath.Abs(candidate)
		if err != nil {
			panic(err)
		}
		os.Exit(m.Run())
	}
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

func TestIncludeAuthorization(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fixture.star"), []byte("value = 73"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		flags []string
		allow bool
	}{
		{"default open", nil, true},
		{"safe has no implicit CWD", []string{"--caps", "safe"}, false},
		{"network has no implicit CWD", []string{"--caps", "network"}, false},
		{"explicit relative root", []string{"--caps", "safe", "-I", "."}, true},
		{"explicit absolute root", []string{"--caps", "safe", "-I", dir}, true},
		{"filesystem grant", []string{"--caps", "safe", "--allow-fs"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append(append([]string{}, tc.flags...), "-c", `load("fixture.star", "value"); print(value)`)
			cmd := exec.Command(binPath, args...)
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()
			if (err == nil) != tc.allow {
				t.Fatalf("exit=%v, allowed=%v: %s", err, tc.allow, out)
			}
			if tc.allow && !strings.Contains(string(out), "73") {
				t.Fatalf("missing loaded value: %s", out)
			}
		})
	}
}

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
		// named time zones resolve via the embedded IANA tzdata (no host
		// /usr/share/zoneinfo needed — see main.go's time/tzdata import):
		{name: "named timezone is valid", args: []string{"-c", `load("time","is_valid_timezone"); print(is_valid_timezone("America/New_York"))`}, wantOut: "True\n", wantExit: 0},
		{name: "named timezone converts", args: []string{"-c", `load("time","parse_time"); print(parse_time("2021-03-22T12:00:00Z").in_location("America/New_York").hour)`}, wantOut: "8\n", wantExit: 0},
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

// TestDomainModuleIntegration proves the offline-testable domain modules
// actually *do their job* through the CLI — not merely that they load. sqlite
// runs a real local CRUD round-trip, markdown renders HTML, and gum's headless
// table renders without a TTY.
func TestDomainModuleIntegration(t *testing.T) {
	cases := []struct {
		name   string
		args   []string
		outSub []string // stdout must contain each of these
	}{
		{
			name: "sqlite local CRUD round-trip",
			args: []string{"-c", `
load("sqlite", "connect")
db = connect(":memory:")
db.execute("CREATE TABLE t (id INTEGER PRIMARY KEY, name TEXT)")
db.insert("t", {"name": "Ada"})
print(db.query("SELECT name FROM t")[0]["name"])
db.close()
`},
			outSub: []string{"Ada"},
		},
		{
			name:   "markdown renders HTML",
			args:   []string{"-c", `load("markdown", "convert"); print(convert(text="# Hi"))`},
			outSub: []string{"<h1", "Hi</h1>"},
		},
		{
			name:   "gum table renders headless",
			args:   []string{"--allow-cmd", "-c", `load("gum", "table"); print(table(["Name"], [["Ada"]]))`},
			outSub: []string{"Name", "Ada"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			so, se, exit := runCLI(t, "", c.args...)
			if exit != 0 {
				t.Fatalf("exit=%d, want 0 (stdout=%q stderr=%q)", exit, so, se)
			}
			for _, sub := range c.outSub {
				if !strings.Contains(so, sub) {
					t.Errorf("stdout=%q missing %q", so, sub)
				}
			}
		})
	}
}

func TestStdinConsumption(t *testing.T) {
	for _, tc := range []struct{ name, input, code, want string }{
		{"EOF tail", "last", `load("sys", "input"); print(input())`, "last\n"},
		{"mixed readers", "a\nb\nc", `load("sys", "input", "lines", "read"); print(input()); print(list(lines())); print(read())`, "a\n[\"b\", \"c\"]\n\n"},
		{"continuous input", "a\nb", `load("sys", "input"); print(input()); print(input())`, "a\nb\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, errOut, exit := runCLI(t, tc.input, "-c", tc.code)
			if exit != 0 || out != tc.want {
				t.Fatalf("exit=%d stdout=%q stderr=%q, want %q", exit, out, errOut, tc.want)
			}
		})
	}
}

// TestContainer exercises the repository Dockerfile, including its entrypoint,
// trust store, embedded time zones, non-root user, and PID 1 shutdown behavior.
// Build it for linux/amd64 and set STARCLI_TEST_IMAGE to opt in.
func TestContainer(t *testing.T) {
	image := os.Getenv("STARCLI_TEST_IMAGE")
	if image == "" {
		t.Skip("set STARCLI_TEST_IMAGE to test a built container image")
	}
	docker := func(args ...string) (string, error) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
		return string(out), err
	}
	runArgs := []string{"run", "--rm", "--platform", "linux/amd64", "--read-only",
		"--cap-drop=ALL", "--security-opt=no-new-privileges", "--memory=256m", "--cpus=1",
		"--pids-limit=64", "--tmpfs", "/tmp:rw,noexec,nosuid,size=16m"}
	run := func(options, args []string) (string, error) {
		all := append(append([]string{}, runArgs...), options...)
		all = append(all, image)
		return docker(append(all, args...)...)
	}
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"arguments reach CLI", []string{"-c", `print(6 * 7)`}, "42\n"},
		{"non-root user", []string{"--allow-cmd", "-c", `load("cmd", "run"); print(run("id -u").stdout.strip())`}, "65532\n"},
		{"embedded timezone", []string{"-c", `load("time", "parse_time"); print(parse_time("2021-03-22T12:00:00Z").in_location("America/New_York").hour)`}, "8\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := run(nil, tc.args)
			if err != nil || out != tc.want {
				t.Fatalf("output=%q error=%v, want %q", out, err, tc.want)
			}
		})
	}
	t.Run("exit status", func(t *testing.T) {
		out, err := run(nil, []string{"-c", `fail("container failure")`})
		if err == nil || !strings.Contains(out, "container failure") {
			t.Fatalf("output=%q error=%v, want script failure", out, err)
		}
	})
	t.Run("HTTPS trust", func(t *testing.T) {
		bundle, err := run([]string{"--entrypoint", "/bin/cat"}, []string{"/etc/ssl/certs/ca-certificates.crt"})
		if err != nil || !x509.NewCertPool().AppendCertsFromPEM([]byte(bundle)) {
			t.Fatalf("image must include a usable CA bundle: %v", err)
		}
		// The standard httptest certificate covers example.com. Resolve that
		// name to the local Docker host, so this test needs no public service.
		srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "trusted fixture")
		}))
		_ = srv.Listener.Close()
		srv.Listener, err = net.Listen("tcp", "0.0.0.0:0")
		if err != nil {
			t.Fatal(err)
		}
		srv.StartTLS()
		defer srv.Close()
		port := srv.Listener.Addr().(*net.TCPAddr).Port
		code := fmt.Sprintf(`load("http", "get"); print(get("https://example.com:%d").status_code)`, port)
		options := []string{"--add-host", "example.com:host-gateway"}
		out, err := run(options, []string{"-c", code})
		if err == nil || !strings.Contains(out, "certificate signed by unknown authority") {
			t.Fatalf("untrusted certificate: output=%q error=%v", out, err)
		}
		caFile := filepath.Join(t.TempDir(), "ca-certificates.crt")
		bundle += string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw}))
		if err := os.WriteFile(caFile, []byte(bundle), 0o644); err != nil {
			t.Fatal(err)
		}
		options = append(options, "--mount", "type=bind,src="+caFile+",dst=/etc/ssl/certs/ca-certificates.crt,readonly")
		out, err = run(options, []string{"-c", code})
		if err != nil || out != "200\n" {
			t.Fatalf("trusted certificate: output=%q error=%v", out, err)
		}
	})
	t.Run("HTTP and graceful stop", func(t *testing.T) {
		args := []string{"run", "-d", "--platform", "linux/amd64", "--read-only", "--cap-drop=ALL",
			"--security-opt=no-new-privileges", "--memory=256m", "--cpus=1", "--pids-limit=64",
			"-p", "127.0.0.1::8080", image, "--caps", "safe", "--web-host", "0.0.0.0", "--web", "8080",
			"-c", `response.set_text("container ready")`}
		id, err := docker(args...)
		if err != nil {
			t.Fatalf("start: %v: %s", err, id)
		}
		id = strings.TrimSpace(id)
		t.Cleanup(func() { _, _ = docker("rm", "-f", id) })
		port, err := docker("port", id, "8080/tcp")
		if err != nil {
			t.Fatalf("published port: %v: %s", err, port)
		}
		client := &http.Client{Timeout: time.Second}
		ready := false
		for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
			resp, err := client.Get("http://" + strings.TrimSpace(port))
			if err == nil {
				body, readErr := io.ReadAll(resp.Body)
				_ = resp.Body.Close()
				ready = readErr == nil && resp.StatusCode == http.StatusOK && string(body) == "container ready"
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		if !ready {
			logs, _ := docker("logs", id)
			t.Fatalf("HTTP endpoint did not become ready: %s", logs)
		}
		if out, err := docker("stop", "--time", "5", id); err != nil {
			t.Fatalf("stop: %v: %s", err, out)
		}
		if out, err := docker("inspect", "--format", "{{.State.ExitCode}}", id); err != nil || strings.TrimSpace(out) != "0" {
			t.Fatalf("graceful exit: %v: %s", err, out)
		}
	})
}
