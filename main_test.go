package main

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestPrintVersion(t *testing.T) {
	previous := version
	version = "1.2.3"
	t.Cleanup(func() { version = previous })

	var output bytes.Buffer
	printVersion(&output)

	if got, want := output.String(), "umbragate 1.2.3\n"; got != want {
		t.Fatalf("printVersion() = %q, want %q", got, want)
	}
}

func TestMainWithoutArgumentsShowsUsage(t *testing.T) {
	if os.Getenv("UMBRAGATE_TEST_HELPER") == "1" {
		os.Args = []string{appName}
		main()
		return
	}

	command := exec.Command(os.Args[0], "-test.run=^TestMainWithoutArgumentsShowsUsage$")
	command.Env = append(os.Environ(), "UMBRAGATE_TEST_HELPER=1")
	output, err := command.Output()
	if err != nil {
		t.Fatalf("run bare command: %v", err)
	}
	if !strings.Contains(string(output), "Usage: umbragate [command] [flags]") {
		t.Errorf("bare command output = %q, want usage", output)
	}
}

func TestUsageIncludesVersionCommands(t *testing.T) {
	var output bytes.Buffer
	printUsage(&output)

	for _, want := range []string{"  version           show version information", "  -v, --version    show version information"} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("usage output does not contain %q", want)
		}
	}
}
