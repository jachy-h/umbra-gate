package main

import (
	"bytes"
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

func TestUsageIncludesVersionCommands(t *testing.T) {
	var output bytes.Buffer
	printUsage(&output)

	for _, want := range []string{"  version           show version information", "  -v, --version    show version information"} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("usage output does not contain %q", want)
		}
	}
}
