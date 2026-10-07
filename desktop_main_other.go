//go:build !windows

package main

import (
	"log"
	"net/http"
)

// serveUI: no desktop shell on non-Windows platforms, wait like headless.
func serveUI(httpSrv *http.Server, url string) {
	log.Printf("wozzle %s listening on %s", version, url)
	select {} // unreachable in practice (main checks GOOS); kept for builds
}

func attachParentConsole() {}

func openInBrowser(url string) {}
