//go:build windows

package desktop

import (
	"fmt"
	"net/http"
	"os/exec"
	"time"
	"unsafe"

	webview "github.com/jchv/go-webview2"
	"golang.org/x/sys/windows"
)

var (
	user32               = windows.NewLazySystemDLL("user32.dll")
	procPeekMessageW     = user32.NewProc("PeekMessageW")
	procTranslateMessage = user32.NewProc("TranslateMessage")
	procDispatchMessage  = user32.NewProc("DispatchMessageW")
)

const pmRemove = 1

// pumpMessages dispatches pending Win32 messages for the calling thread
// (tray icon menu etc.) without blocking.
func pumpMessages() {
	var buf [64]byte // MSG, oversized on purpose
	for {
		r, _, _ := procPeekMessageW.Call(
			uintptr(unsafe.Pointer(&buf[0])), 0, 0, 0, pmRemove)
		if r == 0 {
			return
		}
		_, _, _ = procTranslateMessage.Call(uintptr(unsafe.Pointer(&buf[0])))
		_, _, _ = procDispatchMessage.Call(uintptr(unsafe.Pointer(&buf[0])))
	}
}

// waitPump keeps this thread's message loop alive between webview windows so
// the tray menu keeps working while only the tray is visible. Returns true
// when the app should quit.
func waitPump(openCh <-chan struct{}, quitCh <-chan struct{}) bool {
	for {
		pumpMessages()
		select {
		case <-quitCh:
			return true
		case <-openCh:
			return false
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// WindowLoop drives the WebView2 host window on the main thread. The first
// window opens immediately; after the user closes it the loop keeps the tray
// alive (pumping messages) until Open is requested or the app quits.
func WindowLoop(url string, openCh <-chan struct{}, quitCh <-chan struct{}) {
	first := true
	for {
		if !first && waitPump(openCh, quitCh) {
			return
		}
		first = false
		if err := showWindow(url); err != nil {
			openInBrowser(url) // WebView2 failure: degrade to the browser
		}
	}
}

func showWindow(url string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("webview panic: %v", r)
		}
	}()
	w := webview.New(false)
	w.SetTitle("Wozzle — WSL 容器监控")
	w.SetSize(1280, 820, webview.HintNone)
	w.Navigate(url)
	go setWindowIcon("Wozzle — WSL 容器监控")
	w.Run()
	w.Destroy()
	return nil
}

// OpenInBrowser opens url with the default browser.
func OpenInBrowser(url string) { openInBrowser(url) }

func openInBrowser(url string) {
	// explorer.exe is a GUI-subsystem binary: no console flash from
	// windowsgui builds (unlike `cmd /c start`).
	_ = exec.Command("explorer.exe", url).Start()
}

// WaitForHTTP polls the server until it answers or the timeout expires; used
// to avoid flashing a dead page when the window opens before the server binds.
func WaitForHTTP(baseURL string, timeout time.Duration) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := client.Get(baseURL + "/api/system/info")
		if err == nil {
			resp.Body.Close()
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}
