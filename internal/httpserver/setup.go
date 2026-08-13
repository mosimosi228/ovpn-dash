package httpserver

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/mosimosi228/kit/auth"
	"github.com/mosimosi228/ovpn-dash/internal/ovpn"
	"github.com/mosimosi228/ovpn-dash/internal/setup"
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
	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password required")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}
	paths := hostPaths{
		PKIDir:     req.PKIDir,
		ServerConf: req.ServerConf,
		Unit:       req.Unit,
		LogFile:    req.LogFile,
		PublicHost: req.PublicHost,
	}
	if err := paths.validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
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
	_ = h.DB.SetMeta(ctx, setup.KeyPKIDir, paths.PKIDir)
	_ = h.DB.SetMeta(ctx, setup.KeyServerConf, paths.ServerConf)
	_ = h.DB.SetMeta(ctx, setup.KeyUnit, paths.Unit)
	_ = h.DB.SetMeta(ctx, setup.KeyLogFile, paths.LogFile)
	_ = h.DB.SetMeta(ctx, setup.KeyPublicHost, paths.PublicHost)
	_ = h.DB.SetMeta(ctx, setup.KeySetupComplete, "1")
	_ = os.Remove(filepath.Join(h.Dir, "setup.token"))
	h.apiState(w, r)
}
