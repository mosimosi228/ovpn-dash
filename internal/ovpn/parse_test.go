package ovpn

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFile(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "server.conf")
	ta := filepath.Join(dir, "ta.key")
	crl := filepath.Join(dir, "crl.pem")
	if err := os.WriteFile(ta, []byte("-----BEGIN OpenVPN Static key V1-----\nabc\n-----END OpenVPN Static key V1-----\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	body := `
# comment
port 1195
proto udp4
dev tun
data-ciphers AES-256-GCM:AES-128-GCM
auth SHA256
tls-crypt ta.key
crl-verify crl.pem
status /var/log/openvpn/status.log
log /var/log/openvpn/server.log
<ca>
-----BEGIN CERTIFICATE-----
xxx
-----END CERTIFICATE-----
</ca>
`
	if err := os.WriteFile(conf, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := ParseFile(conf)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 1195 {
		t.Fatalf("port %d", cfg.Port)
	}
	if cfg.Proto != "udp" {
		t.Fatalf("proto %s", cfg.Proto)
	}
	if cfg.Cipher != "AES-256-GCM" {
		t.Fatalf("cipher %s", cfg.Cipher)
	}
	if !cfg.HasTLSCrypt || cfg.TLSCryptPath != ta {
		t.Fatalf("tls-crypt %v %s", cfg.HasTLSCrypt, cfg.TLSCryptPath)
	}
	if !cfg.HasCRLVerify || cfg.CRLVerify != crl {
		t.Fatalf("crl %v %s", cfg.HasCRLVerify, cfg.CRLVerify)
	}
	if cfg.StatusFile != "/var/log/openvpn/status.log" {
		t.Fatalf("status %s", cfg.StatusFile)
	}
	if cfg.LogFile != "/var/log/openvpn/server.log" {
		t.Fatalf("log %s", cfg.LogFile)
	}
}

func TestParseFileWarnings(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "server.conf")
	if err := os.WriteFile(conf, []byte("port 1194\nproto tcp\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := ParseFile(conf)
	if err != nil {
		t.Fatal(err)
	}
	w := cfg.Warnings()
	if len(w) != 2 {
		t.Fatalf("want 2 warnings got %v", w)
	}
}
