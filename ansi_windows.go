//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows"
)

// enableANSIControl turns on virtual-terminal processing for stdout so ANSI
// escape sequences (colours, cursor control) render on Windows consoles.
func enableANSIControl() {
	stdout := windows.Handle(os.Stdout.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(stdout, &mode); err != nil {
		return
	}
	_ = windows.SetConsoleMode(stdout, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
}
