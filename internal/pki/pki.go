package pki

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/mosimosi228/ovpn-dash/internal/ovpn"
)

const clientCertDays = 825

// Store talks to an existing easy-rsa-style PKI on disk.
type Store struct {
	Dir string
}

type Client struct {
	Name     string    `json:"name"`
	NotAfter time.Time `json:"not_after"`
	Serial   string    `json:"serial"`
	Revoked  bool      `json:"revoked"`
	HasKey   bool      `json:"has_key"`
	IssuedAt time.Time `json:"issued_at"`
}

func (s *Store) caCrtPath() string { return filepath.Join(s.Dir, "ca.crt") }
func (s *Store) caKeyPath() string {
	p := filepath.Join(s.Dir, "private", "ca.key")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return filepath.Join(s.Dir, "ca.key")
}
func (s *Store) issuedDir() string  { return filepath.Join(s.Dir, "issued") }
func (s *Store) privateDir() string { return filepath.Join(s.Dir, "private") }
func (s *Store) crlPath() string    { return filepath.Join(s.Dir, "crl.pem") }
func (s *Store) indexPath() string  { return filepath.Join(s.Dir, "index.txt") }
func (s *Store) serialPath() string { return filepath.Join(s.Dir, "serial") }

func ValidateName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" || strings.EqualFold(name, "ca") || name == "." || name == ".." || strings.EqualFold(name, "UNDEF") {
		return fmt.Errorf("invalid client name")
	}
	if path.Base(name) != name || strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("invalid client name")
	}
	if utf8.RuneCountInString(name) > 64 {
		return fmt.Errorf("client name too long")
	}
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == '-' {
			continue
		}
		return fmt.Errorf("client name must be letters, digits, dot, underscore or hyphen")
	}
	return nil
}

// opensslTime — формат даты для index.txt: YYMMDDHHMMSSZ (UTC)
func opensslTime(t time.Time) string {
	return t.UTC().Format("060102150405Z")
}

// nextSerial читает pki/serial, возвращает следующий номер и записывает новый.
// Если файла нет — стартует с 01 (как easy-rsa).
func (s *Store) nextSerial() (*big.Int, error) {
	serialPath := s.serialPath()
	b, err := os.ReadFile(serialPath)
	var cur *big.Int
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		cur = big.NewInt(1)
	} else {
		s := strings.TrimSpace(string(b))
		cur = new(big.Int)
		if _, ok := cur.SetString(s, 16); !ok {
			return nil, fmt.Errorf("invalid serial file: %q", s)
		}
	}
	next := new(big.Int).Add(cur, big.NewInt(1))
	hex := strings.ToUpper(next.Text(16))
	if len(hex)%2 != 0 {
		hex = "0" + hex
	}
	if err := os.WriteFile(serialPath, []byte(hex+"\n"), 0o644); err != nil {
		return nil, err
	}
	return cur, nil
}

// appendIndexV добавляет строку V в index.txt
func (s *Store) appendIndexV(cert *x509.Certificate) error {
	serial := strings.ToUpper(cert.SerialNumber.Text(16))
	if len(serial)%2 != 0 {
		serial = "0" + serial
	}
	line := fmt.Sprintf("V\t%s\t\t%s\tunknown\t/CN=%s\n",
		opensslTime(cert.NotAfter),
		serial,
		cert.Subject.CommonName,
	)
	f, err := os.OpenFile(s.indexPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line)
	return err
}

// markIndexR меняет V → R для данного serial
func (s *Store) markIndexR(serial *big.Int) error {
	indexPath := s.indexPath()
	b, err := os.ReadFile(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // нет index.txt — ничего не делаем
		}
		return err
	}
	want := strings.ToUpper(serial.Text(16))
	if len(want)%2 != 0 {
		want = "0" + want
	}
	lines := strings.Split(string(b), "\n")
	changed := false
	now := opensslTime(time.Now())
	for i, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 4 {
			continue
		}
		ser := strings.ToUpper(strings.TrimSpace(parts[3]))
		if ser != want {
			continue
		}
		if parts[0] == "R" {
			return nil // уже отозван
		}
		// V  expiry  <empty>  serial  unknown  /CN=...
		// R  expiry  revokeTime  serial  unknown  /CN=...
		parts[0] = "R"
		if len(parts) == 5 {
			// редко, но на всякий
			parts = append(parts[:2], append([]string{now}, parts[2:]...)...)
		} else {
			parts[2] = now
		}
		lines[i] = strings.Join(parts, "\t")
		changed = true
		break
	}
	if !changed {
		// сертификат выдан не через index.txt — можно дописать R-строку
		// (опционально; пока просто выходим)
		return nil
	}
	out := strings.Join(lines, "\n")
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return os.WriteFile(indexPath, []byte(out), 0o644)
}

