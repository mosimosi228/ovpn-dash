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
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
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
	if err := crl.CheckSignatureFrom(ca); err != nil {
		return crl.RevokedCertificateEntries
	}
	return crl.RevokedCertificateEntries
}

// Revoke adds the client cert to the CRL and removes its key/cert files.
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
	number := big.NewInt(time.Now().Unix())
	rl := &x509.RevocationList{
		Number:                    number,
		ThisUpdate:                time.Now(),
		NextUpdate:                time.Now().Add(10 * 365 * 24 * time.Hour),
		RevokedCertificateEntries: entries,
	}
	der, err := x509.CreateRevocationList(rand.Reader, rl, caForCRL(ca), caKey)
	if err != nil {
		return fmt.Errorf("crl: %w", err)
	}
	pemCRL := pem.EncodeToMemory(&pem.Block{Type: "X509 CRL", Bytes: der})
	if err := os.WriteFile(s.crlPath(), pemCRL, 0o644); err != nil {
		return err
	}
	_ = os.Remove(crtPath)
	_ = os.Remove(s.clientKeyPath(name))
	_ = os.Remove(filepath.Join(s.Dir, "reqs", name+".req"))
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
