package httpserver

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mosimosi228/ovpn-dash/internal/ovpn"
	"github.com/mosimosi228/ovpn-dash/internal/pki"
	"github.com/mosimosi228/ovpn-dash/internal/settingsdb"
	"github.com/mosimosi228/ovpn-dash/internal/setup"
	"github.com/mosimosi228/ovpn-dash/internal/systemd"
)

func (h *Handler) store(r *http.Request) *pki.Store {
	s := h.loadSettings(r)
	return &pki.Store{Dir: s.PKIDir}
}

func (h *Handler) listClients(w http.ResponseWriter, r *http.Request) {
	items, err := h.store(r).List()
	if err != nil {
		items = []pki.Client{}
	}
	if items == nil {
		items = []pki.Client{}
	}
	users, _ := h.DB.ListUsers(r.Context())
	byCN := map[string]settingsdb.User{}
	for _, u := range users {
		if u.ClientName != "" {
			byCN[u.ClientName] = u
		}
	}
	out := make([]map[string]any, 0, len(items))
	seen := map[string]struct{}{}
	for _, c := range items {
		seen[c.Name] = struct{}{}
		row := map[string]any{
			"name":      c.Name,
			"not_after": c.NotAfter.UTC().Format(time.RFC3339),
			"serial":    c.Serial,
			"revoked":   c.Revoked,
			"has_key":   c.HasKey,
			"issued_at": c.IssuedAt.UTC().Format(time.RFC3339),
		}
		if u, ok := byCN[c.Name]; ok {
			row["email"] = u.Email
			row["user_id"] = u.ID
			row["disabled"] = u.Disabled
			if u.Disabled {
				row["revoked"] = true
			}
		}
		out = append(out, row)
	}
	for _, u := range users {
		if u.Role != setup.RoleUser || u.ClientName == "" {
			continue
		}
		if _, ok := seen[u.ClientName]; ok {
			continue
		}
		out = append(out, map[string]any{
			"name":      u.ClientName,
			"email":     u.Email,
			"user_id":   u.ID,
			"disabled":  u.Disabled,
			"revoked":   true,
			"has_key":   false,
			"not_after": nil,
			"serial":    "",
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *Handler) createClient(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name       string `json:"name"`
		Email      string `json:"email"`
		Password   string `json:"password"`
		ClientName string `json:"client_name"`
		Display    string `json:"display_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	display := strings.TrimSpace(req.Display)
	if display == "" {
		display = strings.TrimSpace(req.Name)
	}
	cn := strings.TrimSpace(req.ClientName)
	if cn == "" {
		cn = strings.TrimSpace(req.Name)
	}
	u, err := h.createUserWithCert(r, req.Email, display, req.Password, cn)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"name": u.ClientName,
		"user": u.Public(),
	})
}

func sanitizeClientName(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, " ", "-")
	return s
}

func (h *Handler) downloadOVPN(w http.ResponseWriter, r *http.Request) {
	name, _ := url.PathUnescape(chi.URLParam(r, "name"))
	u, _ := userFromContext(r.Context())
	if u.Role == setup.RoleUser {
		if u.ClientName == "" || u.ClientName != name || u.Disabled {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
	}
	h.writeOVPN(w, r, name)
}

func (h *Handler) myOVPN(w http.ResponseWriter, r *http.Request) {
	u, _ := userFromContext(r.Context())
	if u.ClientName == "" || u.Disabled {
		writeError(w, http.StatusForbidden, "no client certificate")
		return
	}
	h.writeOVPN(w, r, u.ClientName)
}

func (h *Handler) writeOVPN(w http.ResponseWriter, r *http.Request, name string) {
	s := h.loadSettings(r)
	cfg, err := ovpn.ParseFile(s.ServerConf)
	if err != nil {
		writeError(w, http.StatusBadGateway, "server.conf: "+err.Error())
		return
	}
	body, err := h.store(r).Profile(name, s.PublicHost, cfg)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/x-openvpn-profile")
	w.Header().Set("Content-Disposition", ovpnDisposition(name))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func ovpnDisposition(name string) string {
	star := "UTF-8''" + url.PathEscape(name) + ".ovpn"
	ascii := true
	for _, r := range name {
		if r < 0x20 || r > 0x7e || r == '"' {
			ascii = false
			break
		}
	}
	if ascii {
		return `attachment; filename="` + name + `.ovpn"; filename*=` + star
	}
	return `attachment; filename="client.ovpn"; filename*=` + star
}

func (h *Handler) publishCRL(r *http.Request) {
	s := h.loadSettings(r)
	st := h.store(r)
	_ = st.EnsureCRL()
	if cfg, err := ovpn.ParseFile(s.ServerConf); err == nil && cfg.CRLVerify != "" {
		_ = st.CopyCRL(cfg.CRLVerify)
	}
}

func (h *Handler) ensurePublishedCRL(r *http.Request) {
	s := h.loadSettings(r)
	st := h.store(r)
	if _, err := os.Stat(filepath.Join(s.PKIDir, "crl.pem")); err != nil {
		_ = st.EnsureCRL()
	}
	cfg, err := ovpn.ParseFile(s.ServerConf)
	if err != nil || cfg.CRLVerify == "" {
		return
	}
	if _, err := os.Stat(cfg.CRLVerify); err != nil {
		_ = st.EnsureCRL()
		_ = st.CopyCRL(cfg.CRLVerify)
	}
}

func (h *Handler) reissueClient(w http.ResponseWriter, r *http.Request) {
	name, _ := url.PathUnescape(chi.URLParam(r, "name"))
	s := h.loadSettings(r)
	if err := h.store(r).Reissue(name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if u, err := h.DB.GetUserByClientName(r.Context(), name); err == nil {
		u.Disabled = false
		_ = h.DB.UpdateUser(r.Context(), u)
	}
	h.publishCRL(r)
	reloadErr := ""
	if err := systemd.ReloadOrRestart(s.Unit); err != nil {
		reloadErr = err.Error()
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": name, "reload_error": reloadErr})
}

func (h *Handler) deleteClient(w http.ResponseWriter, r *http.Request) {
	name, _ := url.PathUnescape(chi.URLParam(r, "name"))
	s := h.loadSettings(r)
	if err := h.store(r).Revoke(name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if u, err := h.DB.GetUserByClientName(r.Context(), name); err == nil {
		u.Disabled = true
		_ = h.DB.UpdateUser(r.Context(), u)
	}
	h.publishCRL(r)
	cfg, _ := ovpn.ParseFile(s.ServerConf)
	warned := cfg != nil && !cfg.HasCRLVerify
	if err := systemd.ReloadOrRestart(s.Unit); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":            true,
			"reload_error":  err.Error(),
			"crl_verify":    !warned,
			"need_crl_hint": warned,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":            true,
		"crl_verify":    !warned,
		"need_crl_hint": warned,
	})
}
