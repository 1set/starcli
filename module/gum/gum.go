// Package gum provides a Starlark module for TUI, inspired charmbracelet/gum, huh and bubbletea.
package gum

import (
	"errors"
	"fmt"

	"github.com/1set/starlet"
	"github.com/1set/starlet/dataconv"
	"github.com/1set/starlet/dataconv/types"
	"github.com/charmbracelet/huh"
	"go.starlark.net/starlark"
)

const (
	// ModuleName defines the module name.
	ModuleName = "gum"
)

var (
	none   = starlark.None
	theme  = huh.ThemeCharm()
	keymap = huh.NewDefaultKeyMap()
)

// NewModule creates a new module loader for the gum module.
func NewModule() starlet.ModuleLoader {
	// adjust keymap
	keymap.Text.NewLine.SetHelp("ctrl+j", "new line")
	//keymap.Quit = key.NewBinding(key.WithKeys("ctrl+c", "esc"))
	keymap.FilePicker.Open.SetEnabled(false)

	// build module
	sd := starlark.StringDict{
		"write":        starlark.NewBuiltin(ModuleName+".write", starWrite),
		"input":        starlark.NewBuiltin(ModuleName+".input", starInput),
		"select":       starlark.NewBuiltin(ModuleName+".select", starSelect),
		"multi_select": starlark.NewBuiltin(ModuleName+".multi_select", starMultiSelect),
		"confirm":      starlark.NewBuiltin(ModuleName+".confirm", starConfirm),
		"note":         starlark.NewBuiltin(ModuleName+".note", starNote),
		"spin":         starlark.NewBuiltin(ModuleName+".spin", starSpinner),
		"file_pick":    starlark.NewBuiltin(ModuleName+".file_pick", starFilePicker),
		"colorize":     starlark.NewBuiltin(ModuleName+".colorize", starColorize),
	}
	return dataconv.WrapModuleData(ModuleName, sd)
}

func ignorableError(err error) bool {
	if err == nil {
		return true
	}
	if errors.Is(err, huh.ErrUserAborted) || errors.Is(err, huh.ErrTimeout) {
		return true
	}
	return false
}

func convertListString(raw *types.OneOrMany[starlark.Value]) []string {
	if raw == nil {
		return nil
	}
	ss := make([]string, raw.Len())
	for i, v := range raw.Slice() {
		ss[i] = dataconv.StarString(v)
	}
	return ss
}

// genericValidator is a type that can be either a string or a []string
type genericValidator interface {
	string | []string
}

func convertValidator[T genericValidator](thread *starlark.Thread, nc *types.NullableCallable) func(T) error {
	if nc.IsNull() {
		return func(v T) error {
			return nil
		}
	}

	return func(v T) error {
		fc := nc.Value()
		var arg starlark.Value

		switch val := any(v).(type) {
		case string:
			arg = starlark.String(val)
		case []string:
			ss := make([]starlark.Value, len(val))
			for i, s := range val {
				ss[i] = starlark.String(s)
			}
			arg = starlark.NewList(ss)
		default:
			return fmt.Errorf("unsupported type: %T", v)
		}

		// Call the validator function in Starlark
		nt := &starlark.Thread{Name: "validate", Load: thread.Load, Print: thread.Print, OnMaxSteps: thread.OnMaxSteps}
		res, err := starlark.Call(nt, fc, starlark.Tuple{arg}, nil)
		if err != nil {
			return fmt.Errorf("validator error: %v", err)
		}

		// Treat None or nil as valid
		if res == nil || res == starlark.None {
			return nil
		}

		// Treat other values as error message, except empty string
		ss := dataconv.StarString(res)
		if ss == "" {
			return nil
		}
		return errors.New(ss)
	}
}

func convertStringValidator(thread *starlark.Thread, nc *types.NullableCallable) func(string) error {
	return convertValidator[string](thread, nc)
}

func convertStringListValidator(thread *starlark.Thread, nc *types.NullableCallable) func([]string) error {
	return convertValidator[[]string](thread, nc)
}
