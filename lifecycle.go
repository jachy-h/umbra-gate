package main

import (
	"errors"
	"fmt"
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
	pid, err := activePID(pidPath)
	if err != nil {
		return err
	}
	if pid != 0 {
		return fmt.Errorf("%s is already running (PID %d)", appName, pid)
	}

	// Load once before spawning so invalid configuration fails synchronously.
	if _, err := config.Load(configPath); err != nil {
		return fmt.Errorf("load config: %w", err)
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
	fmt.Printf("%s started in the background (PID %d)\nlog: %s\n", appName, cmd.Process.Pid, logPath)
	return nil
}

func stopDaemon() error {
	pidPath, _, err := runtimePaths()
	if err != nil {
		return err
	}
	pid, err := activePID(pidPath)
	if err != nil {
		return err
	}
	if pid == 0 {
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
	fmt.Printf("%s stopped\n", appName)
	return nil
}

func daemonStatus() error {
	pidPath, logPath, err := runtimePaths()
	if err != nil {
		return err
	}
	pid, err := activePID(pidPath)
	if err != nil {
		return err
	}
	if pid == 0 {
		fmt.Printf("%s is not running\n", appName)
		return nil
	}
	fmt.Printf("%s is running (PID %d)\nlog: %s\n", appName, pid, logPath)
	return nil
}
