package httpserver

import (
	"strings"
	"testing"
)

func TestFilterHumanLog(t *testing.T) {
	in := strings.Join([]string{
		"peer connected",
		"MANAGEMENT: CMD 'status 3'",
		"MANAGEMENT: CMD 'bytecount 1'",
		"SUCCESS: bytecount interval changed",
		"SUCCESS: hold release succeeded",
		"TLS Error: TLS handshake failed",
	}, "\n")
	got := filterHumanLog(in)
	for _, want := range []string{"peer connected", "SUCCESS: hold release succeeded", "TLS Error: TLS handshake failed"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
	for _, drop := range []string{"status 3", "bytecount 1", "bytecount interval"} {
		if strings.Contains(got, drop) {
			t.Fatalf("kept %q in %q", drop, got)
		}
	}
}
