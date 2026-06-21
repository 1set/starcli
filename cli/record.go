package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// startRecording tees the process's stdout and stderr to a transcript file, so a
// whole session — print output, results, REPL interaction and errors — is saved
// for later review. It returns a stop function that restores the streams and
// closes the file; call it before the process exits. stdin is left untouched, so
// an interactive REPL keeps its terminal. The file is appended to, with a
// timestamped header delimiting each session.
func startRecording(path string) (stop func(), err error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("record: %w", err)
		}
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("record: %w", err)
	}
	fmt.Fprintf(f, "\n===== starcli session %s =====\n", time.Now().Format(time.RFC3339))

	lf := &lockedWriter{w: f} // serialize the two streams into one file
	origOut, origErr := os.Stdout, os.Stderr
	rOut, wOut, err := os.Pipe()
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("record: %w", err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		_ = wOut.Close()
		_ = f.Close()
		return nil, fmt.Errorf("record: %w", err)
	}
	os.Stdout, os.Stderr = wOut, wErr

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _, _ = io.Copy(io.MultiWriter(origOut, lf), rOut) }()
	go func() { defer wg.Done(); _, _ = io.Copy(io.MultiWriter(origErr, lf), rErr) }()

	return func() {
		os.Stdout, os.Stderr = origOut, origErr
		_ = wOut.Close()
		_ = wErr.Close()
		wg.Wait()
		_ = f.Close()
	}, nil
}

// lockedWriter serializes concurrent writes (the stdout and stderr copiers) to a
// single underlying writer so transcript lines are not interleaved mid-write.
type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}
