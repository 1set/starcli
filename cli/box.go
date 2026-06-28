package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/1set/starbox"
	"github.com/1set/starcli/kit"
	"github.com/1set/starlet"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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
	logFile        string // if set, the script's log module writes here
	logFormat      string // log file encoding: "console" (default) or "json"
	maxSteps       uint64 // per-run Starlark step budget; 0 = unlimited
	maxOutput      uint   // per-run top-level output entry cap; 0 = unlimited
	caps           string // capability tier: safe (default) / network / full
	allowNet       bool   // widen the grant with network modules
	allowFS        bool   // widen the grant with filesystem modules
	allowCmd       bool   // allow the cmd (host command execution) module
	dangerous      bool   // --dangerously-allow-all: open everything + run any command
	execCmd        bool   // derived from the grant: construct cmd ENABLED (allow-all)
}

// BuildBox creates a new Starbox with the given options. It is the standard,
// fully-loaded instance of the shared kit core (kit.Box): the CLI-specific
// concerns — capability gating, scenario-driven printer, file logging — are
// resolved here and handed to kit as options, so the turnkey CLI and any
// build-your-own shell construct their runtime through exactly the same path.
//
// By default every wired module is available (the open posture); a restrictive
// --caps tier / STAR_CAPS or an --allow-* flag installs a capability load gate so
// only the permitted modules may be loaded.
func BuildBox(opts *BoxOpts) (*starbox.Starbox, error) {
	if !validCapsTier(opts.caps) {
		return nil, fmt.Errorf("unknown --caps value %q (want: open, full, network, or safe)", opts.caps)
	}
	grant := grantFromFlags(opts.caps, opts.allowNet, opts.allowFS, opts.allowCmd, opts.dangerous)
	// The cmd loader (loadCLIModuleByName) constructs an enabled allow-all module
	// only when the grant permits execution; otherwise cmd loads disabled.
	opts.execCmd = grant.execCmd

	// set print function: TODO: for scenario, and throw errors
	pf, err := getPrinterFunc(opts.scenario, opts.printerName)
	if err != nil {
		return nil, err
	}

	kitOpts := []kit.Option{
		// execution budgets (0 == unlimited): a step budget bounds runaway loops
		// that a wall-clock timeout cannot stop; an output cap bounds result size.
		kit.WithMaxSteps(opts.maxSteps),
		kit.WithMaxOutputEntries(opts.maxOutput),
		// the CLI prints results itself, so keep raw Starlark values.
		kit.WithOutputConversion(false),
		kit.WithGlobalReassign(opts.globalReassign),
		kit.WithRecursion(opts.recursion),
		kit.WithPrintFunc(pf),
		// every starpkg module is resolved on demand from the CLI's registry.
		kit.WithDynamicLoader(func(name string) (starlet.ModuleLoader, error) {
			return loadCLIModuleByName(opts, name)
		}),
		kit.WithModules(opts.moduleToLoad...),
	}

	if !grant.unrestricted() {
		// A tier/flag narrowed the grant: gate loading to the permitted set.
		policy := starbox.Policy{Modules: starbox.ModuleAllow{Names: grant.allowedModules(getDefaultModules())}}
		kitOpts = append(kitOpts, kit.WithPolicy(policy))
	}

	// Route the script's `log` module output to a file when requested (C-4):
	// starbox uses the box logger for the log module, so a file-backed logger
	// captures every log.* call at the interpreter level.
	if opts.logFile != "" {
		lg, err := fileLogger(opts.logFile, opts.logFormat)
		if err != nil {
			return nil, err
		}
		kitOpts = append(kitOpts, kit.WithLogger(lg))
	}

	if strings.TrimSpace(opts.includePath) != "" {
		kitOpts = append(kitOpts, kit.WithFS(os.DirFS(opts.includePath)))
	}

	return kit.New(opts.name, kitOpts...).Box()
}

var (
	logFileMu      sync.Mutex
	logFileLoggers = map[string]*zap.SugaredLogger{}
	logFileHandles = map[string]*os.File{}
)

// fileLogger returns a zap logger that appends every level to path, memoized so
// repeated BuildBox calls (e.g. one per web request) share a single open file
// instead of leaking a descriptor each time. The parent directory is created if
// needed; writes are synchronous, so no explicit flush is required.
func fileLogger(path, format string) (*zap.SugaredLogger, error) {
	enc, err := logEncoder(format)
	if err != nil {
		return nil, err
	}
	logFileMu.Lock()
	defer logFileMu.Unlock()
	if lg, ok := logFileLoggers[path]; ok {
		return lg, nil
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("log file: %w", err)
		}
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("log file: %w", err)
	}
	core := zapcore.NewCore(enc, zapcore.AddSync(f), zapcore.DebugLevel)
	lg := zap.New(core).Sugar()
	logFileLoggers[path] = lg
	logFileHandles[path] = f
	return lg, nil
}

// logEncoder builds the zap encoder for the log file: human-readable "console"
// (the default) or machine-readable "json".
func logEncoder(format string) (zapcore.Encoder, error) {
	encCfg := zap.NewProductionEncoderConfig()
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	switch format {
	case "", "console":
		return zapcore.NewConsoleEncoder(encCfg), nil
	case "json":
		return zapcore.NewJSONEncoder(encCfg), nil
	}
	return nil, fmt.Errorf("unknown --log-format %q (want console or json)", format)
}

// closeLogFiles flushes and closes every memoized log file. The process holds
// them open for its lifetime (the OS closes them on exit), so this exists for
// tests, which must release the handle before the temp dir can be removed
// (notably on Windows, where an open file cannot be deleted).
func closeLogFiles() {
	logFileMu.Lock()
	defer logFileMu.Unlock()
	for path, f := range logFileHandles {
		_ = f.Sync()
		_ = f.Close()
		delete(logFileHandles, path)
		delete(logFileLoggers, path)
	}
}
