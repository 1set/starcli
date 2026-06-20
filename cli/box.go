package cli

import (
	"os"

	"github.com/1set/gut/ystring"
	"github.com/1set/starbox"
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
