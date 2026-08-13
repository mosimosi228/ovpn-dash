package httpserver

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/mosimosi228/kit/auth"
	"github.com/mosimosi228/ovpn-dash/internal/setup"
)

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type refreshReq struct {
	RefreshToken string `json:"refresh_token"`
}

type patchMeReq struct {
	CurrentPassword string `json:"current_password"`
	Password        string `json:"password"`
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	user, _ := h.DB.GetMeta(r.Context(), setup.KeyAdminUser)
	hash, _ := h.DB.GetMeta(r.Context(), setup.KeyAdminPassHash)
	if user == "" || hash == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if strings.TrimSpace(req.Username) != user || auth.CheckPassword(hash, req.Password) != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	h.renderTokens(w, user)
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
	user, _ := h.DB.GetMeta(r.Context(), setup.KeyAdminUser)
	if user == "" || sub != user {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	h.renderTokens(w, user)
}

func (h *Handler) renderTokens(w http.ResponseWriter, user string) {
	access, refresh, err := h.Tokens.GenerateTokens(user)
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
		sub, err := h.Tokens.ParseJWT(token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		user, _ := h.DB.GetMeta(r.Context(), setup.KeyAdminUser)
		if user == "" || sub != user {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r.WithContext(contextWithUser(r.Context(), user)))
	})
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	user, _ := userFromContext(r.Context())
	writeJSON(w, http.StatusOK, map[string]string{"username": user})
}

func (h *Handler) patchMe(w http.ResponseWriter, r *http.Request) {
	var req patchMeReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Password == "" {
		writeError(w, http.StatusBadRequest, "password is required")
		return
	}
	hash, _ := h.DB.GetMeta(r.Context(), setup.KeyAdminPassHash)
	if auth.CheckPassword(hash, req.CurrentPassword) != nil {
		writeError(w, http.StatusBadRequest, "current password is incorrect")
		return
	}
	next, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "hash error")
		return
	}
	if err := h.DB.SetMeta(r.Context(), setup.KeyAdminPassHash, next); err != nil {
		writeError(w, http.StatusInternalServerError, "save error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "1"})
}

func (h *Handler) loadSettings(r *http.Request) setup.Settings {
	s := setup.Settings{}
	s.AdminUser, _ = h.DB.GetMeta(r.Context(), setup.KeyAdminUser)
	s.PKIDir, _ = h.DB.GetMeta(r.Context(), setup.KeyPKIDir)
	s.ServerConf, _ = h.DB.GetMeta(r.Context(), setup.KeyServerConf)
	s.Unit, _ = h.DB.GetMeta(r.Context(), setup.KeyUnit)
	s.LogFile, _ = h.DB.GetMeta(r.Context(), setup.KeyLogFile)
	s.PublicHost, _ = h.DB.GetMeta(r.Context(), setup.KeyPublicHost)
	s.Complete = h.isComplete(r)
	return s
}

func absPath(p string) string {
	if p == "" {
		return p
	}
	if filepath.IsAbs(p) {
		return p
	}
	a, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return a
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func contextWithUser(ctx context.Context, user string) context.Context {
	return context.WithValue(ctx, userKey, user)
}

func userFromContext(ctx context.Context) (string, bool) {
	u, ok := ctx.Value(userKey).(string)
	return u, ok && u != ""
}
