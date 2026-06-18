package main

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/jachy-h/umbra-gate/api"
	"github.com/jachy-h/umbra-gate/config"
	"github.com/jachy-h/umbra-gate/dashboard"
	"github.com/jachy-h/umbra-gate/db"
	"github.com/jachy-h/umbra-gate/proxy"
)

const (
	appName          = "umbragate"
	configFileName   = "config.yaml"
	databaseFileName = "router.db"
	logFileName      = "umbragate.log"
	daemonEnvKey     = "UMBRAGATE_DAEMONIZED"
)

type cliOptions struct {
	daemon bool
	help   bool
}

//go:embed config.example.yaml
var embeddedConfigExample []byte

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	opts, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n\n", err)
		printUsage(os.Stderr)
		os.Exit(2)
	}
	if opts.help {
		printUsage(os.Stdout)
		return
	}

	appDir, err := resolveAppDir()
	if err != nil {
		slog.Error("failed to determine app directory", "error", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(appDir, 0o700); err != nil {
		slog.Error("failed to create app directory", "path", appDir, "error", err)
		os.Exit(1)
	}
	if err := ensureConfigExample(appDir); err != nil {
		slog.Error("failed to write config example", "path", filepath.Join(appDir, "config.example.yaml"), "error", err)
		os.Exit(1)
	}

	if opts.daemon && os.Getenv(daemonEnvKey) != "1" {
		pid, err := startDaemon(appDir)
		if err != nil {
			slog.Error("failed to start daemon", "error", err)
			os.Exit(1)
		}
		slog.Info("started background server", "pid", pid, "app_dir", appDir, "log", filepath.Join(appDir, logFileName))
		return
	}

	configPath := filepath.Join(appDir, configFileName)
	cfg, err := config.Load(configPath)
	if err != nil {
		slog.Error("failed to load config", "path", configPath, "error", err)
		os.Exit(1)
	}

	dbPath := filepath.Join(appDir, databaseFileName)
	database, err := db.Open(dbPath)
	if err != nil {
		slog.Error("failed to open database", "path", dbPath, "error", err)
		os.Exit(1)
	}
	defer database.Close()

	proxyHandler := proxy.New(cfg, database)
	apiHandler := api.New(database, cfg)
	dashHandler := dashboard.New(database, cfg)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/dashboard", http.StatusFound)
			return
		}
		proxyHandler.ServeHTTP(w, r)
	})
	mux.Handle("/api/", http.StripPrefix("/api", apiHandler))
	mux.Handle("/api", apiHandler)
	mux.Handle("/dashboard", dashHandler)
	mux.Handle("/dashboard/", dashHandler)

	srv := &http.Server{
		Addr:    cfg.Listen(),
		Handler: mux,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("starting server", "listen", cfg.Listen(), "providers", cfg.ProviderIDs())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown error", "error", err)
	}
}

func parseArgs(args []string) (cliOptions, error) {
	var opts cliOptions
	for _, arg := range args {
		switch arg {
		case "-d", "--daemon", "daemon", "deamon":
			opts.daemon = true
		case "-h", "--help", "help":
			opts.help = true
		case "":
			continue
		default:
			return cliOptions{}, fmt.Errorf("unknown argument %q", arg)
		}
	}
	return opts, nil
}

func printUsage(w io.Writer) {
	fmt.Fprintf(w, "Usage: %s [daemon|-d]\n\n", appName)
	fprintf(w, "Commands and flags:\n")
	fprintf(w, "  daemon, deamon, -d, --daemon  Start in background\n")
	fprintf(w, "  -h, --help                    Show this help\n\n")
	fprintf(w, "Config and data:\n")
	fprintf(w, "  Uses ./config.yaml when present in the current directory.\n")
	fprintf(w, "  Otherwise uses ~/.%s/.\n", appName)
	fprintf(w, "  Set UMBRAGATE_HOME to override the directory.\n")
}

func fprintf(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

func resolveAppDir() (string, error) {
	if override := strings.TrimSpace(os.Getenv("UMBRAGATE_HOME")); override != "" {
		return override, nil
	}

	wd, err := os.Getwd()
	if err == nil {
		if _, statErr := os.Stat(filepath.Join(wd, configFileName)); statErr == nil {
			return wd, nil
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, "."+appName), nil
}

func startDaemon(appDir string) (int, error) {
	execPath, err := os.Executable()
	if err != nil {
		return 0, err
	}
	logFile, err := os.OpenFile(filepath.Join(appDir, logFileName), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, err
	}
	defer logFile.Close()

	stdin, err := os.Open("/dev/null")
	if err != nil {
		return 0, err
	}
	defer stdin.Close()

	cmd := exec.Command(execPath, filterDaemonArgs(os.Args[1:])...)
	cmd.Env = append(os.Environ(), daemonEnvKey+"=1")
	cmd.Stdin = stdin
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	return cmd.Process.Pid, nil
}

func filterDaemonArgs(args []string) []string {
	filtered := make([]string, 0, len(args))
	for _, arg := range args {
		switch arg {
		case "-d", "--daemon", "daemon", "deamon":
			continue
		default:
			filtered = append(filtered, arg)
		}
	}
	return filtered
}

func ensureConfigExample(appDir string) error {
	examplePath := filepath.Join(appDir, "config.example.yaml")
	if _, err := os.Stat(examplePath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(examplePath, embeddedConfigExample, 0o600)
}