// caForCRL copies the CA so Go will sign a CRL even if the on-disk cert
// omitted KeyUsageCRLSign (common with older easy-rsa / openvpn-install CAs).
func caForCRL(ca *x509.Certificate) *x509.Certificate {
	c := *ca
	c.KeyUsage |= x509.KeyUsageCRLSign
	return &c
}

func (s *Store) loadCA() (*x509.Certificate, crypto.Signer, error) {
	crtPEM, err := os.ReadFile(s.caCrtPath())
	if err != nil {
		return nil, nil, fmt.Errorf("ca.crt: %w", err)
	}
	cert, err := parseCertPEM(crtPEM)
	if err != nil {
		return nil, nil, fmt.Errorf("ca.crt: %w", err)
	}
	keyPEM, err := os.ReadFile(s.caKeyPath())
	if err != nil {
		return nil, nil, fmt.Errorf("ca.key: %w", err)
	}
	key, err := parsePrivateKeyPEM(keyPEM)
	if err != nil {
		return nil, nil, fmt.Errorf("ca.key: %w", err)
	}
	return cert, key, nil
}

func parseCertPEM(b []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, fmt.Errorf("no PEM block")
	}
	return x509.ParseCertificate(block.Bytes)
}

func parsePrivateKeyPEM(b []byte) (crypto.Signer, error) {
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, fmt.Errorf("no PEM block")
	}
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	if k, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	switch t := parsed.(type) {
	case *rsa.PrivateKey:
		return t, nil
	case *ecdsa.PrivateKey:
		return t, nil
	case ed25519.PrivateKey:
		return t, nil
	default:
		return nil, fmt.Errorf("unsupported private key type %T", parsed)
	}
}

func encodeCert(cert *x509.Certificate) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
}

func encodeKey(key crypto.Signer) ([]byte, error) {
	switch k := key.(type) {
	case *rsa.PrivateKey:
		return pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}), nil
	case *ecdsa.PrivateKey:
		b, err := x509.MarshalECPrivateKey(k)
		if err != nil {
			return nil, err
		}
		return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: b}), nil
	default:
		b, err := x509.MarshalPKCS8PrivateKey(key)
		if err != nil {
			return nil, err
		}
		return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: b}), nil
	}
}

func (s *Store) clientCrtPath(name string) string {
	p := filepath.Join(s.issuedDir(), name+".crt")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return filepath.Join(s.Dir, name+".crt")
}

func (s *Store) clientKeyPath(name string) string {
	p := filepath.Join(s.privateDir(), name+".key")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return filepath.Join(s.Dir, name+".key")
}

// ClientsDir is where inline .ovpn profiles are written.
// /etc/openvpn/easy-rsa/pki → /etc/openvpn/clients; otherwise a sibling/clients folder.
func ClientsDir(pkiDir string) string {
	pkiDir = filepath.Clean(pkiDir)
	if filepath.Base(pkiDir) == "pki" {
		parent := filepath.Dir(pkiDir)
		if filepath.Base(parent) == "easy-rsa" {
			return filepath.Join(filepath.Dir(parent), "clients")
		}
		return filepath.Join(parent, "clients")
	}
	return filepath.Join(pkiDir, "clients")
}

func (s *Store) clientsDir() string {
	return ClientsDir(s.Dir)
}

func (s *Store) ovpnPath(name string) string {
	return filepath.Join(s.clientsDir(), name+".ovpn")
}

func (s *Store) dropOldCertCopies(name string) {
	old := filepath.Join(s.Dir, "client")
	_ = os.Remove(filepath.Join(old, name+".crt"))
	_ = os.Remove(filepath.Join(old, name+".key"))
}

