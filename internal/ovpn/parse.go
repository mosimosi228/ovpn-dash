package ovpn

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config is the subset of server.conf needed to build client profiles and warn the operator.
type Config struct {
	Port         int
	Proto        string
	Cipher       string
	Auth         string
	TLSCryptPath string
	TLSAuthPath  string
	TLSAuthDir   int
	CRLVerify    string
	StatusFile   string
	LogFile      string
	Dev          string
	HasTLSCrypt  bool
	HasTLSAuth   bool
	HasCRLVerify bool
}

// ParseFile reads an OpenVPN server.conf.
func ParseFile(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cfg := &Config{
		Port:  1194,
		Proto: "udp",
		Dev:   "tun",
		Auth:  "SHA256",
	}
	dir := filepath.Dir(path)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	inInline := ""
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if inInline != "" {
			if strings.HasPrefix(line, "</") {
				inInline = ""
			}
			continue
		}
		if strings.HasPrefix(line, "<") && strings.HasSuffix(line, ">") && !strings.HasPrefix(line, "</") {
			inInline = strings.TrimSuffix(strings.TrimPrefix(line, "<"), ">")
			continue
		}
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		key, val, _ := strings.Cut(line, " ")
		key = strings.ToLower(strings.TrimSpace(key))
		val = strings.TrimSpace(val)
		switch key {
		case "port":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				cfg.Port = n
			}
		case "proto":
			cfg.Proto = strings.ToLower(val)
			cfg.Proto = strings.TrimSuffix(cfg.Proto, "4")
			cfg.Proto = strings.TrimSuffix(cfg.Proto, "6")
		case "dev":
			cfg.Dev = val
		case "cipher":
			if cfg.Cipher == "" {
				cfg.Cipher = val
			}
		case "data-ciphers", "ncp-ciphers":
			first, _, _ := strings.Cut(val, ":")
			if first != "" {
				cfg.Cipher = first
			}
		case "auth":
			cfg.Auth = val
		case "tls-crypt":
			cfg.TLSCryptPath = resolve(dir, firstArg(val))
			cfg.HasTLSCrypt = cfg.TLSCryptPath != ""
		case "tls-auth":
			parts := strings.Fields(val)
			if len(parts) > 0 {
				cfg.TLSAuthPath = resolve(dir, parts[0])
				cfg.HasTLSAuth = cfg.TLSAuthPath != ""
			}
			if len(parts) > 1 {
				if n, err := strconv.Atoi(parts[1]); err == nil {
					cfg.TLSAuthDir = n
				}
			}
		case "crl-verify":
			cfg.CRLVerify = resolve(dir, firstArg(val))
			cfg.HasCRLVerify = cfg.CRLVerify != ""
		case "status":
			cfg.StatusFile = resolve(dir, firstArg(val))
		case "log", "log-append":
			if p := firstArg(val); p != "" {
				cfg.LogFile = resolve(dir, p)
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if cfg.Cipher == "" {
		cfg.Cipher = "AES-256-GCM"
	}
	return cfg, nil
}

func firstArg(val string) string {
	f := strings.Fields(val)
	if len(f) == 0 {
		return ""
	}
	return f[0]
}

func resolve(dir, p string) string {
	if p == "" {
		return ""
	}
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(dir, p)
}

// Warnings returns operator-facing issues for the UI.
func (c *Config) Warnings() []string {
	if c == nil {
		return nil
	}
	var out []string
	if !c.HasCRLVerify {
		out = append(out, "crl-verify")
	}
	if !c.HasTLSCrypt && !c.HasTLSAuth {
		out = append(out, "tls-crypt")
	}
	return out
}
