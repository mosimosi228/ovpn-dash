package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mosimosi228/ovpn-dash/internal/mailer"
	"github.com/mosimosi228/ovpn-dash/internal/setup"
)

func authJSON(t *testing.T, srv *httptest.Server, token, method, path string, body any) *http.Response {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, srv.URL+path, r)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestWizardSuggestsLivePaths(t *testing.T) {
	h, _ := newTestHandler(t)
	srv := httptest.NewServer(h.Routes())
	t.Cleanup(srv.Close)
	res, err := http.Get(srv.URL + "/dashboard/api/state")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("state %d", res.StatusCode)
	}
	var st map[string]any
	if err := json.NewDecoder(res.Body).Decode(&st); err != nil {
		t.Fatal(err)
	}
	if st["pki_dir"] != setup.DefaultPKIDir {
		t.Fatalf("pki %v", st["pki_dir"])
	}
	if st["server_conf"] != setup.DefaultServerConf || st["unit"] != setup.DefaultUnit {
		t.Fatalf("paths %+v", st)
	}
}

func TestMultiUserCertAndRBAC(t *testing.T) {
	h, dir := newTestHandler(t)
	srv := httptest.NewServer(h.Routes())
	t.Cleanup(srv.Close)
	rootTok, _, _ := setupAndToken(t, srv, dir)

	res := authJSON(t, srv, rootTok, http.MethodPost, "/api/v1/users", map[string]string{
		"email":       "alice@example.com",
		"name":        "Alice",
		"password":    "alicepass",
		"role":        "user",
		"client_name": "alice",
	})
	raw, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create user %d %s", res.StatusCode, raw)
	}

	loginBody, _ := json.Marshal(map[string]string{"email": "alice@example.com", "password": "alicepass"})
	res, err := http.Post(srv.URL+"/auth/login", "application/json", bytes.NewReader(loginBody))
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
		t.Fatal("alice login")
	}

	res = authJSON(t, srv, tokens.AccessToken, http.MethodGet, "/api/v1/server", nil)
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("user server %d", res.StatusCode)
	}
	res = authJSON(t, srv, tokens.AccessToken, http.MethodGet, "/api/v1/users", nil)
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("user list users %d", res.StatusCode)
	}
	res = authJSON(t, srv, tokens.AccessToken, http.MethodGet, "/api/v1/me/ovpn", nil)
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("own ovpn %d %s", res.StatusCode, body)
	}

	res = authJSON(t, srv, rootTok, http.MethodDelete, "/api/v1/clients/alice", nil)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("revoke %d", res.StatusCode)
	}
	res, err = http.Post(srv.URL+"/auth/login", "application/json", bytes.NewReader(loginBody))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("disabled login %d", res.StatusCode)
	}

	res = authJSON(t, srv, rootTok, http.MethodPost, "/api/v1/users", map[string]string{
		"email":    "ops@example.com",
		"name":     "Ops",
		"password": "opspassword",
		"role":     "admin",
	})
	raw, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create admin %d %s", res.StatusCode, raw)
	}
}

func TestForgotWithoutSMTP(t *testing.T) {
	h, dir := newTestHandler(t)
	srv := httptest.NewServer(h.Routes())
	t.Cleanup(srv.Close)
	setupAndToken(t, srv, dir)
	body, _ := json.Marshal(map[string]string{"email": "admin@example.com"})
	res, err := http.Post(srv.URL+"/auth/forgot", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("forgot %d", res.StatusCode)
	}
}

func TestPINLogin(t *testing.T) {
	h, dir := newTestHandler(t)
	_ = dir
	var sent string
	h.TGSend = func(token, chatID, text string) error {
		sent = text
		return nil
	}
	srv := httptest.NewServer(h.Routes())
	t.Cleanup(srv.Close)
	setupAndToken(t, srv, dir)
	ctx := context.Background()

	body, _ := json.Marshal(map[string]string{"email": "admin@example.com"})
	res, err := http.Post(srv.URL+"/auth/login/pin", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("unbound pin %d", res.StatusCode)
	}

	_ = h.DB.SetMeta(ctx, setup.KeyTelegramBotToken, "123:test")
	u, err := h.DB.GetUserByEmail(ctx, "admin@example.com")
	if err != nil {
		t.Fatal(err)
	}
	u.TelegramChatID = "42"
	if err := h.DB.UpdateUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	res, err = http.Post(srv.URL+"/auth/login/pin", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("send pin %d %s", res.StatusCode, raw)
	}
	if !strings.Contains(sent, "PIN") {
		t.Fatalf("telegram text %q", sent)
	}
	pin := extractPIN(sent)
	if pin == "" {
		t.Fatalf("no pin in %q", sent)
	}
	verify, _ := json.Marshal(map[string]string{"email": "admin@example.com", "pin": pin})
	res, err = http.Post(srv.URL+"/auth/login/pin/verify", "application/json", bytes.NewReader(verify))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("verify %d %s", res.StatusCode, b)
	}
}

