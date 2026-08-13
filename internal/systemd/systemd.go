package systemd

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"unicode"
)

// ValidateUnit rejects names that cannot be a systemd unit.
func ValidateUnit(name string) error {
	if name == "" {
		return fmt.Errorf("empty unit name")
	}
	if len(name) > 256 {
		return fmt.Errorf("unit name too long")
	}
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '@' || r == '.' || r == '_' || r == '-' || r == ':' {
			continue
		}
		return fmt.Errorf("invalid unit name %q", name)
	}
	return nil
}

func run(args ...string) (string, error) {
	cmd := exec.Command("systemctl", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := strings.TrimSpace(stdout.String())
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return out, fmt.Errorf("systemctl %s: %s", strings.Join(args, " "), msg)
	}
	return out, nil
}

// IsActive reports whether the unit is active.
func IsActive(unit string) (bool, string, error) {
	if err := ValidateUnit(unit); err != nil {
		return false, "", err
	}
	out, err := run("is-active", unit)
	state := strings.TrimSpace(out)
	if state == "" {
		state = "unknown"
	}
	if err != nil {
		// systemctl is-active returns non-zero for inactive
		return false, state, nil
	}
	return state == "active", state, nil
}

// Start starts the unit.
func Start(unit string) error {
	if err := ValidateUnit(unit); err != nil {
		return err
	}
	_, err := run("start", unit)
	return err
}

// Stop stops the unit.
func Stop(unit string) error {
	if err := ValidateUnit(unit); err != nil {
		return err
	}
	_, err := run("stop", unit)
	return err
}

// ReloadOrRestart reloads the unit so OpenVPN re-reads the CRL.
func ReloadOrRestart(unit string) error {
	if err := ValidateUnit(unit); err != nil {
		return err
	}
	if _, err := run("reload-or-restart", unit); err == nil {
		return nil
	}
	_, err := run("restart", unit)
	return err
}

// UnitLog returns recent journal lines for the unit (empty if journalctl is unavailable).
func UnitLog(unit string, lines int) (string, error) {
	if err := ValidateUnit(unit); err != nil {
		return "", err
	}
	if lines < 1 {
		lines = 200
	}
	if lines > 2000 {
		lines = 2000
	}
	cmd := exec.Command("journalctl", "-u", unit, "-n", strconv.Itoa(lines), "--no-pager", "-o", "short-iso")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("journalctl: %s", msg)
	}
	return strings.TrimSpace(stdout.String()), nil
}
