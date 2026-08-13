package httpserver

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/mosimosi228/kit/auth"
	"github.com/mosimosi228/ovpn-dash/internal/ovpn"
	"github.com/mosimosi228/ovpn-dash/internal/setup"
	"github.com/mosimosi228/ovpn-dash/internal/systemd"
)

type stateResp struct {
	Complete   bool     `json:"complete"`
	HasAdmin   bool     `json:"has_admin"`
	AdminUser  string   `json:"admin_user,omitempty"`
	PKIDir     string   `json:"pki_dir,omitempty"`
	ServerConf string   `json:"server_conf,omitempty"`
	Unit       string   `json:"unit,omitempty"`
	LogFile    string   `json:"log_file,omitempty"`
	PublicHost string   `json:"public_host,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`
}

type setupReq struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	PKIDir     string `json:"pki_dir"`
	ServerConf string `json:"server_conf"`
	Unit       string `json:"unit"`
	LogFile    string `json:"log_file"`
	PublicHost string `json:"public_host"`
}

func (h *Handler) apiState(w http.ResponseWriter, r *http.Request) {
	hasAdmin := h.hasAdmin(r)
	if !hasAdmin && !h.allowBootstrap(r) {
		http.Error(w, "forbidden — open from localhost or pass ?setup_token=…", http.StatusForbidden)
		return
	}
	s := h.loadSettings(r)
	resp := stateResp{
		Complete: s.Complete,
		HasAdmin: hasAdmin,
	}
	if hasAdmin {
		resp.AdminUser = s.AdminUser
		resp.PKIDir = s.PKIDir
		resp.ServerConf = s.ServerConf
		resp.Unit = s.Unit
		resp.LogFile = s.LogFile
		resp.PublicHost = s.PublicHost
		if cfg, err := ovpn.ParseFile(s.ServerConf); err == nil {
			resp.Warnings = cfg.Warnings()
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) apiSetup(w http.ResponseWriter, r *http.Request) {
	if h.hasAdmin(r) {
		writeError(w, http.StatusConflict, "already configured")
		return
	}
	if !h.allowBootstrap(r) {
		http.Error(w, "forbidden — open from localhost or pass ?setup_token=…", http.StatusForbidden)
		return
	}
	var req setupReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.PKIDir = absPath(strings.TrimSpace(req.PKIDir))
	req.ServerConf = absPath(strings.TrimSpace(req.ServerConf))
	req.Unit = strings.TrimSpace(req.Unit)
	req.LogFile = absPath(strings.TrimSpace(req.LogFile))
	req.PublicHost = strings.TrimSpace(req.PublicHost)
	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password required")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}
	if req.PKIDir == "" || req.ServerConf == "" || req.Unit == "" || req.PublicHost == "" {
		writeError(w, http.StatusBadRequest, "pki_dir, server_conf, unit, public_host required")
		return
	}
	if err := systemd.ValidateUnit(req.Unit); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := os.Stat(filepath.Join(req.PKIDir, "ca.crt")); err != nil {
		writeError(w, http.StatusBadRequest, "pki_dir must contain ca.crt")
		return
	}
	caKey := filepath.Join(req.PKIDir, "private", "ca.key")
	if _, err := os.Stat(caKey); err != nil {
		caKey = filepath.Join(req.PKIDir, "ca.key")
		if _, err := os.Stat(caKey); err != nil {
			writeError(w, http.StatusBadRequest, "pki_dir must contain private/ca.key or ca.key")
			return
		}
	}
	if !fileExists(req.ServerConf) {
		writeError(w, http.StatusBadRequest, "server.conf not found")
		return
	}
	if _, err := ovpn.ParseFile(req.ServerConf); err != nil {
		writeError(w, http.StatusBadRequest, "cannot parse server.conf: "+err.Error())
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "hash error")
		return
	}
	ctx := r.Context()
	_ = h.DB.SetMeta(ctx, setup.KeyAdminUser, req.Username)
	_ = h.DB.SetMeta(ctx, setup.KeyAdminPassHash, hash)
	_ = h.DB.SetMeta(ctx, setup.KeyPKIDir, req.PKIDir)
	_ = h.DB.SetMeta(ctx, setup.KeyServerConf, req.ServerConf)
	_ = h.DB.SetMeta(ctx, setup.KeyUnit, req.Unit)
	_ = h.DB.SetMeta(ctx, setup.KeyLogFile, req.LogFile)
	_ = h.DB.SetMeta(ctx, setup.KeyPublicHost, req.PublicHost)
	_ = h.DB.SetMeta(ctx, setup.KeySetupComplete, "1")
	_ = os.Remove(filepath.Join(h.Dir, "setup.token"))
	h.apiState(w, r)
}
