package ovpn

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
	"unicode"
)

const manageTimeout = 4 * time.Second

// CanKill reports whether server.conf has a management interface we can dial.
func (c *Config) CanKill() bool {
	return c != nil && c.ManagementNet != "" && c.ManagementAddr != ""
}

func sanitizeKillTarget(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("client is required")
	}
	for _, r := range s {
		if r == '\n' || r == '\r' || r == 0 || !unicode.IsPrint(r) {
			return "", fmt.Errorf("invalid client")
		}
	}
	if strings.ContainsAny(s, "\"'") {
		return "", fmt.Errorf("invalid client")
	}
	return s, nil
}

// Kill disconnects a client via the OpenVPN management interface.
// Prefer realAddress (IP:port); fall back to common name.
func (c *Config) Kill(realAddress, name string) error {
	if !c.CanKill() {
		return fmt.Errorf("nomanage")
	}
	target := strings.TrimSpace(realAddress)
	if target == "" {
		target = strings.TrimSpace(name)
	}
	target, err := sanitizeKillTarget(target)
	if err != nil {
		return err
	}
	pass := ""
	if c.ManagementPass != "" {
		b, err := os.ReadFile(c.ManagementPass)
		if err != nil {
			return fmt.Errorf("management password: %w", err)
		}
		pass = strings.TrimSpace(string(b))
	}
	conn, err := net.DialTimeout(c.ManagementNet, c.ManagementAddr, manageTimeout)
	if err != nil {
		return fmt.Errorf("management: %w", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(manageTimeout))

	r := bufio.NewReader(conn)
	if err := readUntilReady(r, conn, pass); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(conn, "kill %s\n", target); err != nil {
		return err
	}
	return readCommandResult(r)
}

func readUntilReady(r *bufio.Reader, conn net.Conn, pass string) error {
	for i := 0; i < 32; i++ {
		line, err := r.ReadString('\n')
		if err != nil {
			return fmt.Errorf("management: %w", err)
		}
		line = strings.TrimSpace(line)
		up := strings.ToUpper(line)
		if strings.Contains(up, "ENTER PASSWORD") {
			if pass == "" {
				return fmt.Errorf("management password required")
			}
			if _, err := fmt.Fprintf(conn, "%s\n", pass); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(up, "ERROR") {
			return fmt.Errorf("%s", strings.TrimSpace(strings.TrimPrefix(line, "ERROR:")))
		}
		if strings.HasPrefix(line, ">INFO") || strings.HasPrefix(up, "SUCCESS") {
			return nil
		}
	}
	return fmt.Errorf("management: no greeting")
}

func readCommandResult(r *bufio.Reader) error {
	for i := 0; i < 32; i++ {
		line, err := r.ReadString('\n')
		if err != nil {
			return fmt.Errorf("management: %w", err)
		}
		line = strings.TrimSpace(line)
		up := strings.ToUpper(line)
		if strings.HasPrefix(up, "SUCCESS") {
			return nil
		}
		if strings.HasPrefix(up, "ERROR") {
			msg := strings.TrimSpace(strings.TrimPrefix(line, "ERROR:"))
			msg = strings.TrimSpace(strings.TrimPrefix(msg, "error:"))
			if msg == "" {
				msg = line
			}
			return fmt.Errorf("%s", msg)
		}
	}
	return fmt.Errorf("management: no reply")
}
