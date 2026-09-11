package ovpn

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"
	"unicode"
)

const manageTimeout = 4 * time.Second

type liveCache struct {
	mu  sync.Mutex
	key string
	at  time.Time
	ss  []Session
	err error
}

var liveSessionsCache liveCache

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

func (c *Config) managementPassword() (string, error) {
	if c.ManagementPass == "" {
		return "", nil
	}
	b, err := os.ReadFile(c.ManagementPass)
	if err != nil {
		return "", fmt.Errorf("management password: %w", err)
	}
	return strings.TrimSpace(string(b)), nil
}

func (c *Config) dialManage() (net.Conn, *bufio.Reader, error) {
	if !c.CanKill() {
		return nil, nil, fmt.Errorf("nomanage")
	}
	pass, err := c.managementPassword()
	if err != nil {
		return nil, nil, err
	}
	conn, err := net.DialTimeout(c.ManagementNet, c.ManagementAddr, manageTimeout)
	if err != nil {
		return nil, nil, fmt.Errorf("management: %w", err)
	}
	_ = conn.SetDeadline(time.Now().Add(manageTimeout))
	r := bufio.NewReader(conn)
	if err := readUntilReady(r, conn, pass); err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	return conn, r, nil
}

// Kill disconnects a client via the OpenVPN management interface.
// Tries common name first, then real address (IP:port).
func (c *Config) Kill(realAddress, name string) error {
	if !c.CanKill() {
		return fmt.Errorf("nomanage")
	}
	var targets []string
	if n := strings.TrimSpace(name); n != "" {
		targets = append(targets, n)
	}
	if a := strings.TrimSpace(realAddress); a != "" && a != strings.TrimSpace(name) {
		targets = append(targets, a)
	}
	if len(targets) == 0 {
		return fmt.Errorf("client is required")
	}
	var last error
	for _, target := range targets {
		target, err := sanitizeKillTarget(target)
		if err != nil {
			return err
		}
		conn, r, err := c.dialManage()
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(conn, "kill %s\n", target)
		if err == nil {
			err = readCommandResult(r)
		}
		_ = conn.Close()
		if err == nil {
			return nil
		}
		last = err
	}
	return last
}

// LiveSessions returns the current client list from the management interface.
func (c *Config) LiveSessions() ([]Session, error) {
	if !c.CanKill() {
		return nil, fmt.Errorf("nomanage")
	}
	key := c.ManagementNet + "\x00" + c.ManagementAddr
	liveSessionsCache.mu.Lock()
	if liveSessionsCache.key == key && time.Since(liveSessionsCache.at) < 300*time.Millisecond {
		ss, err := liveSessionsCache.ss, liveSessionsCache.err
		liveSessionsCache.mu.Unlock()
		return ss, err
	}
	liveSessionsCache.mu.Unlock()

	ss, err := c.fetchLiveSessions()
	liveSessionsCache.mu.Lock()
	liveSessionsCache.key = key
	liveSessionsCache.at = time.Now()
	liveSessionsCache.ss = ss
	liveSessionsCache.err = err
	liveSessionsCache.mu.Unlock()
	return ss, err
}

func (c *Config) fetchLiveSessions() ([]Session, error) {
	conn, r, err := c.dialManage()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if _, err := fmt.Fprintf(conn, "status 3\n"); err != nil {
		return nil, err
	}
	var b strings.Builder
	gotEND := false
	for i := 0; i < 20000; i++ {
		line, err := r.ReadString('\n')
		if err != nil {
			break
		}
		trim := strings.TrimSpace(line)
		up := strings.ToUpper(trim)
		if strings.HasPrefix(trim, ">") {
			continue
		}
		if strings.HasPrefix(up, "ERROR") {
			return nil, fmt.Errorf("%s", strings.TrimSpace(strings.TrimPrefix(line, "ERROR:")))
		}
		b.WriteString(line)
		if trim == "END" {
			gotEND = true
			break
		}
	}
	if b.Len() == 0 {
		return nil, fmt.Errorf("management: empty status")
	}
	ss, err := ParseStatus(strings.NewReader(b.String()))
	if err != nil {
		return nil, err
	}
	if !gotEND && len(ss) == 0 {
		return nil, fmt.Errorf("management: no status")
	}
	return ss, nil
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
