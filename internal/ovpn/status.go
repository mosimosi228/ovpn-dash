package ovpn

import (
	"bufio"
	"net"
	"os"
	"strconv"
	"strings"
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
}

// ParseStatusFile reads OpenVPN status log (classic or status-version 2/3).
func ParseStatusFile(path string) ([]Session, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var sessions []Session
	virt := map[string]string{}
	section := ""
	sc := bufio.NewScanner(f)
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
				virt[strings.TrimSpace(parts[2])] = strings.TrimSpace(parts[1])
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
				virt[strings.TrimSpace(parts[1])] = strings.TrimSpace(parts[0])
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
