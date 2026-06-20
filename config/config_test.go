package config

// Characterization tests for configuration loading and build-info display.
//
// Sections:
//   - InitConfig: search mode, explicit-file error, env override
//   - SetDefaults & getters
//   - DisplayBuildInfo (smoke)

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestInitConfig_NoFileIsNotAnError(t *testing.T) {
	viper.Reset()
	if err := InitConfig(""); err != nil {
		t.Fatalf("InitConfig(\"\"): unexpected error %v", err)
	}
	// SetDefaults seeds host_name from os.Hostname.
	if GetHostname() == "" {
		t.Errorf("GetHostname() empty after InitConfig with defaults")
	}
}

func TestInitConfig_ExplicitMissingFileErrors(t *testing.T) {
	viper.Reset()
	missing := filepath.Join(t.TempDir(), "nope.yaml")
	if err := InitConfig(missing); err == nil {
		t.Errorf("InitConfig(%q): expected an error for a missing explicit file", missing)
	}
}

func TestInitConfig_EnvOverride(t *testing.T) {
	viper.Reset()
	t.Setenv("STAR_HOST_NAME", "Aloha")
	if err := InitConfig(""); err != nil {
		t.Fatalf("InitConfig: %v", err)
	}
	if got := GetHostname(); got != "Aloha" {
		t.Errorf("env override: GetHostname()=%q want %q", got, "Aloha")
	}
}

func TestSetDefaultsAndGetters(t *testing.T) {
	viper.Reset()
	SetDefaults()

	host, _ := os.Hostname()
	if got := GetHostname(); got != host {
		t.Errorf("GetHostname()=%q want %q", got, host)
	}
	// The optional credential getters default to empty.
	for name, got := range map[string]string{
		"resend_api_key":  GetResendAPIKey(),
		"sender_domain":   GetSenderDomain(),
		"openai_provider": GetOpenAIProvider(),
		"openai_endpoint": GetOpenAIEndpoint(),
		"openai_key":      GetOpenAIKey(),
		"openai_gpt":      GetOpenAIGPTModel(),
		"openai_dalle":    GetOpenAIDallEModel(),
		"openai_apiver":   GetOpenAIAPIVersion(),
	} {
		if got != "" {
			t.Errorf("%s default=%q want empty", name, got)
		}
	}
}

func TestDisplayBuildInfo_Smoke(t *testing.T) {
	// Build vars are empty in tests; DisplayBuildInfo should still render the
	// logo and return without panicking.
	origOut := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	outC := make(chan string)
	go func() { var b bytes.Buffer; _, _ = io.Copy(&b, r); outC <- b.String() }()

	DisplayBuildInfo()

	_ = w.Close()
	os.Stdout = origOut
	if out := <-outC; out == "" {
		t.Errorf("DisplayBuildInfo produced no output")
	}
}
