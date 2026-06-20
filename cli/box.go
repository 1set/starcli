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
	maxSteps       uint64 // per-run Starlark step budget; 0 = unlimited
	maxOutput      uint   // per-run top-level output entry cap; 0 = unlimited
	caps           string // capability tier: safe (default) / network / full
	allowNet       bool   // widen the grant with network modules
	allowFS        bool   // widen the grant with filesystem modules
	allowCmd       bool   // allow the cmd (host command execution) module
}

// BuildBox creates a new Starbox with the given options.
func BuildBox(opts *BoxOpts) (*starbox.Starbox, error) {
	// create a new Starbox instance behind a default-deny capability load gate:
	// only the modules the active capability flags permit may be loaded (the
	// default Safe tier allows pure/log/process modules, never net/fs/cmd).
	grant := grantFromFlags(opts.caps, opts.allowNet, opts.allowFS, opts.allowCmd)
	policy := starbox.Policy{Modules: starbox.ModuleAllow{Names: grant.allowedModules(getDefaultModules())}}
	box := starbox.NewWithPolicy(opts.name, policy)
	if ystring.IsNotBlank(opts.includePath) {
		box.SetFS(os.DirFS(opts.includePath))
	}

	// execution budgets (0 == unlimited): a step budget bounds runaway loops
	// that a wall-clock timeout cannot stop; an output cap bounds result size.
	box.SetMaxExecutionSteps(opts.maxSteps)
	box.SetMaxOutputEntries(opts.maxOutput)

	// machine-level knobs
	mac := box.GetMachine()
	mac.SetOutputConversionEnabled(false)
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
