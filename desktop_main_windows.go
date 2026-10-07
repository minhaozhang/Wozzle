//go:build windows

package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"wozzle/internal/desktop"
)

// serveUI runs the desktop experience: log file, tray icon, WebView2 window.
func serveUI(httpSrv *http.Server, url string) {
	setupLogFile()
	log.Printf("wozzle %s listening on %s (tray + window)", version, url)

	desktop.WaitForHTTP(url, 3*time.Second)

	openCh := make(chan struct{}, 1)
	quitCh := make(chan struct{})
	desktop.StartTray(desktop.TrayHandlers{
		OnOpen: func() {
			select {
			case openCh <- struct{}{}:
			default:
			}
		},
		OnBrowser: func() { desktop.OpenInBrowser(url) },
		OnQuit:    func() { close(quitCh) },
	}, desktop.AutoStartEnabled, desktop.SetAutoStart)

	desktop.WindowLoop(url, openCh, quitCh)

	log.Println("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
}

// setupLogFile tees logs to %USERPROFILE%\.wozzle\wozzle.log (windowsgui
// builds have no console).
func setupLogFile() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	dir := filepath.Join(home, ".wozzle")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(dir, "wozzle.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	log.SetOutput(f)
}

func attachParentConsole() { desktop.AttachParentConsole() }

func openInBrowser(url string) { desktop.OpenInBrowser(url) }
