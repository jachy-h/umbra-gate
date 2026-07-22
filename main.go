package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jachy-h/llm-gateway-lite/internal/config"
	"github.com/jachy-h/llm-gateway-lite/internal/db"
	"github.com/jachy-h/llm-gateway-lite/internal/server"
)

const appName = "umbragate"

func main() {
	for _, a := range os.Args[1:] {
		switch a {
		case "-h", "--help", "help":
			printUsage(os.Stdout)
			return
		}
	}

	cfgPath := flag.String("config", "", "path to config.yaml")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	d, err := db.Open(cfg.DB.Path)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer d.Close()

	r, statSvc := server.New(cfg, d)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if cfg.Aggregator.IntervalSeconds > 0 {
		go statSvc.Run(ctx, time.Duration(cfg.Aggregator.IntervalSeconds)*time.Second)
	}

	srv := &http.Server{Addr: cfg.Server.Addr, Handler: r}
	go func() {
		log.Printf("llm-gateway-lite listening on %s", cfg.Server.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("shutting down...")
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutCancel()
	_ = srv.Shutdown(shutCtx)
}

func printUsage(w io.Writer) {
	fmt.Fprintf(w, "Usage: %s [flags]\n\n", appName)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  -config string   path to config.yaml (default: ./config.yaml)")
	fmt.Fprintln(w, "  -h, --help       show this help")
}
