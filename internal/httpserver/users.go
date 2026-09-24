package httpserver

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/mosimosi228/kit/auth"
	"github.com/mosimosi228/ovpn-dash/internal/pki"
	"github.com/mosimosi228/ovpn-dash/internal/settingsdb"
	"github.com/mosimosi228/ovpn-dash/internal/setup"
	"github.com/mosimosi228/ovpn-dash/internal/systemd"
)

type userCreateReq struct {
	Email      string `json:"email"`
	Name       string `json:"name"`
	Password   string `json:"password"`
	Role       string `json:"role"`
	ClientName string `json:"client_name"`
}

type userPatchReq struct {
	Name     *string `json:"name"`
	Email    *string `json:"email"`
	Disabled *bool   `json:"disabled"`
	Role     *string `json:"role"`
}

func (h *Handler) listUsers(w http.ResponseWriter, r *http.Request) {
	items, err := h.DB.ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list error")
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, u := range items {
		out = append(out, u.Public())
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *Handler) createUser(w http.ResponseWriter, r *http.Request) {
	actor, _ := userFromContext(r.Context())
	var req userCreateReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	role := strings.TrimSpace(req.Role)
	if role == "" {
		role = setup.RoleUser
	}
	switch role {
	case setup.RoleAdmin:
		if actor.Role != setup.RoleRoot {
			writeError(w, http.StatusForbidden, "only root can create admins")
			return
		}
		u, err := h.insertAccount(r, req.Email, req.Name, req.Password, setup.RoleAdmin, "")
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, u.Public())
	case setup.RoleUser:
		u, err := h.createUserWithCert(r, req.Email, req.Name, req.Password, req.ClientName)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, u.Public())
	default:
		writeError(w, http.StatusBadRequest, "role must be admin or user")
	}
}

func (h *Handler) patchUser(w http.ResponseWriter, r *http.Request) {
	actor, _ := userFromContext(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	u, err := h.DB.GetUserByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if !canManage(actor, u) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	var req userPatchReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if u.Role == setup.RoleRoot {
		if req.Disabled != nil || req.Role != nil {
			writeError(w, http.StatusBadRequest, "root cannot be disabled or demoted")
			return
		}
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}
		u.Name = name
	}
	if req.Email != nil {
		email := normalizeEmail(*req.Email)
		if email == "" {
			if u.Role != setup.RoleUser {
				writeError(w, http.StatusBadRequest, "email is required")
				return
			}
			u.Email = ""
		} else if !validEmail(email) {
			writeError(w, http.StatusBadRequest, "invalid email")
			return
		} else {
			if email != u.Email {
				if other, err := h.DB.GetUserByEmail(r.Context(), email); err == nil && other.ID != u.ID {
					writeError(w, http.StatusConflict, "email already in use")
					return
				}
			}
			u.Email = email
		}
	}
	if req.Role != nil {
		if actor.Role != setup.RoleRoot {
			writeError(w, http.StatusForbidden, "only root can change roles")
			return
		}
		role := strings.TrimSpace(*req.Role)
		if role != setup.RoleAdmin && role != setup.RoleUser {
			writeError(w, http.StatusBadRequest, "role must be admin or user")
			return
		}
		if u.Role == setup.RoleRoot {
			writeError(w, http.StatusBadRequest, "root cannot be demoted")
			return
		}
		if role == setup.RoleUser && u.ClientName == "" {
			writeError(w, http.StatusBadRequest, "demote to user requires an existing client certificate")
			return
		}
		u.Role = role
	}
	if req.Disabled != nil {
		u.Disabled = *req.Disabled
	}
	if u.Role != setup.RoleUser && strings.TrimSpace(u.Email) == "" {
		writeError(w, http.StatusBadRequest, "email is required")
		return
	}
	if err := h.DB.UpdateUser(r.Context(), u); err != nil {
		writeError(w, http.StatusInternalServerError, "save error")
		return
	}
	writeJSON(w, http.StatusOK, u.Public())
}

func (h *Handler) deleteUser(w http.ResponseWriter, r *http.Request) {
	actor, _ := userFromContext(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	u, err := h.DB.GetUserByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if u.Role == setup.RoleRoot {
		writeError(w, http.StatusBadRequest, "root cannot be deleted")
		return
	}
	if !canManage(actor, u) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if u.ClientName != "" {
		_ = h.store(r).Revoke(u.ClientName)
		h.publishCRL(r)
		s := h.loadSettings(r)
		_ = systemd.ReloadOrRestart(s.Unit)
	}
	if err := h.DB.DeleteUser(r.Context(), u.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "delete error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func canManage(actor, target settingsdb.User) bool {
	if target.Role == setup.RoleRoot {
		return actor.Role == setup.RoleRoot && actor.ID == target.ID
	}
	switch actor.Role {
	case setup.RoleRoot:
		return true
	case setup.RoleAdmin:
		return target.Role == setup.RoleUser
	default:
		return false
	}
}

func (h *Handler) insertAccount(r *http.Request, email, name, password, role, clientName string) (settingsdb.User, error) {
	email = normalizeEmail(email)
	if role != setup.RoleUser {
		if !validEmail(email) {
			return settingsdb.User{}, errMsg("email is required")
		}
	} else if email != "" && !validEmail(email) {
		return settingsdb.User{}, errMsg("invalid email")
	}
	if len(password) < 8 {
		return settingsdb.User{}, errMsg("password must be at least 8 characters")
	}
	name = strings.TrimSpace(name)
	if name == "" && email != "" {
		name = strings.Split(email, "@")[0]
	}
	if name == "" {
		name = strings.TrimSpace(clientName)
	}
	if name == "" {
		return settingsdb.User{}, errMsg("name is required")
	}
	if email != "" {
		if _, err := h.DB.GetUserByEmail(r.Context(), email); err == nil {
			return settingsdb.User{}, errMsg("email already in use")
		}
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return settingsdb.User{}, errMsg("hash error")
	}
	return h.DB.InsertUser(r.Context(), settingsdb.User{
		Email:      email,
		Name:       name,
		PassHash:   hash,
		Role:       role,
		ClientName: clientName,
		Theme:      setup.ThemeLight,
		MapStyle:   setup.MapAuto,
	})
}

func (h *Handler) createUserWithCert(r *http.Request, email, name, password, clientName string) (settingsdb.User, error) {
	cn := strings.TrimSpace(clientName)
	if cn == "" {
		cn = sanitizeClientName(name)
	}
	if cn == "" {
		cn = sanitizeClientName(strings.Split(normalizeEmail(email), "@")[0])
	}
	if err := pki.ValidateName(cn); err != nil {
		return settingsdb.User{}, err
	}
	if _, err := h.DB.GetUserByClientName(r.Context(), cn); err == nil {
		return settingsdb.User{}, errMsg("client already has a dashboard user")
	}
	u, err := h.insertAccount(r, email, name, password, setup.RoleUser, cn)
	if err != nil {
		return settingsdb.User{}, err
	}
	if err := h.store(r).Issue(cn); err != nil {
		_ = h.DB.DeleteUser(r.Context(), u.ID)
		return settingsdb.User{}, err
	}
	if err := h.writeClientOvpn(r, cn); err != nil {
		h.store(r).Discard(cn)
		_ = h.DB.DeleteUser(r.Context(), u.ID)
		return settingsdb.User{}, err
	}
	return u, nil
}

type errMsg string

func (e errMsg) Error() string { return string(e) }
