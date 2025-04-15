// Package gum provides a Starlark module for TUI, inspired charmbracelet/gum, huh and bubbletea.
package gum

import (
	"fmt"
	"os"
	"time"

	"github.com/1set/starlet/dataconv"
	"github.com/1set/starlet/dataconv/types"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"go.starlark.net/starlark"
)

// starWrite is a Starlark function to create a TUI text area for getting multi-line input from the user.
// def write(value: str = "", placeholder: str = "Write something...", title: str = "", description: str = "", char_limit: int = 0, validate: Callable = None, width: int = 50, height: int = 5, show_line: bool = false, show_help: bool = true, timeout: float = 0) -> str
func starWrite(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		initialValue    starlark.Value         // initial value, converted to string if not already
		placeholder     = "Write something..." // placeholder value
		title           = ""                   // title text
		description     = ""                   // description text
		charLimit       = 0                    // maximum value length (0 for no limit)
		validateFunc    types.NullableCallable // validation function
		width           = 50                   // text area width (0 for terminal width)
		height          = 5                    // text area height
		showLineNumbers = false                // show line numbers
		showHelp        = true                 // show help key binds
		timeoutSec      = types.FloatOrInt(0)  // timeout in seconds (0 for no timeout)
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"value?", &initialValue,
		"placeholder?", &placeholder,
		"title?", &title,
		"description?", &description,
		"char_limit?", &charLimit,
		"validate?", &validateFunc,
		"width?", &width,
		"height?", &height,
		"show_line?", &showLineNumbers,
		"show_help?", &showHelp,
		"timeout?", &timeoutSec,
	); err != nil {
		return none, err
	}

	// run form
	value := dataconv.StarString(initialValue)
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewText().
				Title(title).
				Description(description).
				Placeholder(placeholder).
				Validate(convertStringValidator(thread, &validateFunc)).
				CharLimit(charLimit).
				ShowLineNumbers(showLineNumbers).
				Value(&value),
		),
	).
		WithWidth(width).
		WithHeight(height).
		WithTheme(theme).
		WithKeyMap(keymap).
		WithShowHelp(showHelp).
		WithTimeout(time.Duration(timeoutSec) * time.Second).
		Run()

	// handle results
	if err != nil {
		if ignorableError(err) {
			return none, nil
		}
		return none, err
	}
	return starlark.String(value), nil
}

// starInput is a Starlark function to create a TUI input field for getting single-line text from the user.
// def input(value: str = "", prompt: str = "> ", placeholder: str = "Type something...", title: str = "", description: str = "", char_limit: int = 0, suggestions: List[str] = [], password: bool = false, validate: Callable = None, width: int = 50, inline: bool = false, show_help: bool = true, timeout: float = 0) -> str
func starInput(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		initialValue starlark.Value                                                         // initial value, converted to string if not already
		prompt                              = "> "                                          // prompt text
		placeholder                         = "Type something..."                           // placeholder value
		title                               = ""                                            // title text
		description                         = ""                                            // description text
		charLimit                           = 0                                             // maximum value length (0 for no limit)
		suggestions                         = types.NewOneOrManyNoDefault[starlark.Value]() // suggestions to display for autocomplete
		password     starlark.Value         = starlark.Bool(false)                          // password mode
		validateFunc types.NullableCallable                                                 // validation function
		width        = 50                                                                   // text area width (0 for terminal width)
		inline       = false                                                                // inline mode
		showHelp     = true                                                                 // show help key binds
		timeoutSec   = types.FloatOrInt(0)                                                  // timeout in seconds (0 for no timeout)
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"value?", &initialValue,
		"prompt?", &prompt,
		"placeholder?", &placeholder,
		"title?", &title,
		"description?", &description,
		"char_limit?", &charLimit,
		"suggestions?", suggestions,
		"password?", &password,
		"validate?", &validateFunc,
		"width?", &width,
		"inline?", &inline,
		"show_help?", &showHelp,
		"timeout?", &timeoutSec,
	); err != nil {
		return none, err
	}

	// convert password mode to echo mode
	var echoMode huh.EchoMode
	switch t := password.(type) {
	case starlark.NoneType:
		echoMode = huh.EchoModeNone
	case starlark.Bool:
		if t.Truth() {
			echoMode = huh.EchoModePassword
		} else {
			echoMode = huh.EchoModeNormal
		}
	default:
		return none, fmt.Errorf("%s: password must be a bool or None", b.Name())
	}

	// convert suggestions
	suggests := convertListString(suggestions)

	// run form
	value := dataconv.StarString(initialValue)
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Prompt(prompt).
				Title(title).
				Description(description).
				Placeholder(placeholder).
				Validate(convertStringValidator(thread, &validateFunc)).
				CharLimit(charLimit).
				Suggestions(suggests).
				EchoMode(echoMode).
				Inline(inline).
				Value(&value),
		),
	).
		WithWidth(width).
		WithTheme(theme).
		WithKeyMap(keymap).
		WithShowHelp(showHelp).
		WithTimeout(time.Duration(timeoutSec) * time.Second).
		WithProgramOptions(tea.WithOutput(os.Stderr)).
		Run()

	// handle results
	if err != nil {
		if ignorableError(err) {
			return none, nil
		}
		return none, err
	}
	return starlark.String(value), nil
}
