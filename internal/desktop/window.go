//go:build windows

package desktop

import (
	"fmt"
	"net/http"
	"os/exec"
	"time"

	webview "github.com/jchv/go-webview2"
)

// WindowLoop drives the WebView2 host window. It blocks until quitCh fires.
// openCh requests (re)opening the window after the user closed it; the
// process stays alive in tray-only mode between windows. The first window
// opens immediately.
func WindowLoop(url string, openCh <-chan struct{}, quitCh <-chan struct{}) {
	open := true
	for {
		if !open {
			select {
			case <-quitCh:
				return
			case <-openCh:
			}
		}
		open = false
		if err := showWindow(url); err != nil {
			openInBrowser(url) // WebView2 failure: degrade to the browser
			select {
			case <-quitCh:
				return
			case <-openCh:
				continue
			}
		}
		// window closed by user; keep tray alive, wait for next request
		select {
		case <-quitCh:
			return
		case <-openCh:
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
	w.Run()
	w.Destroy()
	return nil
}

// OpenInBrowser opens url with the default browser.
func OpenInBrowser(url string) { openInBrowser(url) }

func openInBrowser(url string) {
	_ = exec.Command("cmd", "/c", "start", "", url).Start()
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
