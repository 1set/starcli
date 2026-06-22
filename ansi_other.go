//go:build !windows

package main

// enableANSIControl is a no-op on non-Windows platforms, where terminals already
// understand ANSI escape sequences.
func enableANSIControl() {}
