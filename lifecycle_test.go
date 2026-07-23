package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestBrowserURL(t *testing.T) {
	tests := []struct {
		addr string
		want string
	}{
		{addr: ":8787", want: "http://localhost:8787"},
		{addr: "0.0.0.0:9000", want: "http://localhost:9000"},
		{addr: "[::]:8787", want: "http://localhost:8787"},
		{addr: "127.0.0.1:8787", want: "http://127.0.0.1:8787"},
		{addr: "[2001:db8::1]:8787", want: "http://[2001:db8::1]:8787"},
	}

	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			got, err := browserURL(tt.addr)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("browserURL(%q) = %q, want %q", tt.addr, got, tt.want)
			}
		})
	}
}

func TestBrowserURLRejectsInvalidAddress(t *testing.T) {
	if _, err := browserURL("8787"); err == nil {
		t.Fatal("browserURL accepted an address without a host/port separator")
	}
}

func TestPrintRunningStatusIncludesClickableURL(t *testing.T) {
	var output bytes.Buffer
	printRunningStatus(&output, 12345, "http://localhost:8787", "/tmp/umbragate.log")

	for _, want := range []string{
		"╭─ UmbraGate\n",
		"│  Status  ● Running\n",
		"│  Web UI  http://localhost:8787\n",
		"│  PID     12345\n",
		"│  Logs    /tmp/umbragate.log\n",
		"╰─\n",
	} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("output does not contain %q:\n%s", want, output.String())
		}
	}
}
