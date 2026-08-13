package httpserver

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mosimosi228/kit/auth"
	"github.com/mosimosi228/ovpn-dash/internal/settingsdb"
)

func writeMiniPKI(t *testing.T, dir string) (pkiDir, conf string) {
	t.Helper()
	pkiDir = filepath.Join(dir, "pki")
	if err := os.MkdirAll(filepath.Join(pkiDir, "private"), 0o700); err != nil {
		t.Fatal(err)
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(pkiDir, "ca.crt"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644)
	_ = os.WriteFile(filepath.Join(pkiDir, "private", "ca.key"), pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0o600)
	conf = filepath.Join(dir, "server.conf")
	_ = os.WriteFile(conf, []byte("port 1194\nproto udp\ndev tun\n"), 0o644)
	return pkiDir, conf
}

func newTestHandler(t *testing.T) (*Handler, string) {
	t.Helper()
	dir := t.TempDir()
	db, err := settingsdb.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	tok, err := auth.New(auth.Config{Type: auth.TypeJWT, JWTSecret: "test-secret-test-secret-test-xx"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = EnsureSetupToken(dir)
	if err != nil {
		t.Fatal(err)
	}
	return &Handler{Dir: dir, DB: db, Tokens: tok}, dir
}

func TestSetupGateAndLogin(t *testing.T) {
	h, dir := newTestHandler(t)
	srv := httptest.NewServer(h.Routes())
	t.Cleanup(srv.Close)

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/dashboard/api/state", nil)
	req.Header.Set("X-Forwarded-For", "8.8.8.8")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("remote without token: %d", res.StatusCode)
	}

	tokb, _ := os.ReadFile(filepath.Join(dir, "setup.token"))
	tok := string(bytes.TrimSpace(tokb))
	req, _ = http.NewRequest(http.MethodGet, srv.URL+"/dashboard/api/state?setup_token="+tok, nil)
	req.Header.Set("X-Forwarded-For", "8.8.8.8")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("state with token: %d %s", res.StatusCode, body)
	}

	pkiDir, conf := writeMiniPKI(t, dir)
	setupBody, _ := json.Marshal(map[string]string{
		"username":    "admin",
		"password":    "password1",
		"pki_dir":     pkiDir,
		"server_conf": conf,
		"unit":        "openvpn-server@server",
		"log_file":    filepath.Join(dir, "openvpn.log"),
		"public_host": "vpn.example.com",
	})
	req, _ = http.NewRequest(http.MethodPost, srv.URL+"/dashboard/api/setup", bytes.NewReader(setupBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Setup-Token", tok)
	req.Header.Set("X-Forwarded-For", "8.8.8.8")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("setup: %d %s", res.StatusCode, body)
	}

	loginBody, _ := json.Marshal(map[string]string{"username": "admin", "password": "password1"})
	res, err = http.Post(srv.URL+"/auth/login", "application/json", bytes.NewReader(loginBody))
	if err != nil {
		t.Fatal(err)
	}
	var tokens struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(res.Body).Decode(&tokens); err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK || tokens.AccessToken == "" {
		t.Fatalf("login %d", res.StatusCode)
	}

	req, _ = http.NewRequest(http.MethodGet, srv.URL+"/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("me: %d %s", res.StatusCode, body)
	}

	req, _ = http.NewRequest(http.MethodGet, srv.URL+"/api/v1/me", nil)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("me without jwt: %d", res.StatusCode)
	}

	bad, _ := json.Marshal(map[string]string{"username": "admin", "password": "wrong-password"})
	res, err = http.Post(srv.URL+"/auth/login", "application/json", bytes.NewReader(bad))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad login: %d", res.StatusCode)
	}
}

func setupAndToken(t *testing.T, srv *httptest.Server, dir string) (token, pkiDir, conf string) {
	t.Helper()
	tokb, _ := os.ReadFile(filepath.Join(dir, "setup.token"))
	tok := string(bytes.TrimSpace(tokb))
	pkiDir, conf = writeMiniPKI(t, dir)
	setupBody, _ := json.Marshal(map[string]string{
		"username":    "admin",
		"password":    "password1",
		"pki_dir":     pkiDir,
		"server_conf": conf,
		"unit":        "openvpn-server@server",
		"log_file":    filepath.Join(dir, "openvpn.log"),
		"public_host": "vpn.example.com",
	})
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/dashboard/api/setup", bytes.NewReader(setupBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Setup-Token", tok)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("setup: %d %s", res.StatusCode, body)
	}
	loginBody, _ := json.Marshal(map[string]string{"username": "admin", "password": "password1"})
	res, err = http.Post(srv.URL+"/auth/login", "application/json", bytes.NewReader(loginBody))
	if err != nil {
		t.Fatal(err)
	}
	var tokens struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(res.Body).Decode(&tokens); err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if tokens.AccessToken == "" {
		t.Fatal("no token")
	}
	return tokens.AccessToken, pkiDir, conf
}

