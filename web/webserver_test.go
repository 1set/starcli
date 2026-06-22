package web

// Behavior tests for the web-server request handler, exercised through httptest
// (no port binding). Each request builds a fresh Starbox whose script is given
// the injected `request` / `response` globals.
//
// Sections:
//   - the script's response (status + body) is written back
//   - a runtime error becomes a 500 carrying the error text
//   - request fields (method, body, …) reach the script

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
