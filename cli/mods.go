package cli

import (
	"fmt"
	"sort"

	"github.com/1set/starbox"
	"github.com/1set/starlet"
	"github.com/PureMature/starcli/config"
	"github.com/PureMature/starcli/module/gum"
	"github.com/PureMature/starcli/module/sys"
	"github.com/PureMature/starport/charm/cacc"
	"github.com/PureMature/starport/charm/cfs"
	"github.com/PureMature/starport/charm/ckv"
	"github.com/PureMature/starport/email"
	"github.com/PureMature/starport/llm"
	"github.com/samber/lo"
)

var (
	starMods = starlet.GetAllBuiltinModuleNames()
	cliMods  = []string{
		sys.ModuleName,
		gum.ModuleName,
		email.ModuleName,
		llm.ModuleName,
		cacc.ModuleName,
		ckv.ModuleName,
		cfs.ModuleName,
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
	case sys.ModuleName:
		return sys.NewModule(opts.cmdArgs), nil
	case gum.ModuleName:
		return gum.NewModule(), nil
	case email.ModuleName:
		return email.NewModuleWithGetter(
			config.GetResendAPIKey,
			config.GetSenderDomain,
		).LoadModule(), nil
	case llm.ModuleName:
		return llm.NewModuleWithGetter(
			config.GetOpenAIProvider,
			config.GetOpenAIEndpoint,
			config.GetOpenAIKey,
			config.GetOpenAIGPTModel,
			config.GetOpenAIDallEModel,
		).LoadModule(), nil
	case cacc.ModuleName:
		return cacc.NewModule().LoadModule(), nil
	case ckv.ModuleName:
		return ckv.NewModule().LoadModule(), nil
	case cfs.ModuleName:
		return cfs.NewModule().LoadModule(), nil
	default:
		return nil, fmt.Errorf("unknown module: %s", name)
	}
}
