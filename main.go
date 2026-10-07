package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"wozzle/internal/server"
	"wozzle/internal/store"
	"wozzle/internal/wslc"
)

const version = "0.1.0"

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "listen address")
	exe := flag.String("wslc", "wslc", "path to wslc.exe")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println("wozzle", version)
		return
	}

	provider := wslc.New(*exe)
	st := store.New()
	dist, err := fs.Sub(distFS, "web/dist")
	if err != nil {
		log.Fatal(err)
	}

	srv := server.New(provider, st, dist, version)
	defer srv.Close()

	httpSrv := &http.Server{Addr: *addr, Handler: srv.Routes()}
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()
	log.Printf("wozzle %s listening on http://%s", version, *addr)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	<-ctx.Done()
	log.Println("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
}
