package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mosimosi228/ovpn-dash/internal/ovpn"
	"github.com/mosimosi228/ovpn-dash/internal/setup"
)

type connectionDTO struct {
	ovpn.Session
	Country     string  `json:"country,omitempty"`
	CountryCode string  `json:"country_code,omitempty"`
	Region      string  `json:"region,omitempty"`
	City        string  `json:"city,omitempty"`
	Lat         float64 `json:"lat,omitempty"`
	Lon         float64 `json:"lon,omitempty"`
}

type connectionsPayload struct {
	Items      []connectionDTO `json:"items"`
	StatusFile string          `json:"status_file,omitempty"`
	Hint       string          `json:"hint,omitempty"`
	CanKill    bool            `json:"can_kill"`
}

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	CheckOrigin:     func(*http.Request) bool { return true },
}

func (h *Handler) connectionsSnapshot(r *http.Request) connectionsPayload {
	out := connectionsPayload{Items: []connectionDTO{}}
	s := h.loadSettings(r)
	cfg, err := ovpn.ParseFile(s.ServerConf)
	if err != nil {
		out.Hint = "server.conf unreadable"
		return out
	}
	out.CanKill = cfg.CanKill()
	paths := ovpn.StatusCandidates(cfg.StatusFile, s.Unit)
	if len(paths) == 0 {
		out.Hint = "status"
		return out
	}
	sessions, used, err := ovpn.ParseBestStatus(paths)
	if used != "" {
		out.StatusFile = used
	} else {
		out.StatusFile = cfg.StatusFile
	}
	if err != nil {
		switch {
		case errors.Is(err, os.ErrPermission) || os.IsPermission(err):
			out.Hint = "denied"
		case os.IsNotExist(err):
			out.Hint = "missing"
		default:
			out.Hint = err.Error()
		}
		return out
	}
	out.Items = make([]connectionDTO, 0, len(sessions))
	for _, sess := range sessions {
		dto := connectionDTO{Session: sess}
		if h.Geo != nil && sess.RealIP != "" {
			if g := h.Geo.Lookup(sess.RealIP); g != nil {
				dto.Country = g.Country
				dto.CountryCode = g.CountryCode
				dto.Region = g.Region
				dto.City = g.City
				dto.Lat = g.Lat
				dto.Lon = g.Lon
			}
		}
		out.Items = append(out.Items, dto)
	}
	return out
}

func (h *Handler) listConnections(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.connectionsSnapshot(r))
}

type killConnReq struct {
	Name        string `json:"name"`
	RealAddress string `json:"real_address"`
}

func (h *Handler) killConnection(w http.ResponseWriter, r *http.Request) {
	var req killConnReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	s := h.loadSettings(r)
	cfg, err := ovpn.ParseFile(s.ServerConf)
	if err != nil {
		writeError(w, http.StatusBadGateway, "server.conf: "+err.Error())
		return
	}
	if !cfg.CanKill() {
		writeError(w, http.StatusBadRequest, "nomanage")
		return
	}
	if err := cfg.Kill(req.RealAddress, req.Name); err != nil {
		msg := err.Error()
		if msg == "nomanage" {
			writeError(w, http.StatusBadRequest, "nomanage")
			return
		}
		writeError(w, http.StatusBadGateway, msg)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) connectionsWS(w http.ResponseWriter, r *http.Request) {
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		return
	}
	var auth struct {
		Token string `json:"token"`
	}
	if json.Unmarshal(raw, &auth) != nil || auth.Token == "" {
		_ = conn.WriteJSON(map[string]string{"error": "unauthorized"})
		return
	}
	u, err := h.userFromAccessToken(r.Context(), auth.Token)
	if err != nil || u.Disabled || (u.Role != setup.RoleRoot && u.Role != setup.RoleAdmin) {
		_ = conn.WriteJSON(map[string]string{"error": "unauthorized"})
		return
	}
	_ = conn.SetReadDeadline(time.Time{})
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.NextReader(); err != nil {
				return
			}
		}
	}()

	tick := time.NewTicker(time.Second)
	ping := time.NewTicker(20 * time.Second)
	defer tick.Stop()
	defer ping.Stop()

	var last string
	writeSnap := func() error {
		b, err := json.Marshal(h.connectionsSnapshot(r))
		if err != nil {
			return err
		}
		if string(b) == last {
			return nil
		}
		last = string(b)
		_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		return conn.WriteMessage(websocket.TextMessage, b)
	}
	if err := writeSnap(); err != nil {
		return
	}

	for {
		select {
		case <-done:
			return
		case <-r.Context().Done():
			return
		case <-tick.C:
			if err := writeSnap(); err != nil {
				return
			}
		case <-ping.C:
			_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