// WriteOvpn builds the inline profile and writes {clients}/{name}.ovpn.
func (s *Store) WriteOvpn(name, publicHost string, cfg *ovpn.Config) error {
	name = strings.TrimSpace(name)
	if err := ValidateName(name); err != nil {
		return err
	}
	body, err := s.Profile(name, publicHost, cfg)
	if err != nil {
		return err
	}
	return s.SaveOvpn(name, body)
}

// SaveOvpn writes an already-built .ovpn into the clients folder.
func (s *Store) SaveOvpn(name string, body []byte) error {
	if err := os.MkdirAll(s.clientsDir(), 0o700); err != nil {
		return err
	}
	s.dropOldCertCopies(name)
	return os.WriteFile(s.ovpnPath(name), body, 0o600)
}

// RemoveOvpn deletes the on-disk .ovpn copy for name.
func (s *Store) RemoveOvpn(name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	_ = os.Remove(s.ovpnPath(name))
	s.dropOldCertCopies(name)
}

// SyncOvpns rewrites .ovpn files for active clients and removes copies of revoked ones.
func (s *Store) SyncOvpns(publicHost string, cfg *ovpn.Config) {
	if strings.TrimSpace(publicHost) == "" || cfg == nil {
		return
	}
	clients, err := s.List()
	if err != nil {
		return
	}
	for _, c := range clients {
		if c.Revoked || !c.HasKey {
			s.RemoveOvpn(c.Name)
			continue
		}
		_ = s.WriteOvpn(c.Name, publicHost, cfg)
	}
}

func (s *Store) revokedSerials() map[string]bool {
	out := map[string]bool{}
	b, err := os.ReadFile(s.crlPath())
	if err != nil {
		return out
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return out
	}
	crl, err := x509.ParseRevocationList(block.Bytes)
	if err != nil {
		return out
	}
	for _, e := range crl.RevokedCertificateEntries {
		out[e.SerialNumber.Text(16)] = true
	}
	return out
}

// List returns issued client certificates (not the CA).
func (s *Store) List() ([]Client, error) {
	revoked := s.revokedSerials()
	dir := s.issuedDir()
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			ents = nil
			dir = s.Dir
			ents, err = os.ReadDir(dir)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	var clients []Client
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".crt") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".crt")
		if strings.EqualFold(name, "ca") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		cert, err := parseCertPEM(raw)
		if err != nil {
			continue
		}
		if cert.IsCA {
			continue
		}
		if isProtectedCert(name, cert) {
			continue
		}
		serial := cert.SerialNumber.Text(16)
		_, hasKey := os.Stat(s.clientKeyPath(name))
		clients = append(clients, Client{
			Name:     name,
			NotAfter: cert.NotAfter.UTC(),
			Serial:   serial,
			Revoked:  revoked[serial],
			HasKey:   hasKey == nil,
			IssuedAt: cert.NotBefore.UTC(),
		})
	}
	return clients, nil
}

func isProtectedCert(name string, cert *x509.Certificate) bool {
	if strings.EqualFold(name, "server") || strings.EqualFold(name, "ca") {
		return true
	}
	for _, eku := range cert.ExtKeyUsage {
		if eku == x509.ExtKeyUsageServerAuth {
			return true
		}
	}
	return false
}

func generateClientKey(ca crypto.Signer) (crypto.Signer, error) {
	switch ca.(type) {
	case *rsa.PrivateKey:
		return rsa.GenerateKey(rand.Reader, 2048)
	case *ecdsa.PrivateKey:
		return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	case ed25519.PrivateKey:
		_, k, err := ed25519.GenerateKey(rand.Reader)
		return k, err
	default:
		return rsa.GenerateKey(rand.Reader, 2048)
	}
}

func subjectKeyID(pub crypto.PublicKey) ([]byte, error) {
	b, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return nil, err
	}
	var spki struct {
		Algo      pkix.AlgorithmIdentifier
		BitString asn1.BitString
	}
	if _, err := asn1.Unmarshal(b, &spki); err != nil {
		sum := sha1.Sum(b)
		return sum[:], nil
	}
	sum := sha1.Sum(spki.BitString.Bytes)
	return sum[:], nil
}

