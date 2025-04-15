package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/1set/gut/ystring"
	"github.com/1set/starbox"
	"github.com/1set/starlet"
	"github.com/PureMature/starcli/util"
	"go.starlark.net/starlark"
	"go.uber.org/atomic"
)

type scenarioCode uint

const (
	scenarioREPL scenarioCode = iota + 1
	scenarioDirect
	scenarioFile
	scenarioWeb
)

// BoxOpts defines the options for creating a new Starbox instance.
type BoxOpts struct {
	scenario       scenarioCode
	name           string
	includePath    string
	moduleToLoad   []string
	cmdArgs        []string
	printerName    string
	recursion      bool
	globalReassign bool
}

// BuildBox creates a new Starbox with the given options.
func BuildBox(opts *BoxOpts) (*starbox.Starbox, error) {
	// create a new Starbox instance
	box := starbox.New(opts.name)
	if ystring.IsNotBlank(opts.includePath) {
		box.SetFS(os.DirFS(opts.includePath))
	}
	box.SetLogger(log) // it's a HACK here, since log is a global variable
	box.GetMachine().SetOutputConversionEnabled(false)

	// set inspect condition
	mac := box.GetMachine()
	if opts.globalReassign {
		mac.EnableGlobalReassign()
	} else {
		mac.DisableGlobalReassign()
	}
	if opts.recursion {
		mac.EnableRecursionSupport()
	} else {
		mac.DisableRecursionSupport()
	}

	// set print function: TODO: for scenario, and throw errors
	pf, err := getPrinterFunc(opts.scenario, opts.printerName)
	if err != nil {
		return nil, err
	}
	box.SetPrintFunc(pf)

	// load modules
	box.SetModuleSet(starbox.EmptyModuleSet) // force clean the module set
	if err := loadModules(box, opts); err != nil {
		return nil, err
	}
	return box, nil
}

// genInspectCond creates a function for Starbox runner to inspect the result.
func genInspectCond(inspect bool) starbox.InspectCondFunc {
	if inspect {
		return func(m starlet.StringAnyMap, err error) bool {
			if err != nil {
				util.PrintError(err)
			}
			return true
		}
	}
	return func(starlet.StringAnyMap, error) bool {
		return false
	}
}

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
