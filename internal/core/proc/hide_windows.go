//go:build windows

package proc

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

// The app is built for the GUI subsystem and owns no console, so Windows hands
// every console child process a console window of its own.
func Hide(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}
