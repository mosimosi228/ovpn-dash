package httpserver

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mosimosi228/ovpn-dash/internal/ovpn"
	"github.com/mosimosi228/ovpn-dash/internal/systemd"
)

// hostPaths is the operator-editable OpenVPN layout on the host.
type hostPaths struct {
	PKIDir     string
	ServerConf string
	Unit       string
	LogFile    string
	PublicHost string
}

func (p *hostPaths) normalize() {
	p.PKIDir = absPath(strings.TrimSpace(p.PKIDir))
	p.ServerConf = absPath(strings.TrimSpace(p.ServerConf))
	p.Unit = strings.TrimSpace(p.Unit)
	p.LogFile = absPath(strings.TrimSpace(p.LogFile))
	p.PublicHost = strings.TrimSpace(p.PublicHost)
}

func (p *hostPaths) validate() error {
	p.normalize()
	if p.PKIDir == "" || p.ServerConf == "" || p.Unit == "" || p.PublicHost == "" {
		return fmt.Errorf("pki_dir, server_conf, unit, public_host required")
	}
	if err := systemd.ValidateUnit(p.Unit); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(p.PKIDir, "ca.crt")); err != nil {
		return fmt.Errorf("pki_dir must contain ca.crt")
	}
	caKey := filepath.Join(p.PKIDir, "private", "ca.key")
	if _, err := os.Stat(caKey); err != nil {
		if _, err := os.Stat(filepath.Join(p.PKIDir, "ca.key")); err != nil {
			return fmt.Errorf("pki_dir must contain private/ca.key or ca.key")
		}
	}
	if !fileExists(p.ServerConf) {
		return fmt.Errorf("server.conf not found")
	}
	if _, err := ovpn.ParseFile(p.ServerConf); err != nil {
		return fmt.Errorf("cannot parse server.conf: %w", err)
	}
	return nil
}
