package httpserver

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mosimosi228/kit/auth"
	"github.com/mosimosi228/ovpn-dash/internal/mailer"
	"github.com/mosimosi228/ovpn-dash/internal/settingsdb"
	"github.com/mosimosi228/ovpn-dash/internal/settingsdb/sqlitedb"
	"github.com/mosimosi228/ovpn-dash/internal/setup"
	"github.com/mosimosi228/ovpn-dash/internal/telegram"
)

const pinTTL = 3 * time.Minute
const recoveryTTL = time.Hour

type loginReq struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type refreshReq struct {
	RefreshToken string `json:"refresh_token"`
}

type patchMeReq struct {
	Name            string `json:"name"`
	Email           string `json:"email"`
	CurrentPassword string `json:"current_password"`
	Password        string `json:"password"`
	Theme           string `json:"theme"`
	MapStyle        string `json:"map_style"`
}

type emailReq struct {
	Email string `json:"email"`
}

type pinVerifyReq struct {
	Email string `json:"email"`
	PIN   string `json:"pin"`
}

type resetReq struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	email := normalizeEmail(req.Email)
	if email == "" {
		email = normalizeEmail(req.Username)
	}
	u, err := h.DB.FindLogin(r.Context(), email)
	if err != nil || u.Disabled || auth.CheckPassword(u.PassHash, req.Password) != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	h.renderTokens(w, u)
}

func (h *Handler) loginPIN(w http.ResponseWriter, r *http.Request) {
	var req emailReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	u, err := h.DB.FindLogin(r.Context(), normalizeEmail(req.Email))
	if err != nil || u.Disabled {
		writeError(w, http.StatusBadRequest, "cannot send PIN")
		return
	}
	if u.TelegramChatID == "" {
		writeError(w, http.StatusBadRequest, "telegram is not bound")
		return
	}
	token, _ := h.DB.GetMeta(r.Context(), setup.KeyTelegramBotToken)
	if token == "" {
		writeError(w, http.StatusBadRequest, "telegram is not configured")
		return
	}
	pin, err := randomDigits(6)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "pin error")
		return
	}
	now := time.Now()
	exp := now.Add(pinTTL).Unix()
	_ = h.DB.Q.DeletePinChallenges(r.Context(), u.ID)
	if err := h.DB.Q.InsertPinChallenge(r.Context(), sqlitedb.InsertPinChallengeParams{
		UserID:    u.ID,
		PinHash:   hashOpaque(pin),
		ExpiresAt: exp,
		CreatedAt: now.Unix(),
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "pin error")
		return
	}
	text := "Your OVPN Dashboard PIN: " + pin + "\nValid for 3 minutes."
	if err := h.sendTelegram(token, u.TelegramChatID, text); err != nil {
		writeError(w, http.StatusBadGateway, "could not send PIN")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"expires_at": exp,
		"ttl_sec":    int(pinTTL.Seconds()),
	})
}

