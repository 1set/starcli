package config

import (
	_ "embed"
	"fmt"
	"strings"

	cl "bitbucket.org/ai69/colorlogo"
	"github.com/1set/gut/yos"
	"github.com/1set/gut/ystring"
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
	// is on master
	onMaster := GitBranch == "master" || GitBranch == "main" || GitBranch == ""

	// write logo
	var sb strings.Builder
	if onMaster {
		sb.WriteString(cl.AnamnisarByColumn(logoArt))
	} else {
		sb.WriteString(cl.OceanSandByColumn(logoArt))
	}
	sb.WriteString(ystring.NewLine)

	// inline helpers
	arrow := "✰ "
	if yos.IsOnWindows() {
		arrow = "> "
	}
	addNonBlankField := func(name, value string) {
		if ystring.IsNotBlank(value) {
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
