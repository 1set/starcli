package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/1set/starcli/util"
	"github.com/1set/starlet"
	"go.starlark.net/starlark"
	"go.uber.org/atomic"
)

// getPrinterFunc returns a function to print output based on the given printer name.
func getPrinterFunc(sc scenarioCode, printer string) (starlet.PrintFunc, error) {
	// normalize printer name
	pn := strings.ToLower(strings.TrimSpace(printer))
	if pn == "auto" {
		switch sc {
		case scenarioREPL:
			pn = "stdout"
		case scenarioDirect:
			pn = "stdout"
		case scenarioFile:
			pn = "lineno"
		case scenarioWeb:
			pn = "basic"
		}
	}
	// switch based on name
	switch pn {
	case "none", "nil", "no":
		return func(thread *starlark.Thread, msg string) {}, nil
	case "stdout":
		return func(thread *starlark.Thread, msg string) {
			fmt.Println(msg)
		}, nil
	case "stderr":
		return func(thread *starlark.Thread, msg string) {
			fmt.Fprintln(os.Stderr, msg)
		}, nil
	case "basic":
		// nil means using the default print function provided by Starbox
		return nil, nil
	case "lineno", "linenum":
		cnt := atomic.NewInt64(0)
		return func(thread *starlark.Thread, msg string) {
			prefix := fmt.Sprintf("[%04d](%s)%s", cnt.Inc(), time.Now().UTC().Format(`15:04:05.000`), util.StringEmoji(msg))
			fmt.Fprintln(os.Stderr, prefix, msg)
		}, nil
	case "since":
		cnt := atomic.NewInt64(0)
		now := time.Now()
		return func(thread *starlark.Thread, msg string) {
			prefix := fmt.Sprintf("[%04d](%.03f)%s", cnt.Inc(), time.Since(now).Seconds(), util.StringEmoji(msg))
			fmt.Fprintln(os.Stderr, prefix, msg)
		}, nil
	default:
		return nil, fmt.Errorf("unknown printer name: %s", printer)
	}
}
