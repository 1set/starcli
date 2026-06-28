// Package kit is the reusable embed→wire→run core of starcli: it turns an
// embedded Starlark script plus a chosen set of modules and runtime knobs into a
// ready-to-run starbox runtime.
//
// It is the seam that keeps the whole project consistent. The standard turnkey
// starcli is one instance of kit wired with every starpkg module; a build-your-
// own shell is another instance wired with just the few modules it needs. Both
// go through the same construction path, so they behave identically.
//
// kit depends only on starbox (the pure host runtime) and starlet — never on any
// starpkg module — so a build-your-own shell pulls in only the modules it
// actually imports and wires itself.
//
//	//go:embed app.star
//	var app string
//
//	func main() {
//	    if _, err := kit.Run(app, kit.WithModules("json", "math")); err != nil {
//	        log.Fatal(err)
//	    }
//	}
package kit

import (
	"io/fs"

	"github.com/1set/starbox"
	"github.com/1set/starlet"
	"go.uber.org/zap"
)

// DefaultName is the box name used by the package-level Run / RunFS helpers when
// no explicit name is given.
const DefaultName = "app"

// Kit holds the configuration for a starbox runtime. Build one with New and a set
// of Option values, then call Box, Run, or RunFile. The zero value is not usable;
// always go through New.
type Kit struct {
	name      string
	moduleSet starbox.ModuleSetName

	// module wiring (see the resolution order note on Box).
	moduleNames   []string
	loaders       map[string]starlet.ModuleLoader
	dynamicLoader starbox.DynamicModuleLoader

	policy    *starbox.Policy
	fsys      fs.FS
	logger    *zap.SugaredLogger
	printFunc starlet.PrintFunc
	globals   starlet.StringAnyMap

	globalReassign bool
	recursion      bool
	outputConv     bool
	maxSteps       uint64
	maxOutput      uint
}

// Option configures a Kit.
type Option func(*Kit)

// New returns a Kit for a box with the given name, applying the options. The
// defaults match a clean, predictable runtime: the empty module set (nothing is
// loaded unless asked via WithModules / WithLoader / WithDynamicLoader), output
// conversion on (Run results come back as native Go values), and global
// reassignment, recursion, and execution budgets all off/unlimited.
func New(name string, opts ...Option) *Kit {
	k := &Kit{
		name:       name,
		moduleSet:  starbox.EmptyModuleSet,
		loaders:    map[string]starlet.ModuleLoader{},
		outputConv: true,
	}
	for _, opt := range opts {
		opt(k)
	}
	return k
}

// WithModules names modules to load. A name that is a starlet builtin (e.g.
// "json", "math") resolves on its own; any other name must be backed by a loader
// registered via WithLoader or WithDynamicLoader.
func WithModules(names ...string) Option {
	return func(k *Kit) { k.moduleNames = append(k.moduleNames, names...) }
}

// WithLoader registers an explicit loader for a module name — the way a build-
// your-own shell wires a specific starpkg module it imports, e.g.
// WithLoader(qrcode.ModuleName, qrcode.NewModule().LoadModule()). A registered
// loader is always loaded; it does not also need to appear in WithModules.
func WithLoader(name string, loader starlet.ModuleLoader) Option {
	return func(k *Kit) { k.loaders[name] = loader }
}

// WithDynamicLoader registers a single loader that resolves any requested module
// name on demand — the way the standard starcli exposes its whole module
// registry. Names listed in WithModules that are neither builtins nor explicit
// loaders are resolved through it.
func WithDynamicLoader(loader starbox.DynamicModuleLoader) Option {
	return func(k *Kit) { k.dynamicLoader = loader }
}

// WithModuleSet selects a predefined starbox module set (EmptyModuleSet by
// default). WithModules still adds individual modules on top of the set.
func WithModuleSet(set starbox.ModuleSetName) Option {
	return func(k *Kit) { k.moduleSet = set }
}

// WithPolicy installs a starbox load-gate policy (the host-only capability
// allowlist). Without it, every wired module is loadable.
func WithPolicy(p starbox.Policy) Option {
	return func(k *Kit) { k.policy = &p }
}

// WithFS sets the filesystem the script's load() statements and RunFile read
// from — typically an embed.FS of .star files.
func WithFS(fsys fs.FS) Option {
	return func(k *Kit) { k.fsys = fsys }
}

// WithGlobals injects host values as script globals.
func WithGlobals(g starlet.StringAnyMap) Option {
	return func(k *Kit) {
		if k.globals == nil {
			k.globals = starlet.StringAnyMap{}
		}
		for key, val := range g {
			k.globals[key] = val
		}
	}
}

