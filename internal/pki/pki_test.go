package pki

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mosimosi228/ovpn-dash/internal/ovpn"
	"github.com/mosimosi228/ovpn-dash/internal/setup"
)

func writeTestCA(t *testing.T, dir string) {
	t.Helper()
	writeTestCAUsage(t, dir, x509.KeyUsageCertSign|x509.KeyUsageCRLSign|x509.KeyUsageDigitalSignature)
}

func writeTestCAUsage(t *testing.T, dir string, usage x509.KeyUsage) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Easy-RSA CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              usage,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "private"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "issued"), 0o755); err != nil {
		t.Fatal(err)
	}
	crt := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err := os.WriteFile(filepath.Join(dir, "ca.crt"), crt, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "private", "ca.key"), keyPEM, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestIssueRevokeProfile(t *testing.T) {
	dir := t.TempDir()
	writeTestCA(t, dir)
	s := &Store{Dir: dir, Clients: filepath.Join(dir, "clients")}
	if err := s.Issue("alice"); err != nil {
		t.Fatal(err)
	}
	if err := s.Issue("alice"); err == nil {
		t.Fatal("duplicate should fail")
	}
	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "alice" || list[0].Revoked {
		t.Fatalf("list %+v", list)
	}
	ta := filepath.Join(dir, "ta.key")
	if err := os.WriteFile(ta, []byte("-----BEGIN OpenVPN Static key V1-----\n00\n-----END OpenVPN Static key V1-----\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &ovpn.Config{
		Port:         1194,
		Proto:        "udp",
		Cipher:       "AES-256-GCM",
		Auth:         "SHA256",
		Dev:          "tun",
		HasTLSCrypt:  true,
		TLSCryptPath: ta,
	}
	prof, err := s.Profile("alice", "vpn.example.com", cfg)
	if err != nil {
		t.Fatal(err)
	}
	text := string(prof)
	for _, want := range []string{"remote vpn.example.com 1194", "<ca>", "<cert>", "<key>", "<tls-crypt>", "cipher AES-256-GCM"} {
		if !strings.Contains(text, want) {
			t.Fatalf("profile missing %q:\n%s", want, text)
		}
	}
	if err := s.SaveOvpn("alice", prof); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "clients", "alice.ovpn")); err != nil {
		t.Fatal(err)
	}
	if err := s.Revoke("alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "crl.pem")); err != nil {
		t.Fatal(err)
	}
	list, err = s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || !list[0].Revoked || list[0].HasKey {
		t.Fatalf("expected revoked cert kept without key, got %+v", list)
	}
	if _, err := os.Stat(filepath.Join(dir, "clients", "alice.ovpn")); !os.IsNotExist(err) {
		t.Fatalf("revoked ovpn copy should be removed: %v", err)
	}
}

func TestValidateName(t *testing.T) {
	if err := ValidateName("ok_client-1"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateName("Иван_Петров"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateName("клиент-1"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateName("../etc"); err == nil {
		t.Fatal("expected error")
	}
	if err := ValidateName("ca"); err == nil {
		t.Fatal("expected error")
	}
	if err := ValidateName("a/b"); err == nil {
		t.Fatal("slash")
	}
}

func TestRevokeWithoutCRLSignBit(t *testing.T) {
	dir := t.TempDir()
	writeTestCAUsage(t, dir, x509.KeyUsageCertSign|x509.KeyUsageDigitalSignature)
	s := &Store{Dir: dir}
	if err := s.Issue("bob"); err != nil {
		t.Fatal(err)
	}
	if err := s.Revoke("bob"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "crl.pem")); err != nil {
		t.Fatal(err)
	}
}

func TestIssueCyrillic(t *testing.T) {
	dir := t.TempDir()
	writeTestCA(t, dir)
	s := &Store{Dir: dir}
	if err := s.Issue("Сергей"); err != nil {
		t.Fatal(err)
	}
	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "Сергей" {
		t.Fatalf("list %+v", list)
	}
}

func TestClientsDir(t *testing.T) {
	s := &Store{}
	if s.clientsDir() != setup.DefaultClientsDir {
		t.Fatalf("default: %s", s.clientsDir())
	}
	s.Clients = "/tmp/ovpn-client"
	if s.clientsDir() != "/tmp/ovpn-client" {
		t.Fatalf("override: %s", s.clientsDir())
	}
}

func TestWriteOvpnToClientsDir(t *testing.T) {
	root := t.TempDir()
	pkiDir := filepath.Join(root, "easy-rsa", "pki")
	writeTestCA(t, pkiDir)
	clients := filepath.Join(root, "server", "client")
	s := &Store{Dir: pkiDir, Clients: clients}
	if err := s.Issue("alice"); err != nil {
		t.Fatal(err)
	}
	cfg := &ovpn.Config{Port: 1194, Proto: "udp", Cipher: "AES-256-GCM", Auth: "SHA256", Dev: "tun"}
	if err := s.WriteOvpn("alice", "vpn.example.com", cfg); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(clients, "alice.ovpn")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "remote vpn.example.com 1194") {
		t.Fatalf("%s", body)
	}
	s.SyncOvpns("vpn.example.com", cfg)
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	if err := s.Revoke("alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected ovpn removed: %v", err)
	}
}

func TestReissueAndCRLCopy(t *testing.T) {
	dir := t.TempDir()
	writeTestCA(t, dir)
	s := &Store{Dir: dir}
	if err := s.Issue("alice"); err != nil {
		t.Fatal(err)
	}
	old, err := os.ReadFile(filepath.Join(dir, "issued", "alice.crt"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Reissue("alice"); err != nil {
		t.Fatal(err)
	}
	next, err := os.ReadFile(filepath.Join(dir, "issued", "alice.crt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(old) == string(next) {
		t.Fatal("cert should change")
	}
	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Revoked || !list[0].HasKey {
		t.Fatalf("list %+v", list)
	}
	if _, err := os.Stat(filepath.Join(dir, "crl.pem")); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "server-crl.pem")
	if err := s.EnsureCRL(); err != nil {
		t.Fatal(err)
	}
	if err := s.CopyCRL(dest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Fatal(err)
	}
}