// Issue creates a client cert/key signed by the existing CA.
func (s *Store) Issue(name string) error {
	name = strings.TrimSpace(name)
	if err := ValidateName(name); err != nil {
		return err
	}
	if _, err := os.Stat(s.clientCrtPath(name)); err == nil {
		return fmt.Errorf("client %q already exists", name)
	}
	ca, caKey, err := s.loadCA()
	if err != nil {
		return err
	}
	key, err := generateClientKey(caKey)
	if err != nil {
		return err
	}
	serial, err := s.nextSerial()
	if err != nil {
		return err
	}
	ski, err := subjectKeyID(key.Public())
	if err != nil {
		return err
	}
	now := time.Now().Add(-time.Minute)
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: name},
		NotBefore:    now,
		NotAfter:     now.Add(clientCertDays * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		SubjectKeyId: ski,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca, key.Public(), caKey)
	if err != nil {
		return err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return err
	}
	keyPEM, err := encodeKey(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.issuedDir(), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(s.privateDir(), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(s.issuedDir(), name+".crt"), encodeCert(cert), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(s.privateDir(), name+".key"), keyPEM, 0o600); err != nil {
		return err
	}
	if err := s.appendIndexV(cert); err != nil {
		return fmt.Errorf("index.txt: %w", err)
	}
	return nil
}

func (s *Store) loadExistingCRL(ca *x509.Certificate) []x509.RevocationListEntry {
	b, err := os.ReadFile(s.crlPath())
	if err != nil {
		return nil
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return nil
	}
	crl, err := x509.ParseRevocationList(block.Bytes)
	if err != nil {
		return nil
	}
	return crl.RevokedCertificateEntries
}

func (s *Store) writeCRL(ca *x509.Certificate, caKey crypto.Signer, entries []x509.RevocationListEntry) error {
	if entries == nil {
		entries = []x509.RevocationListEntry{}
	}
	rl := &x509.RevocationList{
		Number:                    big.NewInt(time.Now().Unix()),
		ThisUpdate:                time.Now(),
		NextUpdate:                time.Now().Add(10 * 365 * 24 * time.Hour),
		RevokedCertificateEntries: entries,
	}
	der, err := x509.CreateRevocationList(rand.Reader, rl, caForCRL(ca), caKey)
	if err != nil {
		return fmt.Errorf("crl: %w", err)
	}
	pemCRL := pem.EncodeToMemory(&pem.Block{Type: "X509 CRL", Bytes: der})
	return os.WriteFile(s.crlPath(), pemCRL, 0o644)
}

// EnsureCRL writes a CRL if missing and refreshes ThisUpdate when one exists.
func (s *Store) EnsureCRL() error {
	ca, caKey, err := s.loadCA()
	if err != nil {
		return err
	}
	return s.writeCRL(ca, caKey, s.loadExistingCRL(ca))
}

// CopyCRL writes the PKI CRL to dest (the crl-verify path in server.conf).
func (s *Store) CopyCRL(dest string) error {
	dest = strings.TrimSpace(dest)
	if dest == "" {
		return nil
	}
	src, err := filepath.Abs(s.crlPath())
	if err != nil {
		src = s.crlPath()
	}
	dst, err := filepath.Abs(dest)
	if err != nil {
		dst = dest
	}
	if src == dst {
		return nil
	}
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o644)
}

// Reissue revokes the current cert (if any) and issues a new one under the same CN.
func (s *Store) Reissue(name string) error {
	name = strings.TrimSpace(name)
	if err := ValidateName(name); err != nil {
		return err
	}
	if _, err := os.Stat(s.clientCrtPath(name)); err == nil {
		if err := s.Revoke(name); err != nil {
			return err
		}
	}
	_ = os.Remove(s.clientCrtPath(name))
	_ = os.Remove(s.clientKeyPath(name))
	return s.Issue(name)
}

