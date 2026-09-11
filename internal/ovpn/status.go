package ovpn

import (
	"bufio"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Session is one connected client from the OpenVPN status file.
type Session struct {
	Name          string `json:"name"`
	RealAddress   string `json:"real_address"`
	RealIP        string `json:"real_ip"`
	VirtualIP     string `json:"virtual_ip,omitempty"`
	BytesReceived int64  `json:"bytes_received"`
	BytesSent     int64  `json:"bytes_sent"`
	Since         string `json:"since,omitempty"`
	SinceUnix     int64  `json:"since_unix,omitempty"`
	LastRef       string `json:"last_ref,omitempty"`
	ClientID      int64  `json:"client_id"`
}

// RuntimeStatusFile is the status path systemd injects for openvpn-server@instance.
func RuntimeStatusFile(unit string) string {
	unit = strings.TrimSpace(unit)
	unit = strings.TrimSuffix(unit, ".service")
	_, inst, ok := strings.Cut(unit, "@")
	if !ok || inst == "" {
		return ""
	}
	if strings.ContainsAny(inst, `/\`) {
		return ""
	}
	return filepath.Join("/run/openvpn-server", "status-"+inst+".log")
}

// StatusCandidates lists files that may hold the live client list.
func StatusCandidates(confStatus, unit string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		out = append(out, p)
	}
	add(confStatus)
	add(RuntimeStatusFile(unit))
	return out
}

// ParseBestStatus returns sessions from the most useful readable status file.
func ParseBestStatus(paths []string) ([]Session, string, error) {
	type hit struct {
		path string
		ss   []Session
		mod  time.Time
	}
	var ok []hit
	var firstErr error
	var firstPath string
	for _, p := range paths {
		ss, err := ParseStatusFile(p)
		if err != nil {
			if firstErr == nil {
				firstErr = err
				firstPath = p
			} else if errors.Is(firstErr, os.ErrNotExist) && !os.IsNotExist(err) {
				firstErr = err
				firstPath = p
			}
			continue
		}
		var mod time.Time
		if st, e := os.Stat(p); e == nil {
			mod = st.ModTime()
		}
		ok = append(ok, hit{path: p, ss: ss, mod: mod})
	}
	if len(ok) == 0 {
		return nil, firstPath, firstErr
	}
	best := ok[0]
	for _, h := range ok[1:] {
		if len(h.ss) > len(best.ss) || (len(h.ss) == len(best.ss) && h.mod.After(best.mod)) {
			best = h
		}
	}
	return best.ss, best.path, nil
}

// ParseStatusFile reads OpenVPN status log (classic or status-version 2/3).
func ParseStatusFile(path string) ([]Session, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseStatus(f)
}

// ParseStatus reads OpenVPN status output (file or management `status 3`).
func ParseStatus(r io.Reader) ([]Session, error) {
	var sessions []Session
	virt := map[string]string{}
	lastRef := map[string]string{}
	section := ""
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || line == "END" {
			continue
		}
		if strings.HasPrefix(line, "HEADER,") {
			continue
		}
		upper := strings.ToUpper(line)
		switch {
		case upper == "OPENVPN CLIENT LIST" || strings.HasPrefix(upper, "CLIENT LIST"):
			section = "clients"
			continue
		case upper == "ROUTING TABLE":
			section = "routing"
			continue
		case upper == "GLOBAL STATS":
			section = "stats"
			continue
		case strings.HasPrefix(upper, "UPDATED,") || strings.HasPrefix(upper, "TITLE,") || strings.HasPrefix(upper, "TIME,"):
			continue
		}

		if strings.HasPrefix(line, "CLIENT_LIST,") {
			if s, ok := parseClientListCSV(line); ok {
				sessions = append(sessions, s)
			}
			continue
		}
		if strings.HasPrefix(line, "ROUTING_TABLE,") {
			parts := strings.Split(line, ",")
			if len(parts) >= 3 {
				name := strings.TrimSpace(parts[2])
				virt[name] = strings.TrimSpace(parts[1])
				if len(parts) >= 5 {
					lastRef[name] = strings.TrimSpace(parts[4])
				}
			}
			continue
		}

		switch section {
		case "clients":
			if strings.HasPrefix(line, "Common Name,") {
				continue
			}
			parts := strings.Split(line, ",")
			if len(parts) < 5 {
				continue
			}
			s := Session{
				Name:        strings.TrimSpace(parts[0]),
				RealAddress: strings.TrimSpace(parts[1]),
				Since:       strings.TrimSpace(parts[4]),
			}
			s.RealIP = stripPort(s.RealAddress)
			s.BytesReceived, _ = strconv.ParseInt(strings.TrimSpace(parts[2]), 10, 64)
			s.BytesSent, _ = strconv.ParseInt(strings.TrimSpace(parts[3]), 10, 64)
			if s.Name == "" || s.Name == "UNDEF" {
				continue
			}
			sessions = append(sessions, s)
		case "routing":
			if strings.HasPrefix(line, "Virtual Address,") {
				continue
			}
			parts := strings.Split(line, ",")
			if len(parts) >= 2 {
				name := strings.TrimSpace(parts[1])
				virt[name] = strings.TrimSpace(parts[0])
				if len(parts) >= 4 {
					lastRef[name] = strings.TrimSpace(parts[3])
				}
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	for i := range sessions {
		if sessions[i].VirtualIP == "" {
			sessions[i].VirtualIP = virt[sessions[i].Name]
		}
		if sessions[i].LastRef == "" {
			sessions[i].LastRef = lastRef[sessions[i].Name]
		}
	}
	return sessions, nil
}

func parseClientListCSV(line string) (Session, bool) {
	parts := strings.Split(line, ",")
	// CLIENT_LIST,Common Name,Real Address,Virtual Address,...
	if len(parts) < 5 {
		return Session{}, false
	}
	s := Session{
		Name:        strings.TrimSpace(parts[1]),
		RealAddress: strings.TrimSpace(parts[2]),
		VirtualIP:   strings.TrimSpace(parts[3]),
	}
	s.RealIP = stripPort(s.RealAddress)
	if len(parts) > 5 {
		s.BytesReceived, _ = strconv.ParseInt(strings.TrimSpace(parts[5]), 10, 64)
	}
	if len(parts) > 6 {
		s.BytesSent, _ = strconv.ParseInt(strings.TrimSpace(parts[6]), 10, 64)
	}
	if len(parts) > 7 {
		s.Since = strings.TrimSpace(parts[7])
	}
	if len(parts) > 8 {
		s.SinceUnix, _ = strconv.ParseInt(strings.TrimSpace(parts[8]), 10, 64)
	}
	if len(parts) > 10 {
		s.ClientID, _ = strconv.ParseInt(strings.TrimSpace(parts[10]), 10, 64)
	}
	if s.Name == "" || s.Name == "UNDEF" {
		return Session{}, false
	}
	return s, true
}

func stripPort(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(addr)
	if err == nil {
		return host
	}
	return addr
}
