//go:build !windows

package wslc

import "os/exec"

func hideWindow(*exec.Cmd) {}