func TestPatchSettings(t *testing.T) {
	h, dir := newTestHandler(t)
	srv := httptest.NewServer(h.Routes())
	t.Cleanup(srv.Close)
	token, _, _ := setupAndToken(t, srv, dir)

	pki2, conf2 := writeMiniPKI(t, filepath.Join(dir, "alt"))
	body, _ := json.Marshal(map[string]string{
		"pki_dir":     pki2,
		"server_conf": conf2,
		"unit":        "openvpn@server",
		"log_file":    filepath.Join(dir, "other.log"),
		"public_host": "vpn.other.example",
	})
	req, _ := http.NewRequest(http.MethodPatch, srv.URL+"/api/v1/settings", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("patch settings: %d %s", res.StatusCode, raw)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got["public_host"] != "vpn.other.example" {
		t.Fatalf("host %v", got["public_host"])
	}
	if got["unit"] != "openvpn@server" {
		t.Fatalf("unit %v", got["unit"])
	}

	bad, _ := json.Marshal(map[string]string{
		"pki_dir":          pki2,
		"server_conf":      conf2,
		"unit":             "openvpn@server",
		"public_host":      "vpn.other.example",
		"current_password": "wrong",
		"password":         "newpassword",
	})
	req, _ = http.NewRequest(http.MethodPatch, srv.URL+"/api/v1/settings", bytes.NewReader(bad))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad current password: %d", res.StatusCode)
	}

	okpw, _ := json.Marshal(map[string]string{
		"pki_dir":          pki2,
		"server_conf":      conf2,
		"unit":             "openvpn@server",
		"public_host":      "vpn.other.example",
		"current_password": "password1",
		"password":         "newpassword",
	})
	req, _ = http.NewRequest(http.MethodPatch, srv.URL+"/api/v1/settings", bytes.NewReader(okpw))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("password patch: %d", res.StatusCode)
	}

	loginBody, _ := json.Marshal(map[string]string{"username": "admin", "password": "newpassword"})
	res, err = http.Post(srv.URL+"/auth/login", "application/json", bytes.NewReader(loginBody))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("login with new password: %d", res.StatusCode)
	}
}

func TestServerLogNoFileIsOK(t *testing.T) {
	h, dir := newTestHandler(t)
	srv := httptest.NewServer(h.Routes())
	t.Cleanup(srv.Close)
	token, _, _ := setupAndToken(t, srv, dir)

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/server/log", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("missing log must be 200, got %d %s", res.StatusCode, raw)
	}

	logPath := filepath.Join(dir, "openvpn.log")
	if err := os.WriteFile(logPath, []byte("hello\nworld\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	req, _ = http.NewRequest(http.MethodGet, srv.URL+"/api/v1/server/log", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("log file: %d %s", res.StatusCode, raw)
	}
	if !strings.Contains(string(raw), "world") {
		t.Fatalf("want log body, got %s", raw)
	}
}

func TestNoRegister(t *testing.T) {
	h, _ := newTestHandler(t)
	srv := httptest.NewServer(h.Routes())
	t.Cleanup(srv.Close)
	res, err := http.Post(srv.URL+"/auth/register", "application/json", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode == http.StatusOK {
		t.Fatal("register must not exist")
	}
}

func TestStaticAssetCSSMime(t *testing.T) {
	h, _ := newTestHandler(t)
	srv := httptest.NewServer(h.Routes())
	t.Cleanup(srv.Close)

	res, err := http.Get(srv.URL + "/dashboard/")
	if err != nil {
		t.Fatal(err)
	}
	html, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	href := ""
	const marker = `href="/dashboard/`
	if i := bytes.Index(html, []byte(marker)); i >= 0 {
		rest := html[i+len(marker):]
		end := bytes.IndexByte(rest, '"')
		if end > 0 {
			href = "/dashboard/" + string(rest[:end])
		}
	}
	if href == "" || !strings.HasSuffix(href, ".css") {
		t.Fatalf("no css href in index.html:\n%s", html)
	}
	res, err = http.Get(srv.URL + href)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	ct := res.Header.Get("Content-Type")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("css %s: %d", href, res.StatusCode)
	}
	if !strings.HasPrefix(ct, "text/css") {
		t.Fatalf("css Content-Type %q, want text/css", ct)
	}
}