func TestResetPassword(t *testing.T) {
	h, dir := newTestHandler(t)
	_ = dir
	var mailBody string
	h.MailSend = func(cfg mailer.Config, to, subject, body string) error {
		mailBody = body
		return nil
	}
	srv := httptest.NewServer(h.Routes())
	t.Cleanup(srv.Close)
	setupAndToken(t, srv, dir)
	ctx := context.Background()
	_ = h.DB.SetMeta(ctx, setup.KeySMTPHost, "smtp.example.com")
	_ = h.DB.SetMeta(ctx, setup.KeySMTPFrom, "noreply@example.com")

	req, _ := json.Marshal(map[string]string{"email": "admin@example.com"})
	res, err := http.Post(srv.URL+"/auth/forgot", "application/json", bytes.NewReader(req))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("forgot %d", res.StatusCode)
	}
	idx := strings.Index(mailBody, "reset=")
	if idx < 0 {
		t.Fatalf("mail %q", mailBody)
	}
	token := strings.TrimSpace(mailBody[idx+len("reset="):])
	if i := strings.IndexAny(token, " \n"); i >= 0 {
		token = token[:i]
	}
	reset, _ := json.Marshal(map[string]string{"token": token, "password": "resetpass"})
	res, err = http.Post(srv.URL+"/auth/reset", "application/json", bytes.NewReader(reset))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("reset %d %s", res.StatusCode, raw)
	}
	loginBody, _ := json.Marshal(map[string]string{"email": "admin@example.com", "password": "resetpass"})
	res, err = http.Post(srv.URL+"/auth/login", "application/json", bytes.NewReader(loginBody))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("login after reset %d", res.StatusCode)
	}
}

func extractPIN(s string) string {
	for _, p := range strings.Fields(s) {
		if len(p) != 6 {
			continue
		}
		ok := true
		for _, r := range p {
			if r < '0' || r > '9' {
				ok = false
			}
		}
		if ok {
			return p
		}
	}
	return ""
}

func TestClientWithoutEmailAndAdminKeepsCert(t *testing.T) {
	h, dir := newTestHandler(t)
	srv := httptest.NewServer(h.Routes())
	t.Cleanup(srv.Close)
	rootTok, pkiDir, _ := setupAndToken(t, srv, dir)

	res := authJSON(t, srv, rootTok, http.MethodPost, "/api/v1/users", map[string]string{
		"name":        "Carol",
		"password":    "carolpass",
		"role":        "user",
		"client_name": "carol",
	})
	raw, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create %d %s", res.StatusCode, raw)
	}
	if _, err := os.Stat(filepath.Join(pkiDir, "issued", "carol.crt")); err != nil {
		t.Fatal(err)
	}
	loginBody, _ := json.Marshal(map[string]string{"username": "carol", "password": "carolpass"})
	res, err := http.Post(srv.URL+"/auth/login", "application/json", bytes.NewReader(loginBody))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("login by client name %d", res.StatusCode)
	}

	res = authJSON(t, srv, rootTok, http.MethodPost, "/api/v1/users", map[string]string{
		"email":       "dave@example.com",
		"name":        "Dave",
		"password":    "davepass1",
		"role":        "user",
		"client_name": "dave",
	})
	res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatal(res.StatusCode)
	}
	res = authJSON(t, srv, rootTok, http.MethodPost, "/api/v1/users", map[string]string{
		"email":       "dave@example.com",
		"name":        "Dave Two",
		"password":    "davepass1",
		"role":        "user",
		"client_name": "dave2",
	})
	raw, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode == http.StatusCreated {
		t.Fatalf("duplicate email created a user: %s", raw)
	}
	if _, err := os.Stat(filepath.Join(pkiDir, "issued", "dave2.crt")); !os.IsNotExist(err) {
		t.Fatalf("duplicate email still issued a cert: %v", err)
	}

	res = authJSON(t, srv, rootTok, http.MethodGet, "/api/v1/users", nil)
	raw, _ = io.ReadAll(res.Body)
	res.Body.Close()
	var list struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		t.Fatal(err)
	}
	var id float64
	for _, it := range list.Items {
		if it["client_name"] == "dave" {
			id, _ = it["id"].(float64)
		}
	}
	if id == 0 {
		t.Fatalf("dave missing: %s", raw)
	}
	res = authJSON(t, srv, rootTok, http.MethodPatch, "/api/v1/users/"+strconv.FormatInt(int64(id), 10), map[string]string{"role": "admin"})
	raw, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("promote %d %s", res.StatusCode, raw)
	}
	var promoted map[string]any
	if err := json.Unmarshal(raw, &promoted); err != nil {
		t.Fatal(err)
	}
	if promoted["role"] != "admin" || promoted["client_name"] != "dave" {
		t.Fatalf("%s", raw)
	}
}
