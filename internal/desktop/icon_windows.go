//go:build windows

package desktop

import (
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procFindWindowW      = user32.NewProc("FindWindowW")
	procSendMessageW     = user32.NewProc("SendMessageW")
	procGetModuleHandleW = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetModuleHandleW")
	procLoadImageW       = user32.NewProc("LoadImageW")
)

const (
	imageIcon     = 1
	lrDefaultSize = 0x40
	wmSetIcon     = 0x80
	iconBig       = 1
	iconSmall     = 0
)

// setWindowIcon overrides the webview host window's icon with the exe's
// embedded icon resource (group ID 1, added via rsrc_windows_amd64.syso).
// The WebView2 wrapper does not expose window icons itself.
func setWindowIcon(title string) {
	deadline := time.Now().Add(5 * time.Second)
	var hwnd uintptr
	for time.Now().Before(deadline) {
		p, _ := windows.UTF16PtrFromString(title)
		h, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(p)))
		if h != 0 {
			hwnd = h
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if hwnd == 0 {
		return
	}
	inst, _, _ := procGetModuleHandleW.Call(0)
	big, _, _ := procLoadImageW.Call(inst, 1, imageIcon, 0, 0, lrDefaultSize)
	small, _, _ := procLoadImageW.Call(inst, 1, imageIcon, 32, 32, 0)
	if big != 0 {
		_, _, _ = procSendMessageW.Call(hwnd, wmSetIcon, iconBig, big)
	}
	if small != 0 {
		_, _, _ = procSendMessageW.Call(hwnd, wmSetIcon, iconSmall, small)
	}
}
