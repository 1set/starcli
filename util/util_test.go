package util

// Characterization tests for the output/emoji/color helpers.
//
// Sections:
//   - PrintError (plain error vs starlark eval backtrace)
//   - StringEmoji (empty sentinel + determinism)
//   - ParseColor (name/alias/rgb/hex6/hex3 + error cases)

import (
	"bytes"
	"errors"
	"image/color"
	"io"
	"os"
	"testing"

	"go.starlark.net/starlark"
)

func captureStderr(t *testing.T, f func()) string {
	t.Helper()
	orig := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	c := make(chan string)
	go func() { var b bytes.Buffer; _, _ = io.Copy(&b, r); c <- b.String() }()
	f()
	_ = w.Close()
	os.Stderr = orig
	return <-c
}

func TestPrintError_PlainError(t *testing.T) {
	se := captureStderr(t, func() { PrintError(errors.New("boom")) })
	if se != "boom\n" {
		t.Errorf("plain error: stderr=%q want %q", se, "boom\n")
	}
}

func TestPrintError_StarlarkEvalBacktrace(t *testing.T) {
	thread := &starlark.Thread{Name: "test"}
	_, err := starlark.ExecFile(thread, "t.star", "x = 1 // 0", nil)
	if _, ok := err.(*starlark.EvalError); !ok {
		t.Fatalf("setup: expected *starlark.EvalError, got %T (%v)", err, err)
	}
	se := captureStderr(t, func() { PrintError(err) })
	if !bytes.Contains([]byte(se), []byte("floored division by zero")) {
		t.Errorf("eval error: stderr=%q missing the message", se)
	}
	if !bytes.Contains([]byte(se), []byte("Traceback")) {
		t.Errorf("eval error: stderr=%q missing the backtrace", se)
	}
}

func TestStringEmoji(t *testing.T) {
	if got := StringEmoji(""); got != "⭐" {
		t.Errorf("StringEmoji(\"\")=%q want star", got)
	}
	// Deterministic: same input -> same emoji.
	if a, b := StringEmoji("hello"), StringEmoji("hello"); a != b {
		t.Errorf("StringEmoji not deterministic: %q vs %q", a, b)
	}
	// Non-empty input maps to a single-rune emoji.
	got := StringEmoji("anything")
	if n := len([]rune(got)); n != 1 {
		t.Errorf("StringEmoji(\"anything\")=%q has %d runes, want 1", got, n)
	}
}

func TestParseColor(t *testing.T) {
	rgba := func(r, g, b uint8) color.RGBA { return color.RGBA{R: r, G: g, B: b, A: 0xFF} }
	cases := []struct {
		query string
		want  color.RGBA
	}{
		{"blue", rgba(0x00, 0x00, 0xFF)},
		{"AQUA", rgba(0x00, 0xFF, 0xFF)}, // alias -> cyan, case-insensitive
		{"rgb(255, 0, 0)", rgba(0xFF, 0x00, 0x00)},
		{"#FF0000", rgba(0xFF, 0x00, 0x00)}, // hex6
		{"#0f0", rgba(0x00, 0xFF, 0x00)},    // hex3 expands
	}
	for _, c := range cases {
		got, err := ParseColor(c.query)
		if err != nil {
			t.Errorf("ParseColor(%q): unexpected error %v", c.query, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseColor(%q)=%v want %v", c.query, got, c.want)
		}
	}

	t.Run("blank", func(t *testing.T) {
		if _, err := ParseColor("   "); err == nil {
			t.Errorf("blank query: expected error")
		}
	})
	t.Run("no-match", func(t *testing.T) {
		if _, err := ParseColor("not-a-color"); err == nil {
			t.Errorf("unknown color: expected error")
		}
	})
}
