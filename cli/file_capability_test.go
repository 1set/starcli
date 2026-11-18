package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"go.starlark.net/starlark"
)

type fileGrantTransport func(*http.Request) (*http.Response, error)

func (f fileGrantTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// The same local-file inputs must be rejected before execution without a file
// grant, and actually read and transmitted to a fake provider with that grant.
// Testing the provider payload also guards against a false pass caused by an
// unrelated credential, argument, or image-decoding error.
func TestFileReadingNetworkModulesRequireFileGrant(t *testing.T) {
	dir := t.TempDir()
	var imageBytes bytes.Buffer
	if err := png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "private-image.png")
	if err := os.WriteFile(path, imageBytes.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	wantPayload := base64.StdEncoding.EncodeToString(imageBytes.Bytes())

	for _, module := range []string{"email", "llm"} {
		t.Run(module, func(t *testing.T) {
			for _, grant := range []struct {
				name, tier       string
				allowNet, fs     bool
				include          string
				allowed, preload bool
			}{
				{name: "network only", tier: "network"},
				{name: "network-only preloaded global", tier: "network", preload: true},
				{name: "safe and allow-net", tier: "safe", allowNet: true},
				{name: "include is not a file grant", tier: "network", include: dir},
				{name: "network and allow-fs", tier: "network", fs: true, allowed: true},
				{name: "net-fs preloaded global", tier: "network", fs: true, allowed: true, preload: true},
				{name: "default open", allowed: true},
				{name: "full", tier: "full", allowed: true},
			} {
				t.Run(grant.name, func(t *testing.T) {
					var requests atomic.Int32
					var transmitted atomic.Bool
					capture := func(r *http.Request) {
						requests.Add(1)
						body, err := io.ReadAll(r.Body)
						if err == nil {
							if module == "llm" {
								transmitted.Store(strings.Contains(string(body), wantPayload))
							} else {
								var payload struct {
									Attachments []struct {
										Content []byte `json:"content"`
									} `json:"attachments"`
								}
								if json.Unmarshal(body, &payload) == nil && len(payload.Attachments) == 1 {
									transmitted.Store(bytes.Equal(payload.Attachments[0].Content, imageBytes.Bytes()))
								}
							}
						}
					}
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						capture(r)
						w.Header().Set("Content-Type", "application/json")
						fmt.Fprint(w, `{"id":"fixture","object":"chat.completion","model":"fixture","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`)
					}))
					defer server.Close()
					client := http.DefaultClient
					http.DefaultClient = &http.Client{Transport: fileGrantTransport(func(r *http.Request) (*http.Response, error) {
						capture(r)
						return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"id":"fixture"}`)), Request: r}, nil
					})}
					defer func() { http.DefaultClient = client }()

					code := fmt.Sprintf(`
load("email", "send", "set_resend_api_key")
set_resend_api_key("fixture-key")
observed = send(subject="fixture", text="fixture", to="to@example.invalid", sender="from@example.invalid", attachment_file=%q).success
`, path)
					if module == "llm" {
						code = fmt.Sprintf(`
load("llm", "chat", "set_openai_provider", "set_openai_endpoint_url", "set_openai_api_key")
set_openai_provider("openai")
set_openai_endpoint_url(%q)
set_openai_api_key("fixture-key")
observed = chat(text="fixture", image_file=%q, model="fixture", retry=1)
`, server.URL+"/v1", path)
					}
					if grant.preload {
						parts := strings.SplitN(strings.TrimSpace(code), "\n", 2)
						code = parts[1]
						functions := []string{"send", "set_resend_api_key"}
						if module == "llm" {
							functions = []string{"chat", "set_openai_provider", "set_openai_endpoint_url", "set_openai_api_key"}
						}
						for _, name := range functions {
							code = strings.ReplaceAll(code, name+"(", module+"."+name+"(")
						}
					}
					box, err := BuildBox(&BoxOpts{scenario: scenarioDirect, printerName: "none", moduleToLoad: getDefaultModules(), caps: grant.tier, allowNet: grant.allowNet, allowFS: grant.fs, includePath: grant.include})
					if err != nil {
						t.Fatal(err)
					}
					globals, err := box.Run(code)
					if !grant.allowed {
						if err == nil || requests.Load() != 0 || transmitted.Load() {
							t.Fatalf("without filesystem grant: err=%v requests=%d file transmitted=%v", err, requests.Load(), transmitted.Load())
						}
						if !strings.Contains(err.Error(), module) {
							t.Fatalf("expected module denial, got %v", err)
						}
						return
					}
					if err != nil || requests.Load() != 1 || !transmitted.Load() {
						t.Fatalf("with filesystem grant: err=%v requests=%d file transmitted=%v", err, requests.Load(), transmitted.Load())
					}
					var want starlark.Value = starlark.True
					if module == "llm" {
						want = starlark.String("ok")
					}
					if globals["observed"] != want {
						t.Fatalf("provider result = %v, want %v", globals["observed"], want)
					}
				})
			}
		})
	}
}
