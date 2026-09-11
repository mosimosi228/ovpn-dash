package ovpn

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Monitor keeps one OpenVPN management connection and a live client list.
// OpenVPN accepts a single management client; polling with a new TCP socket
// each second races that slot and falls back to the 60s status file.
type Monitor struct {
	mu        sync.Mutex
	cfg       *Config
	sessions  []Session
	gotStatus bool
	stop      chan struct{}
	stopped   bool

	connMu sync.Mutex
	conn   net.Conn

	killMu sync.Mutex
	killCh chan error
}

// NewMonitor starts the reconnect loop.
func NewMonitor() *Monitor {
	m := &Monitor{stop: make(chan struct{})}
	go m.loop()
	return m
}

// Close stops the monitor.
func (m *Monitor) Close() {
	if m == nil {
		return
	}
	m.mu.Lock()
	if m.stopped {
		m.mu.Unlock()
		return
	}
	m.stopped = true
	close(m.stop)
	m.mu.Unlock()
	m.closeConn()
}

// Configure points the monitor at the current server.conf management socket.
func (m *Monitor) Configure(cfg *Config) {
	if m == nil || cfg == nil {
		return
	}
	m.mu.Lock()
	prev := m.cfg
	m.cfg = cfg
	changed := prev == nil || prev.ManagementNet != cfg.ManagementNet || prev.ManagementAddr != cfg.ManagementAddr || prev.ManagementPass != cfg.ManagementPass
	m.mu.Unlock()
	if changed {
		m.closeConn()
	}
}

func (m *Monitor) currentCfg() *Config {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cfg
}

// Sessions returns the last management snapshot. ok is true after a successful status parse.
func (m *Monitor) Sessions() ([]Session, bool) {
	if m == nil {
		return nil, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.gotStatus {
		return nil, false
	}
	out := make([]Session, len(m.sessions))
	copy(out, m.sessions)
	return out, true
}

// WaitStatus waits until the first status parse or timeout.
func (m *Monitor) WaitStatus(d time.Duration) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if _, ok := m.Sessions(); ok {
			return true
		}
		select {
		case <-m.stop:
			return false
		case <-time.After(20 * time.Millisecond):
		}
	}
	_, ok := m.Sessions()
	return ok
}

// Kill disconnects a client on the held management connection.
func (m *Monitor) Kill(realAddress, name string, clientID int64) error {
	if m == nil {
		return fmt.Errorf("nomanage")
	}
	realAddress = strings.TrimSpace(realAddress)
	name = strings.TrimSpace(name)
	matched := false
	m.mu.Lock()
	for _, s := range m.sessions {
		hit := (realAddress != "" && s.RealAddress == realAddress) || (name != "" && s.Name == name)
		if !hit && clientID != 0 && s.ClientID == clientID {
			hit = true
		}
		if !hit {
			continue
		}
		matched = true
		if name == "" {
			name = s.Name
		}
		if realAddress == "" {
			realAddress = s.RealAddress
		}
		clientID = s.ClientID
		break
	}
	m.mu.Unlock()

	var cmds []string
	if name != "" {
		if _, err := sanitizeKillTarget(name); err != nil {
			return err
		}
		cmds = append(cmds, "kill "+name+"\n")
	}
	if realAddress != "" && realAddress != name {
		if _, err := sanitizeKillTarget(realAddress); err != nil {
			return err
		}
		cmds = append(cmds, "kill "+realAddress+"\n")
	}
	if matched || clientID != 0 {
		cmds = append(cmds, fmt.Sprintf("client-kill %d\n", clientID))
	}
	if len(cmds) == 0 {
		return fmt.Errorf("client is required")
	}

	if m.conn == nil && m.currentCfg() != nil {
		time.Sleep(150 * time.Millisecond)
	}
	var last error
	for i, cmd := range cmds {
		ch := make(chan error, 1)
		m.killMu.Lock()
		m.killCh = ch
		m.killMu.Unlock()
		if err := m.write(cmd); err != nil {
			m.killMu.Lock()
			m.killCh = nil
			m.killMu.Unlock()
			return err
		}
		select {
		case err := <-ch:
			if err == nil {
				_ = m.write("status 3\n")
				return nil
			}
			last = err
			if i == len(cmds)-1 {
				return err
			}
		case <-time.After(manageTimeout):
			m.killMu.Lock()
			m.killCh = nil
			m.killMu.Unlock()
			last = fmt.Errorf("management: kill timeout")
			if i == len(cmds)-1 {
				return last
			}
		}
	}
	if last != nil {
		return last
	}
	return fmt.Errorf("management: kill failed")
}

