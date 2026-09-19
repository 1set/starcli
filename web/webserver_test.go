package web

// Behavior tests for the web-server request handler, exercised through httptest
// (no port binding). Each request builds a fresh Starbox whose script is given
// the injected `request` / `response` globals.
//
// Sections:
//   - the script's response (status + body) is written back
//   - a runtime error becomes a 500 carrying the error text
//   - request fields (method, body, …) reach the script
//   - body/header limits, admission, deadlines, cancellation and lifecycle

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/1set/starbox"
)

// builderFor returns a per-request builder whose Starbox runs the given script.
// handler() injects `request` and `response` as globals before executing it.
func builderFor(script string) func() *starbox.RunnerConfig {
	return func() *starbox.RunnerConfig {
		return starbox.New("webtest").CreateRunConfig().Script(script)
	}
}

func TestHandler_WritesScriptResponse(t *testing.T) {
	h := handler(builderFor(`
response.set_status(201)
response.set_text("hello " + request.method)
`))
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	res := rec.Result()
	defer res.Body.Close()
	if res.StatusCode != 201 {
		t.Errorf("status = %d, want 201", res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	if got := string(body); got != "hello GET" {
		t.Errorf("body = %q, want %q", got, "hello GET")
	}
}

func TestHandler_RuntimeErrorIs500(t *testing.T) {
	h := handler(builderFor(`fail("boom")`))
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	res := rec.Result()
	defer res.Body.Close()
	if res.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	if s := string(body); !strings.Contains(s, "Runtime Error") || !strings.Contains(s, "boom") {
		t.Errorf("body = %q, want it to report the runtime error and 'boom'", s)
	}
}

func TestHandler_InjectsRequestFields(t *testing.T) {
	h := handler(builderFor(`response.set_text(request.method + " " + request.body)`))
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader("payload")))

	res := rec.Result()
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Errorf("status = %d, want 200", res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	if got := string(body); got != "POST payload" {
		t.Errorf("body = %q, want %q (request method+body should reach the script)", got, "POST payload")
	}
}

func TestHandlerRejectsOversizedAndUnreadableBody(t *testing.T) {
	calls := 0
	build := func() *starbox.RunnerConfig { calls++; return builderFor(`response.set_text("ok")`)() }
	for _, tc := range []struct {
		name   string
		body   io.Reader
		status int
	}{
		{"oversized", strings.NewReader(strings.Repeat("x", 1048577)), http.StatusRequestEntityTooLarge},
		{"read error", failingReader{}, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler(build)(rec, httptest.NewRequest(http.MethodPost, "/", tc.body))
			if rec.Code != tc.status {
				t.Fatalf("status=%d want %d", rec.Code, tc.status)
			}
		})
	}
	if calls != 0 {
		t.Fatalf("invalid input invoked script %d times", calls)
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("broken input") }

func TestHandlerPropagatesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler(builderFor(`response.set_text("should not run")`))(rec, r)
	if rec.Code != http.StatusRequestTimeout {
		t.Fatalf("status=%d, want 408", rec.Code)
	}
}

func TestHandlerBodyBoundaryAndMissingBuilder(t *testing.T) {
	cfg := DefaultConfig(0)
	cfg.MaxBodyBytes = 4
	for _, size := range []int{0, 4, 5} {
		for _, chunked := range []bool{false, true} {
			r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("x", size)))
			if chunked {
				r.ContentLength = -1
				r.TransferEncoding = []string{"chunked"}
			}
			rec := httptest.NewRecorder()
			requestHandler(cfg, builderFor(`response.set_text(request.body)`))(rec, r)
			want := http.StatusOK
			if size > 4 {
				want = http.StatusRequestEntityTooLarge
			}
			if rec.Code != want {
				t.Fatalf("size=%d chunked=%v: %d, want %d", size, chunked, rec.Code, want)
			}
		}
	}
	for _, builder := range []func() *starbox.RunnerConfig{nil, func() *starbox.RunnerConfig { return nil }} {
		rec := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Body = nil
		requestHandler(cfg, builder)(rec, r)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("nil builder: %d", rec.Code)
		}
	}
}

