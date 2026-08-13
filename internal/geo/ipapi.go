package geo

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Info is a coarse location for a public IP.
type Info struct {
	IP      string  `json:"ip"`
	Country string  `json:"country,omitempty"`
	City    string  `json:"city,omitempty"`
	Lat     float64 `json:"lat,omitempty"`
	Lon     float64 `json:"lon,omitempty"`
}

// Locator looks up coordinates for a public IP.
type Locator interface {
	Lookup(ip string) *Info
}

type cacheEntry struct {
	info *Info
	at   time.Time
}

// API looks up IPs via ip-api.com and caches results.
type API struct {
	mu     sync.Mutex
	cache  map[string]cacheEntry
	client *http.Client
	ttl    time.Duration
}

func New() *API {
	return &API{
		cache:  map[string]cacheEntry{},
		client: &http.Client{Timeout: 3 * time.Second},
		ttl:    6 * time.Hour,
	}
}

func (a *API) Lookup(ip string) *Info {
	parsed := net.ParseIP(strings.TrimSpace(ip))
	if parsed == nil || parsed.IsLoopback() || parsed.IsPrivate() || parsed.IsUnspecified() || parsed.IsLinkLocalUnicast() {
		return nil
	}
	ip = parsed.String()
	a.mu.Lock()
	if e, ok := a.cache[ip]; ok && time.Since(e.at) < a.ttl {
		a.mu.Unlock()
		return e.info
	}
	a.mu.Unlock()

	info := a.fetch(ip)
	a.mu.Lock()
	a.cache[ip] = cacheEntry{info: info, at: time.Now()}
	a.mu.Unlock()
	return info
}

func (a *API) fetch(ip string) *Info {
	u := "http://ip-api.com/json/" + ip + "?fields=status,country,city,lat,lon,query"
	res, err := a.client.Get(u)
	if err != nil {
		return nil
	}
	defer res.Body.Close()
	var raw struct {
		Status  string  `json:"status"`
		Country string  `json:"country"`
		City    string  `json:"city"`
		Lat     float64 `json:"lat"`
		Lon     float64 `json:"lon"`
		Query   string  `json:"query"`
	}
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil || raw.Status != "success" {
		return nil
	}
	return &Info{IP: raw.Query, Country: raw.Country, City: raw.City, Lat: raw.Lat, Lon: raw.Lon}
}
