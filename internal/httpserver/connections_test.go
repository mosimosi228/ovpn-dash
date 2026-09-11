package httpserver

import (
	"bufio"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestConnectionsWSRequiresAuth(t *testing.T) {
	h, dir := newTestHandler(t)
	srv := httptest.NewServer(h.Routes())
	t.Cleanup(srv.Close)
	_, _, _ = setupAndToken(t, srv, dir)

	u := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/v1/connections/ws"
	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.WriteJSON(map[string]string{"token": "nope"}); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	var msg map[string]string
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatal(err)
	}
	if msg["error"] != "unauthorized" {
		t.Fatalf("%s", raw)
	}
}

func TestConnectionsWSStreams(t *testing.T) {
	h, dir := newTestHandler(t)
	srv := httptest.NewServer(h.Routes())
	t.Cleanup(srv.Close)
	tok, _, conf := setupAndToken(t, srv, dir)

	status := filepath.Join(dir, "status.log")
	body := `OpenVPN CLIENT LIST
Updated,Thu Aug 13 12:00:00 2026
Common Name,Real Address,Bytes Received,Bytes Sent,Connected Since
alice,203.0.113.10:54321,100,200,Thu Aug 13 11:00:00 2026
ROUTING TABLE
Virtual Address,Common Name,Real Address,Last Ref
10.8.0.2,alice,203.0.113.10:54321,Thu Aug 13 12:00:00 2026
GLOBAL STATS
Max bcast/mcast queue length,0
END
`
	if err := os.WriteFile(status, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	prev, err := os.ReadFile(conf)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(conf, append(prev, []byte("\nstatus "+status+"\n")...), 0o644); err != nil {
		t.Fatal(err)
	}

	u := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/v1/connections/ws"
	conn, res, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if res.StatusCode != http.StatusSwitchingProtocols {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("upgrade %d %s", res.StatusCode, b)
	}
	if err := conn.WriteJSON(map[string]string{"token": tok}); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Items) != 1 || payload.Items[0]["name"] != "alice" {
		t.Fatalf("%s", raw)
	}
}

func TestKillConnection(t *testing.T) {
	h, dir := newTestHandler(t)
	srv := httptest.NewServer(h.Routes())
	t.Cleanup(srv.Close)
	tok, _, conf := setupAndToken(t, srv, dir)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	prev, err := os.ReadFile(conf)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(conf, append(prev, []byte("\nmanagement 127.0.0.1 "+port+"\n")...), 0o644); err != nil {
		t.Fatal(err)
	}

	killed := make(chan struct{}, 1)
	go serveFakeOpenVPNMgmt(ln, "CLIENT_LIST,alice,203.0.113.10:1,10.8.0.2,,1,2,now,1,x\nEND\n", killed)

	body, _ := json.Marshal(map[string]string{"name": "alice"})
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/connections/kill", strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("%d %s", res.StatusCode, raw)
	}
	select {
	case <-killed:
	case <-time.After(2 * time.Second):
		t.Fatal("kill was not sent")
	}
}

func TestKillConnectionNoManage(t *testing.T) {
	h, dir := newTestHandler(t)
	srv := httptest.NewServer(h.Routes())
	t.Cleanup(srv.Close)
	tok, _, _ := setupAndToken(t, srv, dir)

	body, _ := json.Marshal(map[string]string{"name": "alice"})
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/connections/kill", strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("%d %s", res.StatusCode, raw)
	}
	if !strings.Contains(string(raw), "nomanage") {
		t.Fatalf("%s", raw)
	}
}

func TestListConnectionsUsesManagement(t *testing.T) {
	h, dir := newTestHandler(t)
	srv := httptest.NewServer(h.Routes())
	t.Cleanup(srv.Close)
	tok, _, conf := setupAndToken(t, srv, dir)

	stale := filepath.Join(dir, "status.log")
	if err := os.WriteFile(stale, []byte("TITLE,OpenVPN\nEND\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	prev, err := os.ReadFile(conf)
	if err != nil {
		t.Fatal(err)
	}
	extra := "\nstatus " + stale + "\nmanagement 127.0.0.1 " + port + "\n"
	if err := os.WriteFile(conf, append(prev, []byte(extra)...), 0o644); err != nil {
		t.Fatal(err)
	}
	go serveFakeOpenVPNMgmt(ln, "CLIENT_LIST,live,198.51.100.9:9,10.8.0.9,,1,2,now,1,x\nEND\n", nil)

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/connections", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("%d %s", res.StatusCode, raw)
	}
	if !strings.Contains(string(raw), `"name":"live"`) {
		t.Fatalf("want live client from management, got %s", raw)
	}
	if !strings.Contains(string(raw), `"status_file":"management"`) {
		t.Fatalf("%s", raw)
	}
}

func serveFakeOpenVPNMgmt(ln net.Listener, statusBody string, killed chan struct{}) {
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go handleFakeOpenVPNMgmt(c, statusBody, killed)
		}
	}()
}

func handleFakeOpenVPNMgmt(c net.Conn, statusBody string, killed chan struct{}) {
	defer c.Close()
	_, _ = c.Write([]byte(">INFO:OpenVPN Management Interface Version 3\n"))
	r := bufio.NewReader(c)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(cmd, "bytecount"):
			_, _ = c.Write([]byte("SUCCESS: bytecount interval changed\n"))
		case cmd == "status 3" || cmd == "status":
			body := statusBody
			if !strings.HasSuffix(body, "\n") {
				body += "\n"
			}
			if !strings.Contains(body, "END") {
				body += "END\n"
			}
			_, _ = c.Write([]byte(body))
		case strings.HasPrefix(cmd, "kill "), strings.HasPrefix(cmd, "client-kill "):
			_, _ = c.Write([]byte("SUCCESS: common name found, 1 client(s) killed\n"))
			if killed != nil {
				select {
				case killed <- struct{}{}:
				default:
				}
			}
		}
	}
}