func TestHandlerDeadlineAndAdmission(t *testing.T) {
	cfg := DefaultConfig(0)
	cfg.RequestTimeout = 20 * time.Millisecond
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		requestHandler(cfg, builderFor("for i in range(1000000000):\n    pass"))(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("script did not stop at deadline")
	}
	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("deadline status=%d", rec.Code)
	}

	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	build := func() *starbox.RunnerConfig {
		once.Do(func() { close(entered); <-release })
		return builderFor(`response.set_text("ok")`)()
	}
	cfg = DefaultConfig(0)
	cfg.MaxConcurrent = 1
	h := requestHandler(cfg, build)
	done = make(chan struct{})
	go func() { defer close(done); h(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil)) }()
	<-entered
	rec = httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("busy status=%d", rec.Code)
	}
	close(release)
	<-done
	rec = httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("slot not released: %d", rec.Code)
	}
}

func TestServerLifecycleAndLimits(t *testing.T) {
	cfg := DefaultConfig(0)
	if cfg.Address != "127.0.0.1:0" {
		t.Fatalf("default bind: %q", cfg.Address)
	}
	server := newServer(context.Background(), cfg, builderFor(`response.set_text("ok")`))
	if server.ReadTimeout <= 0 || server.ReadHeaderTimeout <= 0 || server.WriteTimeout <= cfg.RequestTimeout || server.IdleTimeout <= 0 || server.MaxHeaderBytes != 16<<10 {
		t.Fatal("missing transport limits")
	}
	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Address = listener.Addr().String()
	if err := Start(uint16(listener.Addr().(*net.TCPAddr).Port), builderFor("pass")); err == nil {
		t.Error("legacy Start accepted an occupied listener")
	}
	// Starting on an occupied address must return to the host, never exit it.
	if err := StartContext(context.Background(), cfg, builderFor("pass")); err == nil {
		t.Error("occupied address accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- serve(ctx, listener, cfg, builderFor(`response.set_text("ok")`)) }()
	client := &http.Client{Timeout: 2 * time.Second}
	defer client.CloseIdleConnections()
	resp, err := client.Get("http://" + cfg.Address)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if string(data) != "ok" {
		t.Fatalf("body=%q", data)
	}
	// Exercise the wire header limit (net/http reserves an extra read buffer).
	conn, err := net.Dial("tcp", cfg.Address)
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	_, _ = fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: localhost\r\nX-Large: %s\r\n\r\n", strings.Repeat("x", 24<<10))
	response, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusRequestHeaderFieldsTooLarge {
		t.Errorf("header status=%d", response.StatusCode)
	}
	_ = response.Body.Close()
	_ = conn.Close()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(7 * time.Second):
		t.Fatal("shutdown did not finish")
	}
	if conn, err := net.DialTimeout("tcp", cfg.Address, time.Second); err == nil {
		_ = conn.Close()
		t.Fatal("listener survived shutdown")
	}
}

func TestServerInvalidConfiguration(t *testing.T) {
	for _, mutate := range []func(*Config){
		func(c *Config) { c.Address = "" }, func(c *Config) { c.MaxBodyBytes = 0 },
		func(c *Config) { c.MaxConcurrent = 0 }, func(c *Config) { c.RequestTimeout = 0 },
	} {
		cfg := DefaultConfig(0)
		mutate(&cfg)
		if err := StartContext(context.Background(), cfg, builderFor("pass")); err == nil {
			t.Fatal("invalid config accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := StartContext(ctx, DefaultConfig(0), builderFor("pass")); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled start: %v", err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_ = listener.Close()
	if err := serve(context.Background(), listener, DefaultConfig(0), builderFor("pass")); err == nil {
		t.Fatal("closed listener accepted")
	}
}