func (m *Monitor) loop() {
	for {
		select {
		case <-m.stop:
			return
		default:
		}
		cfg := m.currentCfg()
		if cfg == nil || !cfg.CanKill() {
			time.Sleep(400 * time.Millisecond)
			continue
		}
		if err := m.serve(cfg); err != nil {
			m.mu.Lock()
			m.gotStatus = false
			m.mu.Unlock()
		}
		select {
		case <-m.stop:
			return
		case <-time.After(time.Second):
		}
	}
}

func (m *Monitor) serve(cfg *Config) error {
	conn, r, err := cfg.dialManage()
	if err != nil {
		return err
	}
	m.connMu.Lock()
	m.conn = conn
	m.connMu.Unlock()
	defer m.closeConn()

	if err := m.write("bytecount 1\n"); err != nil {
		return err
	}
	if err := m.write("status 3\n"); err != nil {
		return err
	}

	done := make(chan struct{})
	defer close(done)
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-m.stop:
				return
			case <-t.C:
				_ = m.write("status 3\n")
			}
		}
	}()

	var status strings.Builder
	inStatus := false
	for {
		select {
		case <-m.stop:
			return nil
		default:
		}
		_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		line, err := r.ReadString('\n')
		if err != nil {
			if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
				continue
			}
			return err
		}
		trim := strings.TrimSpace(line)
		up := strings.ToUpper(trim)

		if strings.HasPrefix(trim, ">BYTECOUNT_CLI:") {
			m.applyBytecount(strings.TrimPrefix(trim, ">BYTECOUNT_CLI:"))
			continue
		}
		if strings.HasPrefix(trim, ">CLIENT:") {
			_ = m.write("status 3\n")
			continue
		}
		if strings.HasPrefix(trim, ">") {
			continue
		}
		if strings.HasPrefix(up, "SUCCESS") {
			if strings.Contains(up, "BYTECOUNT") {
				continue
			}
			m.resolveKill(nil)
			continue
		}
		if strings.HasPrefix(up, "ERROR") {
			msg := strings.TrimSpace(strings.TrimPrefix(line, "ERROR:"))
			m.resolveKill(fmt.Errorf("%s", msg))
			continue
		}
		if strings.HasPrefix(trim, "TITLE,") || strings.HasPrefix(trim, "HEADER,") || strings.HasPrefix(trim, "CLIENT_LIST,") || up == "OPENVPN CLIENT LIST" || strings.HasPrefix(up, "CLIENT LIST") {
			if !inStatus {
				status.Reset()
				inStatus = true
			}
		}
		if inStatus {
			status.WriteString(line)
			if trim == "END" {
				ss, err := ParseStatus(strings.NewReader(status.String()))
				if err == nil {
					m.mu.Lock()
					m.sessions = ss
					m.gotStatus = true
					m.mu.Unlock()
				}
				inStatus = false
				status.Reset()
			}
		}
	}
}

func (m *Monitor) applyBytecount(payload string) {
	parts := strings.Split(payload, ",")
	if len(parts) < 3 {
		return
	}
	cid, _ := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
	in, _ := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
	out, _ := strconv.ParseInt(strings.TrimSpace(parts[2]), 10, 64)
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.sessions {
		if m.sessions[i].ClientID == cid {
			m.sessions[i].BytesReceived = in
			m.sessions[i].BytesSent = out
			return
		}
	}
}

func (m *Monitor) resolveKill(err error) {
	m.killMu.Lock()
	ch := m.killCh
	m.killCh = nil
	m.killMu.Unlock()
	if ch != nil {
		ch <- err
	}
}

func (m *Monitor) write(cmd string) error {
	m.connMu.Lock()
	defer m.connMu.Unlock()
	if m.conn == nil {
		return fmt.Errorf("management: not connected")
	}
	_ = m.conn.SetWriteDeadline(time.Now().Add(manageTimeout))
	_, err := m.conn.Write([]byte(cmd))
	return err
}

func (m *Monitor) closeConn() {
	m.connMu.Lock()
	c := m.conn
	m.conn = nil
	m.connMu.Unlock()
	if c != nil {
		_ = c.Close()
	}
}
