//go:build windows

package desktop

import (
	"os"

	"golang.org/x/sys/windows"
)

// AttachParentConsole re-attaches stdout/stderr to the console the process
// was launched from. Useful for -headless runs of a -H windowsgui build.
func AttachParentConsole() {
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	attach := kernel32.NewProc("AttachConsole")
	getStdHandle := kernel32.NewProc("GetStdHandle")
	const attachParentProcess = ^uintptr(0) // (DWORD)-1
	const stdOutputHandle = ^uintptr(10)    // (DWORD)-11
	const stdErrorHandle = ^uintptr(11)     // (DWORD)-12

	r, _, _ := attach.Call(attachParentProcess)
	if r == 0 {
		return // not launched from a console
	}
	if h, _, _ := getStdHandle.Call(stdOutputHandle); h != 0 && h != ^uintptr(0) {
		os.Stdout = os.NewFile(h, "/dev/stdout")
	}
	if h, _, _ := getStdHandle.Call(stdErrorHandle); h != 0 && h != ^uintptr(0) {
		os.Stderr = os.NewFile(h, "/dev/stderr")
	}
}
