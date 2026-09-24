package httpserver

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	repoURL    = "https://github.com/mosimosi228/ovpn-dash"
	releaseURL = "https://api.github.com/repos/mosimosi228/ovpn-dash/releases/latest"
)

var fetchLatestRelease = func() string {
	client := &http.Client{Timeout: 4 * time.Second}
	req, err := http.NewRequest(http.MethodGet, releaseURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "ovpn-dash")
	res, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return ""
	}
	var body struct {
		TagName string `json:"tag_name"`
	}
	if json.NewDecoder(res.Body).Decode(&body) != nil {
		return ""
	}
	return strings.TrimSpace(body.TagName)
}

func (h *Handler) appVersion(w http.ResponseWriter, r *http.Request) {
	cur := strings.TrimSpace(h.Version)
	if cur == "" {
		cur = "dev"
	}
	update := ""
	if latest := h.latestRelease(); semverNewer(latest, cur) {
		update = latest
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"version": cur,
		"update":  update,
		"repo":    repoURL,
	})
}

func (h *Handler) latestRelease() string {
	if h == nil {
		return ""
	}
	h.relMu.Lock()
	if !h.relAt.IsZero() && time.Since(h.relAt) < time.Hour {
		tag := h.relTag
		h.relMu.Unlock()
		return tag
	}
	h.relMu.Unlock()

	tag := fetchLatestRelease()
	h.relMu.Lock()
	h.relAt = time.Now()
	h.relTag = tag
	h.relMu.Unlock()
	return tag
}

func semverNewer(latest, current string) bool {
	l, okL := parseSemver(latest)
	c, okC := parseSemver(current)
	if !okL || !okC {
		return false
	}
	for i := 0; i < 3; i++ {
		if l[i] == c[i] {
			continue
		}
		return l[i] > c[i]
	}
	return false
}

func parseSemver(s string) ([3]int, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "v")
	s = strings.TrimPrefix(s, "V")
	parts := strings.Split(s, ".")
	if len(parts) < 3 {
		return [3]int{}, false
	}
	var out [3]int
	for i := 0; i < 3; i++ {
		n, err := strconv.Atoi(parts[i])
		if err != nil || n < 0 {
			return [3]int{}, false
		}
		out[i] = n
	}
	return out, true
}
