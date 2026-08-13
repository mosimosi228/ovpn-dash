package httpserver

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"
	"github.com/mosimosi228/ovpn-dash/internal/ovpn"
	"github.com/mosimosi228/ovpn-dash/internal/pki"
	"github.com/mosimosi228/ovpn-dash/internal/systemd"
)

func (h *Handler) store(r *http.Request) *pki.Store {
	s := h.loadSettings(r)
	return &pki.Store{Dir: s.PKIDir}
}

func (h *Handler) listClients(w http.ResponseWriter, r *http.Request) {
	items, err := h.store(r).List()
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if items == nil {
		items = []pki.Client{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) createClient(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := h.store(r).Issue(req.Name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"name": req.Name})
}

func (h *Handler) downloadOVPN(w http.ResponseWriter, r *http.Request) {
	name, _ := url.PathUnescape(chi.URLParam(r, "name"))
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
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`.ovpn"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (h *Handler) deleteClient(w http.ResponseWriter, r *http.Request) {
	name, _ := url.PathUnescape(chi.URLParam(r, "name"))
	s := h.loadSettings(r)
	if err := h.store(r).Revoke(name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
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
