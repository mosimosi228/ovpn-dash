package ovpn

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseManagementTCP(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "server.conf")
	pw := filepath.Join(dir, "mgmt.pw")
	if err := os.WriteFile(pw, []byte("secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	body := "port 1194\nmanagement 127.0.0.1 7505 " + pw + "\n"
	if err := os.WriteFile(conf, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := ParseFile(conf)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.CanKill() || cfg.ManagementNet != "tcp" || cfg.ManagementAddr != "127.0.0.1:7505" {
		t.Fatalf("%+v", cfg)
	}
	if cfg.ManagementPass != pw {
		t.Fatalf("pass %s", cfg.ManagementPass)
	}
}

func TestParseManagementUnix(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "server.conf")
	sock := filepath.Join(dir, "manage.sock")
	body := "management " + sock + " unix\n"
	if err := os.WriteFile(conf, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := ParseFile(conf)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ManagementNet != "unix" || cfg.ManagementAddr != sock {
		t.Fatalf("%+v", cfg)
	}
}

func TestKillTCP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	errc := make(chan error, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			errc <- err
			return
		}
		defer c.Close()
		if _, err := fmt.Fprintf(c, ">INFO:OpenVPN Management Interface Version 3\n"); err != nil {
			errc <- err
			return
		}
		r := bufio.NewReader(c)
		line, err := r.ReadString('\n')
		if err != nil {
			errc <- err
			return
		}
		if strings.TrimSpace(line) != "kill 203.0.113.10:1234" {
			errc <- fmt.Errorf("cmd %q", line)
			return
		}
		_, _ = fmt.Fprintf(c, "SUCCESS: common name 'alice' found, 1 client(s) killed\n")
		errc <- nil
	}()
	cfg := &Config{ManagementNet: "tcp", ManagementAddr: ln.Addr().String()}
	if err := cfg.Kill("203.0.113.10:1234", "alice"); err != nil {
		t.Fatal(err)
	}
	if err := <-errc; err != nil {
		t.Fatal(err)
	}
}

func TestKillRejectsBadTarget(t *testing.T) {
	cfg := &Config{ManagementNet: "tcp", ManagementAddr: "127.0.0.1:1"}
	if err := cfg.Kill("foo\nbar", ""); err == nil {
		t.Fatal("expected error")
	}
}

func TestKillNoManage(t *testing.T) {
	cfg := &Config{}
	if err := cfg.Kill("", "alice"); err == nil || err.Error() != "nomanage" {
		t.Fatalf("%v", err)
	}
}
