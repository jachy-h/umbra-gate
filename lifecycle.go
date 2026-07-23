package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jachy-h/llm-gateway-lite/internal/config"
)

const (
	pidFileName = "umbragate.pid"
	logFileName = "umbragate.log"
	urlFileName = "umbragate.url"
)

func runtimePaths() (pidPath, logPath string, err error) {
	dir, err := config.AppDir()
	if err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	return filepath.Join(dir, pidFileName), filepath.Join(dir, logFileName), nil
}

func readPID(pidPath string) (int, error) {
	b, err := os.ReadFile(pidPath)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 0 {
		return 0, fmt.Errorf("invalid PID file %s", pidPath)
	}
	return pid, nil
}

func processRunning(pid int) bool {
	p, err := os.FindProcess(pid)
	return err == nil && p.Signal(syscall.Signal(0)) == nil
}

func activePID(pidPath string) (int, error) {
	pid, err := readPID(pidPath)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if processRunning(pid) {
		return pid, nil
	}
	if err := os.Remove(pidPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return 0, err
	}
	return 0, nil
}

func startDaemon(configPath string) error {
	pidPath, logPath, err := runtimePaths()
	if err != nil {
		return err
	}
	urlPath := filepath.Join(filepath.Dir(pidPath), urlFileName)
	pid, err := activePID(pidPath)
	if err != nil {
		return err
	}
	if pid != 0 {
		return daemonStatus()
	}

	// Load once before spawning so invalid configuration fails synchronously.
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	webURL, err := browserURL(cfg.Server.Addr)
	if err != nil {
		return fmt.Errorf("resolve web URL: %w", err)
	}

	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	args := []string{"run"}
	if configPath != "" {
		args = append(args, "-config", configPath)
	}
	cmd := exec.Command(executable, args...)
	// A separate session ensures the daemon is not sent SIGHUP when the shell
	// that launched `start` exits.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Env = append(os.Environ(), "UMBRAGATE_BACKGROUND=1")
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer devNull.Close()
	cmd.Stdin = nil
	cmd.Stdout = devNull
	cmd.Stderr = devNull
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", appName, err)
	}
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)+"\n"), 0o644); err != nil {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		return fmt.Errorf("write PID file: %w", err)
	}
	if err := os.WriteFile(urlPath, []byte(webURL+"\n"), 0o644); err != nil {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		_ = os.Remove(pidPath)
		return fmt.Errorf("write URL file: %w", err)
	}
	printRunningStatus(os.Stdout, cmd.Process.Pid, webURL, logPath)
	return nil
}

func stopDaemon() error {
	pidPath, _, err := runtimePaths()
	if err != nil {
		return err
	}
	urlPath := filepath.Join(filepath.Dir(pidPath), urlFileName)
	pid, err := activePID(pidPath)
	if err != nil {
		return err
	}
	if pid == 0 {
		_ = os.Remove(urlPath)
		fmt.Printf("%s is not running\n", appName)
		return nil
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := p.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("stop %s (PID %d): %w", appName, pid, err)
	}

	deadline := time.Now().Add(10 * time.Second)
	for processRunning(pid) {
		if time.Now().After(deadline) {
			return fmt.Errorf("%s (PID %d) did not stop within 10 seconds", appName, pid)
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err := os.Remove(pidPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Remove(urlPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	fmt.Printf("%s stopped\n", appName)
	return nil
}

func daemonStatus() error {
	pidPath, logPath, err := runtimePaths()
	if err != nil {
		return err
	}
	urlPath := filepath.Join(filepath.Dir(pidPath), urlFileName)
	pid, err := activePID(pidPath)
	if err != nil {
		return err
	}
	if pid == 0 {
		_ = os.Remove(urlPath)
		fmt.Printf("%s is not running\n", appName)
		return nil
	}
	webURL, err := readDaemonURL(urlPath)
	if err != nil {
		return err
	}
	printRunningStatus(os.Stdout, pid, webURL, logPath)
	return nil
}

func browserURL(addr string) (string, error) {
	host, port, err := net.SplitHostPort(strings.TrimSpace(addr))
	if err != nil {
		return "", fmt.Errorf("invalid server address %q: %w", addr, err)
	}
	switch host {
	case "", "0.0.0.0", "::":
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, port), nil
}

func readDaemonURL(urlPath string) (string, error) {
	b, err := os.ReadFile(urlPath)
	if err == nil {
		if webURL := strings.TrimSpace(string(b)); webURL != "" {
			return webURL, nil
		}
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("read URL file: %w", err)
	}

	// Processes started by older versions do not have a URL file. Use the
	// default configuration as a backwards-compatible fallback.
	cfg, err := config.Load("")
	if err != nil {
		return "", fmt.Errorf("load config: %w", err)
	}
	return browserURL(cfg.Server.Addr)
}

func printRunningStatus(w io.Writer, pid int, webURL, logPath string) {
	fmt.Fprintln(w, "╭─ UmbraGate")
	fmt.Fprintln(w, "│  Status  ● Running")
	fmt.Fprintf(w, "│  Web UI  %s\n", webURL)
	fmt.Fprintf(w, "│  PID     %d\n", pid)
	fmt.Fprintf(w, "│  Logs    %s\n", logPath)
	fmt.Fprintln(w, "╰─")
}
