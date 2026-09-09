package setup

import "testing"

func TestLegacyRootEmail(t *testing.T) {
	if got := LegacyRootEmail("Admin@Corp.example", ""); got != "admin@corp.example" {
		t.Fatalf("keep email: %s", got)
	}
	if got := LegacyRootEmail("admin", "vpn.example.com"); got != "admin@vpn.example.com" {
		t.Fatalf("from host: %s", got)
	}
	if got := LegacyRootEmail("admin", "https://vpn.example.com:443/path"); got != "admin@vpn.example.com" {
		t.Fatalf("stripped host: %s", got)
	}
	if got := LegacyRootEmail("admin", "10.0.0.1"); got != "admin" {
		t.Fatalf("ip host: %s", got)
	}
	if got := LegacyRootEmail("admin", ""); got != "admin" {
		t.Fatalf("empty host: %s", got)
	}
}
