package cli

import (
	"fmt"
	"sort"

	"github.com/1set/starbox"
	"github.com/1set/starcli/config"
	"github.com/1set/starcli/module/args"
	"github.com/1set/starcli/module/sys"
	"github.com/1set/starlet"
	"github.com/samber/lo"
	"github.com/starpkg/cache"
	"github.com/starpkg/cmd"
	"github.com/starpkg/email"
	"github.com/starpkg/emoji"
	"github.com/starpkg/gum"
	"github.com/starpkg/liquid"
	"github.com/starpkg/llm"
	"github.com/starpkg/markdown"
	"github.com/starpkg/qrcode"
	"github.com/starpkg/sqlite"
	"github.com/starpkg/toml"
	"github.com/starpkg/totp"
	"github.com/starpkg/web"
	"github.com/starpkg/yaml"
)

var (
	starMods = starlet.GetAllBuiltinModuleNames()
	cliMods  = []string{
		args.ModuleName,
		sys.ModuleName,
		gum.ModuleName,
		email.ModuleName,
		llm.ModuleName,
		markdown.ModuleName,
		cmd.ModuleName,
		sqlite.ModuleName,
		web.ModuleName,
		// pure domain modules (no network / filesystem) — wired by default so
		// the turnkey CLI matches the ecosystem's pure starpkg surface.
		cache.ModuleName,
		emoji.ModuleName,
		liquid.ModuleName,
		qrcode.ModuleName,
		toml.ModuleName,
		totp.ModuleName,
		yaml.ModuleName,
	}
)

// getDefaultModules returns the default modules for CLI, including builtin modules from Starlet and local modules in CLI.
func getDefaultModules() []string {
	allMods := lo.Union(starMods, cliMods)
	sort.Strings(allMods)
	return allMods
}

// loadModules loads the given modules into the Starbox instance.
func loadModules(box *starbox.Starbox, opts *BoxOpts) error {
	usrMods := opts.moduleToLoad
	if len(usrMods) == 0 {
		// no modules to load
		log.Debugw("no modules to load", "user_modules", usrMods)
		return nil
	}

	// set dynamic module loader
	box.SetDynamicModuleLoader(func(name string) (starlet.ModuleLoader, error) {
		return loadCLIModuleByName(opts, name)
	})
	box.AddModulesByName(usrMods...)

	// all is well
	return nil
}

func loadCLIModuleByName(opts *BoxOpts, name string) (starlet.ModuleLoader, error) {
	switch name {
	case args.ModuleName:
		return args.NewModule(opts.cmdArgs), nil
	case sys.ModuleName:
		return sys.NewModule(opts.cmdArgs), nil
	case gum.ModuleName:
		return gum.NewModule().LoadModule(), nil
	case email.ModuleName:
		return email.NewModuleWithConfig(
			config.GetResendAPIKey(),
			config.GetSenderDomain(),
		).LoadModule(), nil
	case llm.ModuleName:
		return llm.NewModuleWithConfig(
			config.GetOpenAIProvider(),
			config.GetOpenAIEndpoint(),
			config.GetOpenAIKey(),
			config.GetOpenAIGPTModel(),
			config.GetOpenAIDallEModel(),
			config.GetOpenAIAPIVersion(),
		).LoadModule(), nil
	case markdown.ModuleName:
		return markdown.NewModule().LoadModule(), nil
	case cmd.ModuleName:
		// Command execution is off unless the operator opted in (--allow-cmd or
		// --dangerously-allow-all → execCmd). Enabled means allow-all: any command
		// runs (still argv-only + input-hardened by the cmd module). Otherwise the
		// module loads disabled — which() works, run() returns a clear error.
		if opts.execCmd {
			return cmd.NewModuleWithAllowAll().LoadModule(), nil
		}
		return cmd.NewModule().LoadModule(), nil
	case sqlite.ModuleName:
		return sqlite.NewModule().LoadModule(), nil
	case web.ModuleName:
		return web.NewModule().LoadModule(), nil
	case cache.ModuleName:
		return cache.NewModule().LoadModule(), nil
	case emoji.ModuleName:
		return emoji.NewModule().LoadModule(), nil
	case liquid.ModuleName:
		return liquid.NewModule().LoadModule(), nil
	case qrcode.ModuleName:
		return qrcode.NewModule().LoadModule(), nil
	case toml.ModuleName:
		return toml.NewModule().LoadModule(), nil
	case totp.ModuleName:
		return totp.NewModule().LoadModule(), nil
	case yaml.ModuleName:
		return yaml.NewModule().LoadModule(), nil
	// Add more modules as needed
	default:
		return nil, fmt.Errorf("unknown module: %s", name)
	}
}