// WithGlobal injects a single host value as a script global.
func WithGlobal(key string, value interface{}) Option {
	return func(k *Kit) {
		if k.globals == nil {
			k.globals = starlet.StringAnyMap{}
		}
		k.globals[key] = value
	}
}

// WithLogger routes the script's log module to the given zap logger.
func WithLogger(l *zap.SugaredLogger) Option {
	return func(k *Kit) { k.logger = l }
}

// WithPrintFunc overrides how the script's print() output is rendered. Without
// it, starbox's default (stdout) is used.
func WithPrintFunc(pf starlet.PrintFunc) Option {
	return func(k *Kit) { k.printFunc = pf }
}

// WithGlobalReassign toggles top-level global reassignment in the script dialect.
func WithGlobalReassign(on bool) Option {
	return func(k *Kit) { k.globalReassign = on }
}

// WithRecursion toggles recursion support in the script dialect.
func WithRecursion(on bool) Option {
	return func(k *Kit) { k.recursion = on }
}

// WithOutputConversion toggles whether Run results are converted to native Go
// values (on by default). Turn it off to keep raw Starlark values.
func WithOutputConversion(on bool) Option {
	return func(k *Kit) { k.outputConv = on }
}

// WithMaxSteps bounds a single run to n Starlark execution steps (0 = unlimited),
// a guard against runaway loops a wall-clock timeout cannot stop.
func WithMaxSteps(n uint64) Option {
	return func(k *Kit) { k.maxSteps = n }
}

// WithMaxOutputEntries caps the number of top-level result entries a run may
// produce (0 = unlimited).
func WithMaxOutputEntries(n uint) Option {
	return func(k *Kit) { k.maxOutput = n }
}

// Box constructs the configured *starbox.Starbox. Module names resolve in
// starbox's order — starlet builtins first, then explicit WithLoader entries,
// then the WithDynamicLoader fallback — so the same name list works whether a
// shell wires modules one by one or through a registry.
func (k *Kit) Box() (*starbox.Starbox, error) {
	var box *starbox.Starbox
	if k.policy != nil {
		box = starbox.NewWithPolicy(k.name, *k.policy)
	} else {
		box = starbox.New(k.name)
	}

	if k.logger != nil {
		box.SetLogger(k.logger)
	}
	if k.fsys != nil {
		box.SetFS(k.fsys)
	}
	box.SetMaxExecutionSteps(k.maxSteps)
	box.SetMaxOutputEntries(k.maxOutput)

	mac := box.GetMachine()
	mac.SetOutputConversionEnabled(k.outputConv)
	if k.globalReassign {
		mac.EnableGlobalReassign()
	} else {
		mac.DisableGlobalReassign()
	}
	if k.recursion {
		mac.EnableRecursionSupport()
	} else {
		mac.DisableRecursionSupport()
	}

	if k.printFunc != nil {
		box.SetPrintFunc(k.printFunc)
	}

	// Start from a clean module set, then wire exactly what was requested.
	box.SetModuleSet(k.moduleSet)
	if k.dynamicLoader != nil {
		box.SetDynamicModuleLoader(k.dynamicLoader)
	}
	for name, loader := range k.loaders {
		box.AddModuleLoader(name, loader)
	}
	if len(k.globals) > 0 {
		box.AddKeyValues(k.globals)
	}
	if len(k.moduleNames) > 0 {
		box.AddModulesByName(k.moduleNames...)
	}
	return box, nil
}

// Run constructs the box and executes the given script source.
func (k *Kit) Run(script string) (starlet.StringAnyMap, error) {
	box, err := k.Box()
	if err != nil {
		return nil, err
	}
	return box.Run(script)
}

// RunFile constructs the box and executes a script file read from the configured
// filesystem (see WithFS).
func (k *Kit) RunFile(file string) (starlet.StringAnyMap, error) {
	box, err := k.Box()
	if err != nil {
		return nil, err
	}
	return box.RunFile(file)
}

// Run is the package-level one-liner for a build-your-own shell: it wires a
// default-named box from the options and runs the embedded script source.
func Run(script string, opts ...Option) (starlet.StringAnyMap, error) {
	return New(DefaultName, opts...).Run(script)
}

// RunFS is the package-level one-liner that runs entry out of fsys (typically an
// embed.FS of .star files), so a shell can ship a whole tree of scripts.
func RunFS(fsys fs.FS, entry string, opts ...Option) (starlet.StringAnyMap, error) {
	return New(DefaultName, append([]Option{WithFS(fsys)}, opts...)...).RunFile(entry)
}
