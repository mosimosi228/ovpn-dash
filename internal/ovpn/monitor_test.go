package ovpn

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func TestMonitorSessionsAndKill(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	killed := make(chan string, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		_, _ = fmt.Fprintf(c, ">INFO:OpenVPN Management Interface Version 3\n")
		r := bufio.NewReader(c)
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			cmd := strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(cmd, "bytecount"):
				_, _ = fmt.Fprintf(c, "SUCCESS: bytecount interval changed\n")
			case cmd == "status 3":
				_, _ = fmt.Fprintf(c, "CLIENT_LIST,bob,203.0.113.9:1,10.8.0.9,,11,22,now,1,bob,7,0,AES\nEND\n")
			case strings.HasPrefix(cmd, "kill "), strings.HasPrefix(cmd, "client-kill "):
				killed <- strings.TrimPrefix(strings.TrimPrefix(cmd, "client-kill "), "kill ")
				_, _ = fmt.Fprintf(c, "SUCCESS: common name 'bob' found, 1 client(s) killed\n")
			}
		}
	}()

	m := NewMonitor()
	defer m.Close()
	m.Configure(&Config{ManagementNet: "tcp", ManagementAddr: ln.Addr().String()})
	if !m.WaitStatus(2 * time.Second) {
		t.Fatal("no status")
	}
	ss, ok := m.Sessions()
	if !ok || len(ss) != 1 || ss[0].Name != "bob" || ss[0].ClientID != 7 {
		t.Fatalf("%v %+v", ok, ss)
	}
	if err := m.Kill("", "bob", 0); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-killed:
		if got != "bob" {
			t.Fatalf("kill %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("kill not received")
	}
}

func TestMonitorKillIgnoresBytecountSuccess(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	killed := make(chan string, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		_, _ = fmt.Fprintf(c, ">INFO:OpenVPN Management Interface Version 3\n")
		r := bufio.NewReader(c)
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			cmd := strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(cmd, "bytecount"):
				_, _ = fmt.Fprintf(c, "SUCCESS: bytecount interval changed\n")
			case cmd == "status 3":
				_, _ = fmt.Fprintf(c, "CLIENT_LIST,bob,203.0.113.9:1,10.8.0.9,,11,22,now,1,bob,7,0,AES\nEND\n")
			case strings.HasPrefix(cmd, "kill "):
				_, _ = fmt.Fprintf(c, "SUCCESS: bytecount interval changed\n")
				killed <- strings.TrimPrefix(cmd, "kill ")
				_, _ = fmt.Fprintf(c, "SUCCESS: common name 'bob' found, 1 client(s) killed\n")
			}
		}
	}()

	m := NewMonitor()
	defer m.Close()
	m.Configure(&Config{ManagementNet: "tcp", ManagementAddr: ln.Addr().String()})
	if !m.WaitStatus(2 * time.Second) {
		t.Fatal("no status")
	}
	if err := m.Kill("203.0.113.9:1", "bob", 7); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-killed:
		if got != "bob" {
			t.Fatalf("kill %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("kill not received")
	}
}
