package cli

import (
	"fmt"
	"sort"

	"github.com/1set/starbox"
	"github.com/1set/starcli/config"
	"github.com/1set/starcli/module/sys"
	"github.com/1set/starlet"
	"github.com/samber/lo"
	"github.com/starpkg/cmd"
	"github.com/starpkg/email"
	"github.com/starpkg/gum"
	"github.com/starpkg/llm"
	"github.com/starpkg/markdown"
)

var (
	starMods = starlet.GetAllBuiltinModuleNames()
	cliMods  = []string{
		sys.ModuleName,
		gum.ModuleName,
		email.ModuleName,
		llm.ModuleName,
		markdown.ModuleName,
		cmd.ModuleName,
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
		return cmd.NewModule().LoadModule(), nil
	default:
		return nil, fmt.Errorf("unknown module: %s", name)
	}
}