// Revoke adds the client cert to the CRL, marks it R in index.txt,
// and removes the private key (cert file is kept, as in easy-rsa).
func (s *Store) Revoke(name string) error {
	name = strings.TrimSpace(name)
	if err := ValidateName(name); err != nil {
		return err
	}
	ca, caKey, err := s.loadCA()
	if err != nil {
		return err
	}
	crtPath := s.clientCrtPath(name)
	raw, err := os.ReadFile(crtPath)
	if err != nil {
		return fmt.Errorf("client cert: %w", err)
	}
	cert, err := parseCertPEM(raw)
	if err != nil {
		return err
	}
	if isProtectedCert(name, cert) {
		return fmt.Errorf("cannot revoke server/CA certificate %q", name)
	}
	entries := s.loadExistingCRL(ca)
	already := false
	for _, e := range entries {
		if e.SerialNumber.Cmp(cert.SerialNumber) == 0 {
			already = true
			break
		}
	}
	if !already {
		entries = append(entries, x509.RevocationListEntry{
			SerialNumber:   cert.SerialNumber,
			RevocationTime: time.Now(),
		})
	}
	if err := s.writeCRL(ca, caKey, entries); err != nil {
		return err
	}
	if err := s.markIndexR(cert.SerialNumber); err != nil {
		return fmt.Errorf("index.txt: %w", err)
	}

	//_ = os.Remove(crtPath)  не удалять .crt (easy-rsa так делает)
	_ = os.Remove(s.clientKeyPath(name))
	_ = os.Remove(filepath.Join(s.Dir, "reqs", name+".req"))
	s.RemoveOvpn(name)
	return nil
}

// Profile builds an inline .ovpn for name.
func (s *Store) Profile(name, publicHost string, cfg *ovpn.Config) ([]byte, error) {
	name = strings.TrimSpace(name)
	if err := ValidateName(name); err != nil {
		return nil, err
	}
	caPEM, err := os.ReadFile(s.caCrtPath())
	if err != nil {
		return nil, err
	}
	crtPEM, err := os.ReadFile(s.clientCrtPath(name))
	if err != nil {
		return nil, err
	}
	keyPEM, err := os.ReadFile(s.clientKeyPath(name))
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		cfg = &ovpn.Config{Port: 1194, Proto: "udp", Cipher: "AES-256-GCM", Auth: "SHA256", Dev: "tun"}
	}
	host := strings.TrimSpace(publicHost)
	if host == "" {
		return nil, fmt.Errorf("public host is not set")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "client\n")
	fmt.Fprintf(&b, "dev %s\n", orDefault(cfg.Dev, "tun"))
	fmt.Fprintf(&b, "proto %s\n", orDefault(cfg.Proto, "udp"))
	fmt.Fprintf(&b, "remote %s %d\n", host, cfg.Port)
	b.WriteString("resolv-retry infinite\nnobind\npersist-key\npersist-tun\nremote-cert-tls server\nverb 3\n")
	if cfg.Auth != "" {
		fmt.Fprintf(&b, "auth %s\n", cfg.Auth)
	}
	if cfg.Cipher != "" {
		fmt.Fprintf(&b, "cipher %s\n", cfg.Cipher)
		fmt.Fprintf(&b, "data-ciphers %s\n", cfg.Cipher)
	}
	b.WriteString("<ca>\n")
	b.Write(normalizePEM(caPEM))
	b.WriteString("</ca>\n<cert>\n")
	b.Write(normalizePEM(crtPEM))
	b.WriteString("</cert>\n<key>\n")
	b.Write(normalizePEM(keyPEM))
	b.WriteString("</key>\n")
	if cfg.HasTLSCrypt && cfg.TLSCryptPath != "" {
		tc, err := os.ReadFile(cfg.TLSCryptPath)
		if err != nil {
			return nil, fmt.Errorf("tls-crypt: %w", err)
		}
		b.WriteString("<tls-crypt>\n")
		b.Write(normalizePEM(tc))
		b.WriteString("</tls-crypt>\n")
	} else if cfg.HasTLSAuth && cfg.TLSAuthPath != "" {
		ta, err := os.ReadFile(cfg.TLSAuthPath)
		if err != nil {
			return nil, fmt.Errorf("tls-auth: %w", err)
		}
		clientDir := 1
		if cfg.TLSAuthDir == 1 {
			clientDir = 0
		}
		fmt.Fprintf(&b, "key-direction %d\n<tls-auth>\n", clientDir)
		b.Write(normalizePEM(ta))
		b.WriteString("</tls-auth>\n")
	}
	return []byte(b.String()), nil
}

func orDefault(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return v
}

func normalizePEM(b []byte) []byte {
	s := strings.TrimSpace(string(b))
	if s == "" {
		return nil
	}
	return []byte(s + "\n")
}