func (h *Handler) verifyPIN(w http.ResponseWriter, r *http.Request) {
	var req pinVerifyReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	u, err := h.DB.FindLogin(r.Context(), normalizeEmail(req.Email))
	if err != nil || u.Disabled {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	ch, err := h.DB.Q.GetPinChallenge(r.Context(), u.ID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if time.Now().Unix() > ch.ExpiresAt {
		_ = h.DB.Q.DeletePinChallenges(r.Context(), u.ID)
		writeError(w, http.StatusUnauthorized, "PIN expired")
		return
	}
	want := hashOpaque(strings.TrimSpace(req.PIN))
	if subtle.ConstantTimeCompare([]byte(want), []byte(ch.PinHash)) != 1 {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	_ = h.DB.Q.DeletePinChallenges(r.Context(), u.ID)
	h.renderTokens(w, u)
}

func (h *Handler) forgotPassword(w http.ResponseWriter, r *http.Request) {
	var req emailReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	cfg := h.smtpConfig(r)
	if !cfg.Configured() {
		writeError(w, http.StatusBadRequest, "password recovery is not configured")
		return
	}
	u, err := h.DB.FindLogin(r.Context(), normalizeEmail(req.Email))
	if err != nil || u.Disabled {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	raw, err := randomHex(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token error")
		return
	}
	_ = h.DB.Q.DeleteRecoveryTokensForUser(r.Context(), u.ID)
	if err := h.DB.Q.InsertRecoveryToken(r.Context(), sqlitedb.InsertRecoveryTokenParams{
		UserID:    u.ID,
		TokenHash: hashOpaque(raw),
		ExpiresAt: time.Now().Add(recoveryTTL).Unix(),
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "token error")
		return
	}
	origin := publicOrigin(r.Host, r.Header.Get("X-Forwarded-Proto"), r.Header.Get("X-Forwarded-Host"))
	if origin == "" {
		s := h.loadSettings(r)
		if s.PublicHost != "" {
			origin = "https://" + s.PublicHost
		}
	}
	link := origin + "/dashboard/?reset=" + raw
	body := "Reset your OVPN Dashboard password:\n\n" + link + "\n\nThis link expires in 1 hour."
	if err := h.sendMail(cfg, u.Email, "OVPN Dashboard password reset", body); err != nil {
		writeError(w, http.StatusBadGateway, "could not send email")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) resetPassword(w http.ResponseWriter, r *http.Request) {
	var req resetReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}
	row, err := h.DB.Q.GetRecoveryToken(r.Context(), hashOpaque(strings.TrimSpace(req.Token)))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid or expired token")
		return
	}
	if time.Now().Unix() > row.ExpiresAt {
		_ = h.DB.Q.DeleteRecoveryToken(r.Context(), row.ID)
		writeError(w, http.StatusBadRequest, "invalid or expired token")
		return
	}
	u, err := h.DB.GetUserByID(r.Context(), row.UserID)
	if err != nil || u.Disabled {
		writeError(w, http.StatusBadRequest, "invalid or expired token")
		return
	}
	next, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "hash error")
		return
	}
	u.PassHash = next
	if err := h.DB.UpdateUser(r.Context(), u); err != nil {
		writeError(w, http.StatusInternalServerError, "save error")
		return
	}
	_ = h.DB.Q.DeleteRecoveryToken(r.Context(), row.ID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	sub, err := h.Tokens.ParseRefreshToken(req.RefreshToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	u, err := h.userBySubject(r.Context(), sub)
	if err != nil || u.Disabled {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	h.renderTokens(w, u)
}

func (h *Handler) renderTokens(w http.ResponseWriter, u settingsdb.User) {
	access, refresh, err := h.Tokens.GenerateTokens(strconv.FormatInt(u.ID, 10))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"access_token":  access,
		"refresh_token": refresh,
		"token_type":    "Bearer",
	})
}

func (h *Handler) requireJWT(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		token, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || token == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		u, err := h.userFromAccessToken(r.Context(), token)
		if err != nil || u.Disabled {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r.WithContext(contextWithUser(r.Context(), u)))
	})
}

func (h *Handler) userFromAccessToken(ctx context.Context, token string) (settingsdb.User, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return settingsdb.User{}, errMsg("unauthorized")
	}
	sub, err := h.Tokens.ParseJWT(token)
	if err != nil {
		return settingsdb.User{}, err
	}
	return h.userBySubject(ctx, sub)
}

