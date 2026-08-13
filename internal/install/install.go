package install

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	etcDir   = "/etc/ovpn-dash"
	binPath  = "/usr/local/bin/ovpn-dash"
	unitPath = "/etc/systemd/system/ovpn-dash.service"
	unitName = "ovpn-dash.service"
)

//go:embed ovpn-dash.service
var unitTemplate []byte

type Options struct {
	NoStart bool
}

type UninstallOptions struct {
	Purge bool
}

// Execute installs ovpn-dash as a root systemd service.
func Execute(opt Options) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("ovpn-dash install must be run as root (try: sudo ovpn-dash install)")
	}
	fmt.Println("→ installing ovpn-dash…")

	if err := os.MkdirAll(etcDir, 0o700); err != nil {
		return fmt.Errorf("mkdir %s: %w", etcDir, err)
	}
	fmt.Printf("✓ data dir: %s\n", etcDir)

	installedBin, err := installBinary()
	if err != nil {
		return err
	}
	fmt.Printf("✓ binary: %s\n", installedBin)

	unit := strings.ReplaceAll(string(unitTemplate), "__BIN__", installedBin)
	unit = strings.ReplaceAll(unit, "__DIR__", etcDir)
	if err := os.WriteFile(unitPath, []byte(unit), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", unitPath, err)
	}
	fmt.Printf("✓ unit:   %s\n", unitPath)

	if err := run("systemctl", "daemon-reload"); err != nil {
		return err
	}
	if err := run("systemctl", "enable", unitName); err != nil {
		return err
	}
	if !opt.NoStart {
		if err := run("systemctl", "restart", unitName); err != nil {
			return err
		}
		fmt.Println("✓ service: ovpn-dash.service started")
	} else {
		fmt.Println("✓ service: enabled (not started, --no-start)")
	}
	fmt.Println()
	fmt.Println("ovpn-dash is installed.")
	fmt.Println("  data:    " + etcDir)
	fmt.Println("  listen:  127.0.0.1:7474")
	fmt.Println("  status:  systemctl status ovpn-dash")
	fmt.Println("  journal: journalctl -u ovpn-dash -f")
	fmt.Println("  proxy the dashboard; do not expose :7474 to the internet.")
	return nil
}

func installBinary() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable: %w", err)
	}
	self, err = filepath.EvalSymlinks(self)
	if err != nil {
		return "", fmt.Errorf("resolve executable symlink: %w", err)
	}
	if self == binPath {
		return binPath, nil
	}
	src, err := os.Open(self)
	if err != nil {
		return "", err
	}
	defer src.Close()
	tmp := binPath + ".new"
	dst, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return "", fmt.Errorf("write %s: %w", tmp, err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		_ = os.Remove(tmp)
		return "", err
	}
	if err := dst.Close(); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	if err := os.Rename(tmp, binPath); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	return binPath, nil
}

func Uninstall(opt UninstallOptions) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("ovpn-dash uninstall must be run as root (try: sudo ovpn-dash uninstall)")
	}
	fmt.Println("→ uninstalling ovpn-dash…")
	_ = runIgnore("systemctl", "stop", unitName)
	_ = runIgnore("systemctl", "disable", unitName)
	if err := removeFile(unitPath); err != nil {
		return err
	}
	_ = runIgnore("systemctl", "daemon-reload")
	_ = runIgnore("systemctl", "reset-failed", unitName)
	if err := removeFile(binPath); err != nil {
		return err
	}
	fmt.Printf("✓ removed: %s\n", binPath)
	if opt.Purge {
		if err := os.RemoveAll(etcDir); err != nil {
			return fmt.Errorf("remove %s: %w", etcDir, err)
		}
		fmt.Printf("✓ removed: %s\n", etcDir)
	} else {
		fmt.Println("· kept data (use --purge to remove):")
		fmt.Println("    " + etcDir)
	}
	fmt.Println("ovpn-dash is uninstalled.")
	return nil
}

func removeFile(path string) error {
	err := os.Remove(path)
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	return fmt.Errorf("remove %s: %w", path, err)
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

func runIgnore(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
	return nil
}
