package config

import (
	_ "embed"
	"fmt"
	"runtime"
	"strings"
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

// DisplayBuildInfo prints the build information to the console.
func DisplayBuildInfo() {
	// write logo
	var sb strings.Builder
	sb.WriteString(colorizeLogo(logoArt))
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

// colorizeLogo renders the logo art with a per-line ANSI 256-colour gradient.
func colorizeLogo(art string) string {
	palette := []int{45, 44, 43, 49, 48, 84, 78, 79} // teal → green gradient
	var sb strings.Builder
	for i, line := range strings.Split(strings.TrimRight(art, "\n"), "\n") {
		fmt.Fprintf(&sb, "\x1b[38;5;%dm%s\x1b[0m\n", palette[i%len(palette)], line)
	}
	return sb.String()
}
