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
	"path/filepath"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/jachy-h/llm-gateway-lite/internal/config"
	"github.com/jachy-h/llm-gateway-lite/internal/db"
	appLogging "github.com/jachy-h/llm-gateway-lite/internal/logging"
	"github.com/jachy-h/llm-gateway-lite/internal/server"
)

const appName = "umbragate"

// version is set at build time for release binaries with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	args := os.Args[1:]
	if len(args) > 0 {
		switch args[0] {
		case "-h", "--help", "help":
			printUsage(os.Stdout)
			return
		case "-v", "--version", "version":
			if len(args) != 1 {
				log.Fatal("version does not accept arguments")
			}
			printVersion(os.Stdout)
			return
		case "start":
			configPath, err := parseConfigFlag("start", args[1:])
			if err != nil {
				log.Fatal(err)
			}
			if err := startDaemon(configPath); err != nil {
				log.Fatal(err)
			}
			return
		case "stop":
			if len(args) != 1 {
				log.Fatal("stop does not accept arguments")
			}
			if err := stopDaemon(); err != nil {
				log.Fatal(err)
			}
			return
		case "restart":
			configPath, err := parseConfigFlag("restart", args[1:])
			if err != nil {
				log.Fatal(err)
			}
			if err := stopDaemon(); err != nil {
				log.Fatal(err)
			}
			if err := startDaemon(configPath); err != nil {
				log.Fatal(err)
			}
			return
		case "status":
			if len(args) != 1 {
				log.Fatal("status does not accept arguments")
			}
			if err := daemonStatus(); err != nil {
				log.Fatal(err)
			}
			return
		case "run":
			args = args[1:]
		}
	}

	cfgPath, err := parseConfigFlag("run", args)
	if err != nil {
		log.Fatal(err)
	}

	effectiveConfigPath, err := config.ResolvePath(cfgPath)
	if err != nil {
		log.Fatalf("resolve config path: %v", err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if os.Getenv("UMBRAGATE_BACKGROUND") == "1" {
		appDir, err := config.AppDir()
		if err != nil {
			log.Fatalf("resolve log directory: %v", err)
		}
		writer, err := appLogging.New(appLogging.Options{Path: filepath.Join(appDir, logFileName), MaxSizeMB: cfg.Logging.MaxSizeMB, MaxBackups: cfg.Logging.MaxBackups, MaxAgeDays: cfg.Logging.MaxAgeDays, Compress: cfg.Logging.Compress})
		if err != nil {
			log.Fatalf("configure log rotation: %v", err)
		}
		defer writer.Close()
		log.SetOutput(writer)
		gin.DefaultWriter = writer
	}
	log.Printf("configuration file: %s", effectiveConfigPath)
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

func parseConfigFlag(command string, args []string) (string, error) {
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", "", "path to config.yaml")
	if err := fs.Parse(args); err != nil {
		return "", err
	}
	if fs.NArg() != 0 {
		return "", fmt.Errorf("unexpected arguments: %s", fs.Args())
	}
	return *configPath, nil
}

func printUsage(w io.Writer) {
	fmt.Fprintf(w, "Usage: %s [command] [flags]\n\n", appName)
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  start             start in the background")
	fmt.Fprintln(w, "  stop              gracefully stop the background process")
	fmt.Fprintln(w, "  restart           restart the background process")
	fmt.Fprintln(w, "  status            show background process status")
	fmt.Fprintln(w, "  run               run in the foreground (default)")
	fmt.Fprintln(w, "  version           show version information")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  -config string   path to config.yaml (default: ~/.umbragate/config.yaml)")
	fmt.Fprintln(w, "  -h, --help       show this help")
	fmt.Fprintln(w, "  -v, --version    show version information")
}

func printVersion(w io.Writer) {
	fmt.Fprintf(w, "%s %s\n", appName, version)
}