func (h *Handler) requireRoles(roles ...string) func(http.Handler) http.Handler {
	allow := map[string]struct{}{}
	for _, role := range roles {
		allow[role] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u, ok := userFromContext(r.Context())
			if !ok {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			if _, ok := allow[u.Role]; !ok {
				writeError(w, http.StatusForbidden, "forbidden")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (h *Handler) userBySubject(ctx context.Context, sub string) (settingsdb.User, error) {
	id, err := strconv.ParseInt(sub, 10, 64)
	if err != nil {
		return settingsdb.User{}, err
	}
	return h.DB.GetUserByID(ctx, id)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	u, _ := userFromContext(r.Context())
	dto := u.Public()
	name, _ := h.DB.GetMeta(r.Context(), setup.KeyTelegramBotName)
	dto["telegram_bot_username"] = name
	writeJSON(w, http.StatusOK, dto)
}

func (h *Handler) patchMe(w http.ResponseWriter, r *http.Request) {
	u, _ := userFromContext(r.Context())
	var req patchMeReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Name != "" {
		u.Name = strings.TrimSpace(req.Name)
	}
	if req.Email != "" {
		email := normalizeEmail(req.Email)
		if !validEmail(email) {
			writeError(w, http.StatusBadRequest, "invalid email")
			return
		}
		if email != u.Email {
			if other, err := h.DB.GetUserByEmail(r.Context(), email); err == nil && other.ID != u.ID {
				writeError(w, http.StatusConflict, "email already in use")
				return
			}
		}
		u.Email = email
	}
	if req.Theme != "" {
		if req.Theme != setup.ThemeLight && req.Theme != setup.ThemeDark {
			writeError(w, http.StatusBadRequest, "theme must be light or dark")
			return
		}
		u.Theme = req.Theme
	}
	if req.MapStyle != "" {
		if req.MapStyle != setup.MapAuto && req.MapStyle != setup.MapLight && req.MapStyle != setup.MapDark {
			writeError(w, http.StatusBadRequest, "map_style must be auto, light or dark")
			return
		}
		u.MapStyle = req.MapStyle
	}
	if req.Password != "" {
		if len(req.Password) < 8 {
			writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
			return
		}
		if auth.CheckPassword(u.PassHash, req.CurrentPassword) != nil {
			writeError(w, http.StatusBadRequest, "current password is incorrect")
			return
		}
		next, err := auth.HashPassword(req.Password)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "hash error")
			return
		}
		u.PassHash = next
	}
	if err := h.DB.UpdateUser(r.Context(), u); err != nil {
		writeError(w, http.StatusInternalServerError, "save error")
		return
	}
	writeJSON(w, http.StatusOK, u.Public())
}

func (h *Handler) loadSettings(r *http.Request) setup.Settings {
	s := setup.Settings{}
	s.PKIDir, _ = h.DB.GetMeta(r.Context(), setup.KeyPKIDir)
	s.ServerConf, _ = h.DB.GetMeta(r.Context(), setup.KeyServerConf)
	s.Unit, _ = h.DB.GetMeta(r.Context(), setup.KeyUnit)
	s.LogFile, _ = h.DB.GetMeta(r.Context(), setup.KeyLogFile)
	s.PublicHost, _ = h.DB.GetMeta(r.Context(), setup.KeyPublicHost)
	s.Complete = h.isComplete(r)
	s.SMTPHost, _ = h.DB.GetMeta(r.Context(), setup.KeySMTPHost)
	s.SMTPPort, _ = h.DB.GetMeta(r.Context(), setup.KeySMTPPort)
	s.SMTPUser, _ = h.DB.GetMeta(r.Context(), setup.KeySMTPUser)
	s.SMTPPass, _ = h.DB.GetMeta(r.Context(), setup.KeySMTPPass)
	s.SMTPFrom, _ = h.DB.GetMeta(r.Context(), setup.KeySMTPFrom)
	tlsv, _ := h.DB.GetMeta(r.Context(), setup.KeySMTPTLS)
	s.SMTPTLS = tlsv == "" || setup.ParseBool(tlsv)
	s.TelegramBotToken, _ = h.DB.GetMeta(r.Context(), setup.KeyTelegramBotToken)
	s.TelegramBotName, _ = h.DB.GetMeta(r.Context(), setup.KeyTelegramBotName)
	return s
}

func (h *Handler) smtpConfig(r *http.Request) mailer.Config {
	s := h.loadSettings(r)
	return mailer.Config{
		Host: s.SMTPHost,
		Port: mailer.ParsePort(s.SMTPPort),
		User: s.SMTPUser,
		Pass: s.SMTPPass,
		From: s.SMTPFrom,
		TLS:  s.SMTPTLS,
	}
}

func (h *Handler) sendMail(cfg mailer.Config, to, subject, body string) error {
	if h.MailSend != nil {
		return h.MailSend(cfg, to, subject, body)
	}
	return mailer.Send(cfg, to, subject, body)
}

func (h *Handler) sendTelegram(token, chatID, text string) error {
	if h.TGSend != nil {
		return h.TGSend(token, chatID, text)
	}
	return telegram.SendMessage(token, chatID, text)
}

func contextWithUser(ctx context.Context, user settingsdb.User) context.Context {
	return context.WithValue(ctx, userKey, user)
}

func userFromContext(ctx context.Context) (settingsdb.User, bool) {
	u, ok := ctx.Value(userKey).(settingsdb.User)
	return u, ok && u.ID != 0
}
