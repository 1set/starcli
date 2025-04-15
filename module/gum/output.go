package gum

import (
	"fmt"
	"strings"
	"time"

	"bitbucket.org/ai69/colorlogo"
	"github.com/1set/starlet/dataconv/types"
	"github.com/PureMature/starcli/util"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"go.starlark.net/starlark"
)

// starNote is a Starlark function to create a TUI note for showing information to the user.
// def note(title: str, description: str = "", height: int = 0, next: str = "", show_help: bool = True, timeout: float = 0) -> None
func starNote(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		title       string                               // title text
		description = ""                                 // description text
		height      = 0                                  // maximum number of items to show (0 for all)
		wordNext    = types.NewNullableStringOrBytes("") // next word
		showHelp    = true                               // show help key binds
		timeoutSec  = types.FloatOrInt(0)                // timeout in seconds (0 for no timeout)
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"title", &title,
		"description?", &description,
		"height?", &height,
		"next?", wordNext,
		"show_help?", &showHelp,
		"timeout?", &timeoutSec,
	); err != nil {
		return none, err
	}

	// next button
	hasNext := !wordNext.IsNullOrEmpty()
	strNext := wordNext.GoString()

	// run note
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title(title).
				Description(description).
				Height(height).
				Next(hasNext).
				NextLabel(strNext),
		),
	).
		WithTheme(theme).
		WithKeyMap(keymap).
		WithShowHelp(showHelp).
		WithTimeout(time.Duration(timeoutSec) * time.Second).
		Run()

	// handle no result
	if err != nil {
		if ignorableError(err) {
			return none, nil
		}
		return none, err
	}
	return none, nil
}

var spinStyleMap = map[string]spinner.Type{
	"line": spinner.Line,
	"dots": spinner.Dots, "dot": spinner.Dots,
	"mini_dot": spinner.MiniDot, "minidot": spinner.MiniDot, "mini": spinner.MiniDot,
	"jump":   spinner.Jump,
	"points": spinner.Points, "point": spinner.Points,
	"pulse": spinner.Pulse,
	"globe": spinner.Globe, "earth": spinner.Globe,
	"moon":      spinner.Moon,
	"monkey":    spinner.Monkey,
	"meter":     spinner.Meter,
	"hamburger": spinner.Hamburger, "burger": spinner.Hamburger,
	"ellipsis": spinner.Ellipsis,
}

// starSpinner is a Starlark function to show a spinner with an optional action.
// def spin(title: str = "Loading", style: str = "dots", action: Callable = None, timeout: float = 1) -> Any
func starSpinner(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		title      = "Loading..."         // title text
		style      = "dots"               // spinner style
		actionFunc types.NullableCallable // action function
		timeoutSec = types.FloatOrInt(1)  // timeout in seconds, it won't be used if action is set
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"title?", &title,
		"style?", &style,
		"action?", &actionFunc,
		"timeout?", &timeoutSec,
	); err != nil {
		return none, err
	}

	// convert spinner style
	st, ok := spinStyleMap[strings.ToLower(style)]
	if !ok {
		return none, fmt.Errorf("unsupported spinner style: %s", style)
	}

	// action function
	var (
		actRes  starlark.Value = none
		actErr  error
		actFunc = func() {
			if actionFunc.IsNull() {
				// default action: sleep for timeout
				time.Sleep(time.Duration(timeoutSec) * time.Second)
			} else {
				// custom action: call and pass through the result and error
				nt := &starlark.Thread{Name: "spin", Load: thread.Load, Print: thread.Print, OnMaxSteps: thread.OnMaxSteps}
				actRes, actErr = starlark.Call(nt, actionFunc.Value(), nil, nil)
			}
		}
	)

	// run spinner and action
	ts := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#00020A", Dark: "#FFFDF5"})
	err := spinner.New().
		Title(title).
		TitleStyle(ts).
		Type(st).
		Action(actFunc).
		Run()

	// handle no result
	if err != nil {
		if ignorableError(err) {
			return none, nil
		}
		return none, err
	}

	// return action result or default
	return actRes, actErr
}

var colorFuncMap = map[string]func(string) string{
	"almost|column":         colorlogo.AlmostByColumn,
	"almost|line":           colorlogo.AlmostByLine,
	"anamnisar|column":      colorlogo.AnamnisarByColumn,
	"anamnisar|line":        colorlogo.AnamnisarByLine,
	"animalcrossing|column": colorlogo.AnimalCrossingByColumn,
	"animalcrossing|line":   colorlogo.AnimalCrossingByLine,
	"brokenhearts|column":   colorlogo.BrokenHeartsByColumn,
	"brokenhearts|line":     colorlogo.BrokenHeartsByLine,
	"cherryblossoms|column": colorlogo.CherryBlossomsByColumn,
	"cherryblossoms|line":   colorlogo.CherryBlossomsByLine,
	"eveningnight|column":   colorlogo.EveningNightByColumn,
	"eveningnight|line":     colorlogo.EveningNightByLine,
	"ibizasunset|column":    colorlogo.IbizaSunsetByColumn,
	"ibizasunset|line":      colorlogo.IbizaSunsetByLine,
	"miwatch|column":        colorlogo.MiWatchByColumn,
	"miwatch|line":          colorlogo.MiWatchByLine,
	"nelson|column":         colorlogo.NelsonByColumn,
	"nelson|line":           colorlogo.NelsonByLine,
	"oceansand|column":      colorlogo.OceanSandByColumn,
	"oceansand|line":        colorlogo.OceanSandByLine,
	"purplelove|column":     colorlogo.PurpleLoveByColumn,
	"purplelove|line":       colorlogo.PurpleLoveByLine,
	"rainbowblue|column":    colorlogo.RainbowBlueByColumn,
	"rainbowblue|line":      colorlogo.RainbowBlueByLine,
	"rosewater|column":      colorlogo.RoseWaterByColumn,
	"rosewater|line":        colorlogo.RoseWaterByLine,
}

// starColorize is a Starlark function to colorize a string.
// def colorize(text: str, color: str = "", pattern: str = "CherryBlossoms", render: str = "Column") -> str
func starColorize(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		text      string             // text to colorize
		colorName = ""               // color name
		pattern   = "CherryBlossoms" // color name
		render    = "Column"         // render type
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"text", &text,
		"color?", &colorName,
		"pattern?", &pattern,
		"render?", &render,
	); err != nil {
		return none, err
	}

	// if color is set, use it
	if colorName != "" {
		rc, err := util.ParseColor(colorName)
		if err != nil {
			return none, err
		}
		tc := termenv.ColorProfile().FromColor(rc)
		return starlark.String(termenv.String(text).Foreground(tc).String()), nil
	}

	// normalize pattern|render key
	normalizedPattern := normalizePattern(pattern)
	normalizedRender := normalizeRenderType(render)
	key := normalizedPattern + "|" + normalizedRender
	colorFunc, ok := colorFuncMap[key]
	if !ok {
		return none, fmt.Errorf("%s: unsupported pattern: %s", b.Name(), key)
	}

	// colorize text
	return starlark.String(colorFunc(text)), nil
}

func normalizePattern(s string) string {
	return strings.NewReplacer("_", "", "-", "", "|", "").Replace(strings.ToLower(s))
}

func normalizeRenderType(s string) string {
	switch strings.ToLower(s) {
	case "column", "col", "c":
		return "column"
	case "line", "l", "row", "r":
		return "line"
	default:
		return s
	}
}
