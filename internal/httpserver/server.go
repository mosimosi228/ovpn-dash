package httpserver

import (
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/mosimosi228/ovpn-dash/internal/ovpn"
	"github.com/mosimosi228/ovpn-dash/internal/systemd"
)

func (h *Handler) serverStatus(w http.ResponseWriter, r *http.Request) {
	s := h.loadSettings(r)
	active, state, err := systemd.IsActive(s.Unit)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	resp := map[string]any{
		"active":      active,
		"unit":        s.Unit,
		"unit_state":  state,
		"pki_dir":     s.PKIDir,
		"server_conf": s.ServerConf,
		"log_file":    s.LogFile,
		"public_host": s.PublicHost,
	}
	if cfg, err := ovpn.ParseFile(s.ServerConf); err == nil {
		resp["port"] = cfg.Port
		resp["proto"] = cfg.Proto
		resp["cipher"] = cfg.Cipher
		resp["has_tls_crypt"] = cfg.HasTLSCrypt
		resp["has_tls_auth"] = cfg.HasTLSAuth
		resp["has_crl_verify"] = cfg.HasCRLVerify
		resp["warnings"] = cfg.Warnings()
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) serverStart(w http.ResponseWriter, r *http.Request) {
	s := h.loadSettings(r)
	if err := systemd.Start(s.Unit); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	h.serverStatus(w, r)
}

func (h *Handler) serverStop(w http.ResponseWriter, r *http.Request) {
	s := h.loadSettings(r)
	if err := systemd.Stop(s.Unit); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	h.serverStatus(w, r)
}

func (h *Handler) serverLog(w http.ResponseWriter, r *http.Request) {
	s := h.loadSettings(r)
	if s.LogFile == "" {
		writeError(w, http.StatusBadRequest, "log file is not configured")
		return
	}
	n := 200
	if q := r.URL.Query().Get("lines"); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v > 0 && v <= 2000 {
			n = v
		}
	}
	text, err := tailFile(s.LogFile, n)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": s.LogFile, "text": text})
}

func tailFile(path string, lines int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", err
	}
	const maxRead = 512 * 1024
	size := st.Size()
	start := int64(0)
	if size > maxRead {
		start = size - maxRead
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return "", err
	}
	b, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}
	all := string(b)
	parts := splitLines(all)
	if len(parts) > lines {
		parts = parts[len(parts)-lines:]
	}
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += "\n"
		}
		out += p
	}
	return out, nil
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			lines = append(lines, line)
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
