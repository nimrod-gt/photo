//go:build !windows

package proc

import "os/exec"

// Only Windows opens a window for a child process.
func Hide(_ *exec.Cmd) {}
