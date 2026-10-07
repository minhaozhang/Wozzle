//go:build windows

package wslc

import (
	"os/exec"
	"syscall"
)

// hideWindow prevents the child from flashing a console window when the
// parent is a GUI-subsystem (-H windowsgui) process.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}
