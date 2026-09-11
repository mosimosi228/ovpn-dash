package httpserver

import (
	"net/http"
	"strings"

	"github.com/mosimosi228/ovpn-dash/internal/ovpn"
	"github.com/mosimosi228/ovpn-dash/internal/setup"
	"github.com/mosimosi228/ovpn-dash/internal/telegram"
)

type settingsPatchReq struct {
	PKIDir     string `json:"pki_dir"`
	ServerConf string `json:"server_conf"`
	Unit       string `json:"unit"`
	LogFile    string `json:"log_file"`
	PublicHost string `json:"public_host"`

	SMTPHost *string `json:"smtp_host"`
	SMTPPort *string `json:"smtp_port"`
	SMTPUser *string `json:"smtp_user"`
	SMTPPass *string `json:"smtp_pass"`
	SMTPFrom *string `json:"smtp_from"`
	SMTPTLS  *bool   `json:"smtp_tls"`

	TelegramBotToken *string `json:"telegram_bot_token"`
}

func (h *Handler) settingsDTO(r *http.Request) map[string]any {
	s := h.loadSettings(r)
	resp := map[string]any{
		"pki_dir":               s.PKIDir,
		"server_conf":           s.ServerConf,
		"unit":                  s.Unit,
		"log_file":              s.LogFile,
		"public_host":           s.PublicHost,
		"smtp_host":             s.SMTPHost,
		"smtp_port":             s.SMTPPort,
		"smtp_user":             s.SMTPUser,
		"smtp_from":             s.SMTPFrom,
		"smtp_tls":              s.SMTPTLS,
		"smtp_pass_set":         s.SMTPPass != "",
		"smtp_configured":       s.SMTPHost != "" && s.SMTPFrom != "",
		"telegram_configured":   s.TelegramBotToken != "",
		"telegram_token_set":    s.TelegramBotToken != "",
		"telegram_bot_username": s.TelegramBotName,
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

	h.syncClientOvpns(r)

	if req.SMTPHost != nil {
		_ = h.DB.SetMeta(ctx, setup.KeySMTPHost, strings.TrimSpace(*req.SMTPHost))
	}
	if req.SMTPPort != nil {
		_ = h.DB.SetMeta(ctx, setup.KeySMTPPort, strings.TrimSpace(*req.SMTPPort))
	}
	if req.SMTPUser != nil {
		_ = h.DB.SetMeta(ctx, setup.KeySMTPUser, strings.TrimSpace(*req.SMTPUser))
	}
	if req.SMTPPass != nil {
		_ = h.DB.SetMeta(ctx, setup.KeySMTPPass, *req.SMTPPass)
	}
	if req.SMTPFrom != nil {
		_ = h.DB.SetMeta(ctx, setup.KeySMTPFrom, strings.TrimSpace(*req.SMTPFrom))
	}
	if req.SMTPTLS != nil {
		v := "0"
		if *req.SMTPTLS {
			v = "1"
		}
		_ = h.DB.SetMeta(ctx, setup.KeySMTPTLS, v)
	}
	if req.TelegramBotToken != nil {
		tok := strings.TrimSpace(*req.TelegramBotToken)
		_ = h.DB.SetMeta(ctx, setup.KeyTelegramBotToken, tok)
		if tok == "" {
			_ = h.DB.SetMeta(ctx, setup.KeyTelegramBotName, "")
		} else if uname, err := telegram.GetMe(tok); err == nil {
			_ = h.DB.SetMeta(ctx, setup.KeyTelegramBotName, uname)
		}
	}
	writeJSON(w, http.StatusOK, h.settingsDTO(r))
}
