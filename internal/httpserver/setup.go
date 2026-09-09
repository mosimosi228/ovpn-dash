package httpserver

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/mosimosi228/kit/auth"
	"github.com/mosimosi228/ovpn-dash/internal/settingsdb"
	"github.com/mosimosi228/ovpn-dash/internal/setup"
	"github.com/mosimosi228/ovpn-dash/internal/telegram"
)

type stateResp struct {
	Complete            bool   `json:"complete"`
	HasAdmin            bool   `json:"has_admin"`
	PKIDir              string `json:"pki_dir,omitempty"`
	ServerConf          string `json:"server_conf,omitempty"`
	Unit                string `json:"unit,omitempty"`
	LogFile             string `json:"log_file,omitempty"`
	PublicHost          string `json:"public_host,omitempty"`
	SMTPConfigured      bool   `json:"smtp_configured"`
	TelegramConfigured  bool   `json:"telegram_configured"`
	TelegramBotUsername string `json:"telegram_bot_username,omitempty"`
}

type setupReq struct {
	Email      string `json:"email"`
	Username   string `json:"username"`
	Name       string `json:"name"`
	Password   string `json:"password"`
	PKIDir     string `json:"pki_dir"`
	ServerConf string `json:"server_conf"`
	Unit       string `json:"unit"`
	LogFile    string `json:"log_file"`
	PublicHost string `json:"public_host"`

	SMTPHost string `json:"smtp_host"`
	SMTPPort string `json:"smtp_port"`
	SMTPUser string `json:"smtp_user"`
	SMTPPass string `json:"smtp_pass"`
	SMTPFrom string `json:"smtp_from"`
	SMTPTLS  *bool  `json:"smtp_tls"`

	TelegramBotToken string `json:"telegram_bot_token"`
}

func (h *Handler) apiState(w http.ResponseWriter, r *http.Request) {
	hasAdmin := h.hasAdmin(r)
	if !hasAdmin && !h.allowBootstrap(r) {
		http.Error(w, "forbidden — open from localhost or pass ?setup_token=…", http.StatusForbidden)
		return
	}
	s := h.loadSettings(r)
	resp := stateResp{
		Complete:            s.Complete,
		HasAdmin:            hasAdmin,
		SMTPConfigured:      s.SMTPHost != "" && s.SMTPFrom != "",
		TelegramConfigured:  s.TelegramBotToken != "",
		TelegramBotUsername: s.TelegramBotName,
	}
	if !hasAdmin {
		resp.PKIDir = setup.DefaultPKIDir
		resp.ServerConf = setup.DefaultServerConf
		resp.Unit = setup.DefaultUnit
		resp.LogFile = setup.DefaultLogFile
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
	email := normalizeEmail(req.Email)
	if email == "" {
		email = normalizeEmail(req.Username)
	}
	if !validEmail(email) || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password required")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = strings.Split(email, "@")[0]
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
	if _, err := h.DB.InsertUser(ctx, settingsdb.User{
		Email:    email,
		Name:     name,
		PassHash: hash,
		Role:     setup.RoleRoot,
		Theme:    setup.ThemeLight,
		MapStyle: setup.MapAuto,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "save error")
		return
	}
	_ = h.DB.SetMeta(ctx, setup.KeyPKIDir, paths.PKIDir)
	_ = h.DB.SetMeta(ctx, setup.KeyServerConf, paths.ServerConf)
	_ = h.DB.SetMeta(ctx, setup.KeyUnit, paths.Unit)
	_ = h.DB.SetMeta(ctx, setup.KeyLogFile, paths.LogFile)
	_ = h.DB.SetMeta(ctx, setup.KeyPublicHost, paths.PublicHost)
	_ = h.DB.SetMeta(ctx, setup.KeySMTPHost, strings.TrimSpace(req.SMTPHost))
	_ = h.DB.SetMeta(ctx, setup.KeySMTPPort, strings.TrimSpace(req.SMTPPort))
	_ = h.DB.SetMeta(ctx, setup.KeySMTPUser, strings.TrimSpace(req.SMTPUser))
	_ = h.DB.SetMeta(ctx, setup.KeySMTPPass, req.SMTPPass)
	from := strings.TrimSpace(req.SMTPFrom)
	if from == "" {
		from = strings.TrimSpace(req.SMTPUser)
	}
	_ = h.DB.SetMeta(ctx, setup.KeySMTPFrom, from)
	tlsOn := "1"
	if req.SMTPTLS != nil && !*req.SMTPTLS {
		tlsOn = "0"
	}
	_ = h.DB.SetMeta(ctx, setup.KeySMTPTLS, tlsOn)
	tok := strings.TrimSpace(req.TelegramBotToken)
	_ = h.DB.SetMeta(ctx, setup.KeyTelegramBotToken, tok)
	if tok != "" {
		if uname, err := telegram.GetMe(tok); err == nil {
			_ = h.DB.SetMeta(ctx, setup.KeyTelegramBotName, uname)
		}
	}
	_ = h.DB.SetMeta(ctx, setup.KeySetupComplete, "1")
	_ = os.Remove(filepath.Join(h.Dir, "setup.token"))
	h.apiState(w, r)
}

func (h *Handler) hasAdmin(r *http.Request) bool {
	n, err := h.DB.CountUsers(r.Context())
	return err == nil && n > 0
}
