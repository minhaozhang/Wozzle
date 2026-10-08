package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"wozzle/internal/config"
	"wozzle/internal/server"
	"wozzle/internal/store"
	"wozzle/internal/wslc"
)

const version = "0.2.1"

func main() {
	var (
		addrFlag    = flag.String("addr", "", `listen address（优先级：-addr > 环境变量 WOZZLE_ADDR > wozzle.json > 默认 127.0.0.1:8080）`)
		headless    = flag.Bool("headless", false, "纯服务模式：不启动托盘和窗口，输出控制台日志")
		browserFlag = flag.Bool("open-browser", false, "启动后用默认浏览器打开面板（headless 模式下生效）")
		exeFlag     = flag.String("wslc", "wslc", "path to wslc.exe")
		showVersion = flag.Bool("version", false, "print version and exit")
	)
	flag.Parse()
	if *showVersion {
		fmt.Println("wozzle", version)
		return
	}

	// config priority: -addr > WOZZLE_ADDR > wozzle.json > default
	cfgPath, err := config.DefaultPath()
	if err != nil {
		cfgPath = "wozzle.json"
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Printf("config %s: %v (using defaults)", cfgPath, err)
	}
	addr := cfg.Addr
	if v := os.Getenv("WOZZLE_ADDR"); v != "" {
		addr = v
	}
	if *addrFlag != "" {
		addr = *addrFlag
	}

	provider := wslc.New(*exeFlag)
	st := store.New()
	dist, err := fs.Sub(distFS, "web/dist")
	if err != nil {
		log.Fatal(err)
	}
	srv := server.New(provider, st, dist, version)
	defer srv.Close()

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("listen %s: %v", addr, err)
	}
	httpSrv := &http.Server{Handler: srv.Routes()}
	go func() {
		if err := httpSrv.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Fatalf("serve: %v", err)
		}
	}()
	boundURL := fmt.Sprintf("http://%s", listener.Addr().String())

	if *headless {
		runHeadless(httpSrv, boundURL, cfg.OpenBrowser || *browserFlag)
		return
	}
	serveUI(httpSrv, boundURL)
}

func runHeadless(httpSrv *http.Server, url string, openBrowser bool) {
	attachParentConsole()
	log.Printf("wozzle %s (headless) listening on %s", version, url)
	if openBrowser {
		go openInBrowser(url)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	<-ctx.Done()
	log.Println("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
}
