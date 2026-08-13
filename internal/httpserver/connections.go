package httpserver

import (
	"net/http"

	"github.com/mosimosi228/ovpn-dash/internal/ovpn"
)

type connectionDTO struct {
	ovpn.Session
	Country string  `json:"country,omitempty"`
	City    string  `json:"city,omitempty"`
	Lat     float64 `json:"lat,omitempty"`
	Lon     float64 `json:"lon,omitempty"`
}

func (h *Handler) listConnections(w http.ResponseWriter, r *http.Request) {
	s := h.loadSettings(r)
	cfg, err := ovpn.ParseFile(s.ServerConf)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"items":       []any{},
			"status_file": "",
			"hint":        "server.conf unreadable",
		})
		return
	}
	if cfg.StatusFile == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"items":       []any{},
			"status_file": "",
			"hint":        "status",
		})
		return
	}
	sessions, err := ovpn.ParseStatusFile(cfg.StatusFile)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"items":       []any{},
			"status_file": cfg.StatusFile,
			"hint":        err.Error(),
		})
		return
	}
	items := make([]connectionDTO, 0, len(sessions))
	for _, sess := range sessions {
		dto := connectionDTO{Session: sess}
		if h.Geo != nil && sess.RealIP != "" {
			if g := h.Geo.Lookup(sess.RealIP); g != nil {
				dto.Country = g.Country
				dto.City = g.City
				dto.Lat = g.Lat
				dto.Lon = g.Lon
			}
		}
		items = append(items, dto)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":       items,
		"status_file": cfg.StatusFile,
	})
}
