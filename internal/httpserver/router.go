package httpserver

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httplog/v3"
	"github.com/go-chi/httprate"
	"github.com/mosimosi228/kit/auth"
	"github.com/mosimosi228/ovpn-dash/internal/geo"
	"github.com/mosimosi228/ovpn-dash/internal/settingsdb"
	"github.com/mosimosi228/ovpn-dash/internal/setup"
	"github.com/mosimosi228/ovpn-dash/web"
)

// Handler is the HTTP surface: SPA, wizard, JWT auth, OpenVPN APIs.
type Handler struct {
	Dir    string
	DB     *settingsdb.DB
	Tokens *auth.Auth
	Log    *slog.Logger
	Geo    geo.Locator
}

type ctxKey int

const userKey ctxKey = 1

func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	if h.Log != nil {
		r.Use(httplog.RequestLogger(h.Log, &httplog.Options{
			Level:         slog.LevelInfo,
			Schema:        httplog.SchemaOTEL,
			RecoverPanics: true,
		}))
	} else {
		r.Use(middleware.Recoverer)
	}
	r.Use(middleware.RealIP)
	r.Use(middleware.CleanPath)
	r.Use(cors)
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"ok": "1"})
	})
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/dashboard/", http.StatusFound)
	})

	r.With(httprate.LimitByIP(5, time.Minute)).Post("/auth/login", h.login)
	r.Post("/auth/refresh", h.refresh)

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(h.requireJWT)
		r.Get("/me", h.me)
		r.Patch("/me", h.patchMe)
		r.Get("/settings", h.getSettings)
		r.Patch("/settings", h.patchSettings)
		r.Get("/server", h.serverStatus)
		r.Post("/server/start", h.serverStart)
		r.Post("/server/stop", h.serverStop)
		r.Get("/server/log", h.serverLog)
		r.Get("/connections", h.listConnections)
		r.Get("/clients", h.listClients)
		r.Post("/clients", h.createClient)
		r.Get("/clients/{name}/ovpn", h.downloadOVPN)
		r.Delete("/clients/{name}", h.deleteClient)
	})

	r.Get("/dashboard/api/state", h.apiState)
	r.Post("/dashboard/api/setup", h.apiSetup)
	r.Handle("/dashboard", h.spa())
	r.Handle("/dashboard/*", h.spa())
	return r
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Requested-With, X-Setup-Token")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func init() {
	// Linux FileServer often sniffs CSS/JS as text/plain when /etc/mime.types is thin.
	_ = mime.AddExtensionType(".css", "text/css; charset=utf-8")
	_ = mime.AddExtensionType(".js", "text/javascript; charset=utf-8")
	_ = mime.AddExtensionType(".mjs", "text/javascript; charset=utf-8")
	_ = mime.AddExtensionType(".map", "application/json")
	_ = mime.AddExtensionType(".svg", "image/svg+xml")
	_ = mime.AddExtensionType(".woff2", "font/woff2")
	_ = mime.AddExtensionType(".json", "application/json")
}

func (h *Handler) spa() http.Handler {
	dist, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		dist = web.Dist
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(r.URL.Path, "/dashboard")
		rel = strings.TrimPrefix(path.Clean("/"+rel), "/")
		if isStaticAsset(rel) && serveStatic(w, dist, rel) {
			return
		}
		if !h.gateSPA(w, r) {
			return
		}
		if rel != "" && rel != "." && serveStatic(w, dist, rel) {
			return
		}
		serveIndex(w, dist)
	})
}

func serveStatic(w http.ResponseWriter, dist fs.FS, rel string) bool {
	b, err := fs.ReadFile(dist, rel)
	if err != nil {
		return false
	}
	ctype := mime.TypeByExtension(path.Ext(rel))
	if ctype == "" {
		ctype = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ctype)
	if strings.HasPrefix(rel, "assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	_, _ = w.Write(b)
	return true
}

func isStaticAsset(rel string) bool {
	return strings.HasPrefix(rel, "assets/") ||
		strings.HasSuffix(rel, ".js") ||
		strings.HasSuffix(rel, ".css") ||
		strings.HasSuffix(rel, ".map") ||
		strings.HasSuffix(rel, ".woff2") ||
		strings.HasSuffix(rel, ".png") ||
		strings.HasSuffix(rel, ".svg") ||
		strings.HasSuffix(rel, ".ico")
}

func serveIndex(w http.ResponseWriter, dist fs.FS) {
	b, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		http.Error(w, "dashboard UI not built — run: make web", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(b)
}

func (h *Handler) hasAdmin(r *http.Request) bool {
	u, _ := h.DB.GetMeta(r.Context(), setup.KeyAdminUser)
	return u != ""
}

func (h *Handler) isComplete(r *http.Request) bool {
	v, _ := h.DB.GetMeta(r.Context(), setup.KeySetupComplete)
	return setup.ParseComplete(v)
}

// gateSPA: before admin exists, require localhost or setup token. After that SPA is public (login form).
func (h *Handler) gateSPA(w http.ResponseWriter, r *http.Request) bool {
	if h.hasAdmin(r) {
		return true
	}
	if !h.allowBootstrap(r) {
		http.Error(w, "forbidden — open from localhost or pass ?setup_token=…", http.StatusForbidden)
		return false
	}
	return true
}

func (h *Handler) allowBootstrap(r *http.Request) bool {
	ip := r.RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		ip = host
	}
	forwarded := r.Header.Get("X-Forwarded-For") != "" || r.Header.Get("X-Real-IP") != ""
	if !forwarded && (ip == "127.0.0.1" || ip == "::1") {
		return true
	}
	tok := r.Header.Get("X-Setup-Token")
	if tok == "" {
		tok = r.URL.Query().Get("setup_token")
	}
	if tok == "" {
		return false
	}
	want := strings.TrimSpace(os.Getenv("OVPNDASH_SETUP_TOKEN"))
	if want == "" {
		if b, err := os.ReadFile(filepath.Join(h.Dir, "setup.token")); err == nil {
			want = strings.TrimSpace(string(b))
		}
	}
	return want != "" && tok == want
}

// EnsureSetupToken writes setup.token if missing (first run).
func EnsureSetupToken(dir string) (string, error) {
	path := filepath.Join(dir, "setup.token")
	if b, err := os.ReadFile(path); err == nil {
		return strings.TrimSpace(string(b)), nil
	}
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	tok := hex.EncodeToString(raw)
	if err := os.WriteFile(path, []byte(tok+"\n"), 0o600); err != nil {
		return "", err
	}
	return tok, nil
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func readJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	return dec.Decode(dst)
}
