package httpserver

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mosimosi228/ovpn-dash/internal/setup"
	"github.com/mosimosi228/ovpn-dash/internal/telegram"
)

func (h *Handler) telegramBind(w http.ResponseWriter, r *http.Request) {
	u, _ := userFromContext(r.Context())
	token, _ := h.DB.GetMeta(r.Context(), setup.KeyTelegramBotToken)
	if token == "" {
		writeError(w, http.StatusBadRequest, "telegram is not configured")
		return
	}
	code, err := randomHex(4)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "code error")
		return
	}
	code = strings.ToUpper(code)
	u.TelegramBind = code
	if err := h.DB.UpdateUser(r.Context(), u); err != nil {
		writeError(w, http.StatusInternalServerError, "save error")
		return
	}
	bot, _ := h.DB.GetMeta(r.Context(), setup.KeyTelegramBotName)
	writeJSON(w, http.StatusOK, map[string]any{
		"code":         code,
		"bot_username": bot,
		"command":      "/start " + code,
	})
}

func (h *Handler) telegramUnbind(w http.ResponseWriter, r *http.Request) {
	u, _ := userFromContext(r.Context())
	u.TelegramChatID = ""
	u.TelegramBind = ""
	if err := h.DB.UpdateUser(r.Context(), u); err != nil {
		writeError(w, http.StatusInternalServerError, "save error")
		return
	}
	writeJSON(w, http.StatusOK, u.Public())
}

// RunTelegram polls getUpdates until ctx is done.
func (h *Handler) RunTelegram(ctx context.Context) {
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			h.pollTelegram(ctx)
		}
	}
}

func (h *Handler) pollTelegram(ctx context.Context) {
	token, err := h.DB.GetMeta(ctx, setup.KeyTelegramBotToken)
	if err != nil || token == "" {
		return
	}
	offStr, _ := h.DB.GetMeta(ctx, setup.KeyTelegramOffset)
	offset, _ := strconv.ParseInt(offStr, 10, 64)
	updates, err := telegram.GetUpdates(token, offset)
	if err != nil {
		if h.Log != nil {
			h.Log.Debug("telegram poll", slog.String("err", err.Error()))
		}
		return
	}
	for _, upd := range updates {
		next := upd.UpdateID + 1
		_ = h.DB.SetMeta(ctx, setup.KeyTelegramOffset, strconv.FormatInt(next, 10))
		h.handleTelegramUpdate(ctx, token, upd)
	}
}

func (h *Handler) handleTelegramUpdate(ctx context.Context, token string, upd telegram.Update) {
	if upd.Message == nil {
		return
	}
	chatID := telegram.ChatIDString(upd.Message.Chat.ID)
	text := strings.TrimSpace(upd.Message.Text)
	code := text
	if strings.HasPrefix(strings.ToLower(text), "/start") {
		parts := strings.Fields(text)
		if len(parts) >= 2 {
			code = parts[1]
		} else {
			_ = h.sendTelegram(token, chatID, "Send /start CODE from your OVPN Dashboard profile to bind this chat.")
			return
		}
	}
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return
	}
	u, err := h.DB.GetUserByTelegramBind(ctx, code)
	if err != nil {
		_ = h.sendTelegram(token, chatID, "Unknown or expired bind code.")
		return
	}
	if other, err := h.DB.GetUserByTelegramChatID(ctx, chatID); err == nil && other.ID != u.ID {
		_ = h.sendTelegram(token, chatID, "This chat is already bound to another account.")
		return
	}
	u.TelegramChatID = chatID
	u.TelegramBind = ""
	if err := h.DB.UpdateUser(ctx, u); err != nil {
		return
	}
	_ = h.sendTelegram(token, chatID, "Bound to "+u.Email+". You can request a login PIN from the dashboard.")
}
