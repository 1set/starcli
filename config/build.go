package config

import (
	_ "embed"
	"fmt"
	"math/rand"
	"runtime"
	"strings"

	cl "bitbucket.org/ai69/colorlogo"
)

// revive:disable:exported
var (
	AppName    string
	CIBuildNum string
	BuildDate  string
	BuildHost  string
	GoVersion  string
	GitBranch  string
	GitCommit  string
	GitSummary string
)

var (
	//go:embed logo.txt
	logoArt string
)

// logoGradients is a set of colorlogo colour-scheme presets; DisplayBuildInfo
// picks one at random each run, so the banner shows a different palette on every
// launch.
var logoGradients = []func(string) string{
	cl.OceanSandByColumn,
	cl.AnamnisarByColumn,
	cl.IbizaSunsetByColumn,
	cl.PurpleParadiseByColumn,
	cl.RainbowBlueByColumn,
	cl.EveningNightByColumn,
	cl.SublimeVividByColumn,
	cl.CherryBlossomsByColumn,
}

// DisplayBuildInfo prints the build information to the console.
func DisplayBuildInfo() {
	// write the logo with a randomly chosen colour scheme
	var sb strings.Builder
	sb.WriteString(logoGradients[rand.Intn(len(logoGradients))](logoArt))
	sb.WriteString("\n")

	// inline helpers
	arrow := "✰ "
	if runtime.GOOS == "windows" {
		arrow = "> "
	}
	addNonBlankField := func(name, value string) {
		if strings.TrimSpace(value) != "" {
			fmt.Fprintln(&sb, arrow+name+":", value)
		}
	}

	addNonBlankField("Build Num ", CIBuildNum)
	addNonBlankField("Build Date", BuildDate)
	addNonBlankField("Build Host", BuildHost)
	addNonBlankField("Go Version", GoVersion)
	addNonBlankField("Git Branch", GitBranch)
	addNonBlankField("Git Commit", GitCommit)
	addNonBlankField("GitSummary", GitSummary)

	fmt.Println(sb.String())
}
