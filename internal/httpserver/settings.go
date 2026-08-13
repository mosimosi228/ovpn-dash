package httpserver

import (
	"net/http"

	"github.com/mosimosi228/kit/auth"
	"github.com/mosimosi228/ovpn-dash/internal/ovpn"
	"github.com/mosimosi228/ovpn-dash/internal/setup"
)

type settingsPatchReq struct {
	PKIDir          string `json:"pki_dir"`
	ServerConf      string `json:"server_conf"`
	Unit            string `json:"unit"`
	LogFile         string `json:"log_file"`
	PublicHost      string `json:"public_host"`
	CurrentPassword string `json:"current_password"`
	Password        string `json:"password"`
}

func (h *Handler) settingsDTO(r *http.Request) map[string]any {
	s := h.loadSettings(r)
	resp := map[string]any{
		"admin_user":  s.AdminUser,
		"pki_dir":     s.PKIDir,
		"server_conf": s.ServerConf,
		"unit":        s.Unit,
		"log_file":    s.LogFile,
		"public_host": s.PublicHost,
	}
	if cfg, err := ovpn.ParseFile(s.ServerConf); err == nil {
		resp["warnings"] = cfg.Warnings()
	}
	return resp
}

func (h *Handler) getSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.settingsDTO(r))
}

func (h *Handler) patchSettings(w http.ResponseWriter, r *http.Request) {
	var req settingsPatchReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	paths := hostPaths{
		PKIDir:     req.PKIDir,
		ServerConf: req.ServerConf,
		Unit:       req.Unit,
		LogFile:    req.LogFile,
		PublicHost: req.PublicHost,
	}
	if req.Password != "" {
		if len(req.Password) < 8 {
			writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
			return
		}
		hash, _ := h.DB.GetMeta(r.Context(), setup.KeyAdminPassHash)
		if auth.CheckPassword(hash, req.CurrentPassword) != nil {
			writeError(w, http.StatusBadRequest, "current password is incorrect")
			return
		}
	}
	if err := paths.validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx := r.Context()
	if err := h.DB.SetMeta(ctx, setup.KeyPKIDir, paths.PKIDir); err != nil {
		writeError(w, http.StatusInternalServerError, "save error")
		return
	}
	_ = h.DB.SetMeta(ctx, setup.KeyServerConf, paths.ServerConf)
	_ = h.DB.SetMeta(ctx, setup.KeyUnit, paths.Unit)
	_ = h.DB.SetMeta(ctx, setup.KeyLogFile, paths.LogFile)
	_ = h.DB.SetMeta(ctx, setup.KeyPublicHost, paths.PublicHost)

	if req.Password != "" {
		next, err := auth.HashPassword(req.Password)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "hash error")
			return
		}
		if err := h.DB.SetMeta(ctx, setup.KeyAdminPassHash, next); err != nil {
			writeError(w, http.StatusInternalServerError, "save error")
			return
		}
	}
	writeJSON(w, http.StatusOK, h.settingsDTO(r))
}
