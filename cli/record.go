package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/chzyer/readline"
)

// startRecording tees the process's stdout and stderr to a transcript file, so a
// whole session — print output, results, REPL interaction and errors — is saved
// for later review. It returns a stop function that restores the streams and
// closes the file and reports write/close errors; call it before the process
// exits. stdin and readline's terminal writers are preserved, so
// an interactive REPL keeps its terminal. The file is appended to, with a
// timestamped header delimiting each session.
func startRecording(path string) (stop func() error, err error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("record: %w", err)
		}
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("record: %w", err)
	}
	return recordTo(f, os.Pipe)
}

func recordTo(f io.WriteCloser, pipe func() (*os.File, *os.File, error)) (func() error, error) {
	lf := &lockedWriter{w: f}
	fmt.Fprintf(lf, "\n===== starcli session %s =====\n", time.Now().Format(time.RFC3339))
	if lf.err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("record: %w", lf.err)
	}

	origOut, origErr := os.Stdout, os.Stderr
	rOut, wOut, err := pipe()
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("record: %w", err)
	}
	rErr, wErr, err := pipe()
	if err != nil {
		_ = rOut.Close()
		_ = wOut.Close()
		_ = f.Close()
		return nil, fmt.Errorf("record: %w", err)
	}
	os.Stdout, os.Stderr = wOut, wErr
	// readline captures its own writers at package initialization. Tee those
	// separately, preserving its platform-specific terminal/ANSI handling.
	origTermOut, origTermErr := readline.Stdout, readline.Stderr
	readline.Stdout = borrowedWriter{io.MultiWriter(origTermOut, lf)}
	readline.Stderr = borrowedWriter{io.MultiWriter(origTermErr, lf)}

	var wg sync.WaitGroup
	var copyErrors [2]error
	wg.Add(2)
	go func() {
		defer wg.Done()
		defer rOut.Close()
		_, copyErrors[0] = io.Copy(io.MultiWriter(origOut, lf), rOut)
	}()
	go func() {
		defer wg.Done()
		defer rErr.Close()
		_, copyErrors[1] = io.Copy(io.MultiWriter(origErr, lf), rErr)
	}()

	var once sync.Once
	var stopErr error
	return func() error {
		once.Do(func() {
			os.Stdout, os.Stderr = origOut, origErr
			readline.Stdout, readline.Stderr = origTermOut, origTermErr
			_ = wOut.Close()
			_ = wErr.Close()
			wg.Wait()
			lf.mu.Lock()
			err := errors.Join(lf.err, copyErrors[0], copyErrors[1], f.Close())
			lf.mu.Unlock()
			if err != nil {
				stopErr = fmt.Errorf("record: %w", err)
			}
		})
		return stopErr
	}, nil
}

func finishRecording(stop func() error, code int) int {
	if err := stop(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		if code == exitOK {
			return exitError
		}
	}
	return code
}

// borrowedWriter must not close the caller's terminal stream.
type borrowedWriter struct{ io.Writer }

func (borrowedWriter) Close() error { return nil }

// lockedWriter serializes transcript writes and retains the first failure.
// It keeps accepting bytes after a recording failure so the pipe readers keep
// draining and a full disk cannot deadlock the script. stop reports the error.
type lockedWriter struct {
	mu  sync.Mutex
	w   io.Writer
	err error
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.err == nil {
		var n int
		n, l.err = l.w.Write(p)
		if l.err == nil && n != len(p) {
			l.err = io.ErrShortWrite
		}
	}
	return len(p), nil
}
