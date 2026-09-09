package ovpn

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseStatusClassic(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "status.log")
	body := `OpenVPN CLIENT LIST
Updated,Thu Aug 13 12:00:00 2026
Common Name,Real Address,Bytes Received,Bytes Sent,Connected Since
alice,203.0.113.10:54321,100,200,Thu Aug 13 11:00:00 2026
bob,198.51.100.8:1194,10,20,Thu Aug 13 11:30:00 2026
ROUTING TABLE
Virtual Address,Common Name,Real Address,Last Ref
10.8.0.2,alice,203.0.113.10:54321,Thu Aug 13 12:00:00 2026
10.8.0.3,bob,198.51.100.8:1194,Thu Aug 13 12:00:00 2026
GLOBAL STATS
Max bcast/mcast queue length,0
END
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	ss, err := ParseStatusFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(ss) != 2 {
		t.Fatalf("len %d", len(ss))
	}
	if ss[0].Name != "alice" || ss[0].RealIP != "203.0.113.10" || ss[0].VirtualIP != "10.8.0.2" {
		t.Fatalf("%+v", ss[0])
	}
	if ss[0].LastRef == "" {
		t.Fatal("last ref")
	}
	if ss[0].BytesReceived != 100 || ss[0].BytesSent != 200 {
		t.Fatalf("bytes %+v", ss[0])
	}
}

func TestParseStatusV2(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "status.log")
	body := `TITLE,OpenVPN
TIME,Thu Aug 13 12:00:00 2026,1690000000
HEADER,CLIENT_LIST,Common Name,Real Address,Virtual Address,Virtual IPv6 Address,Bytes Received,Bytes Sent,Connected Since,Connected Since (time_t),Username
CLIENT_LIST,alice,203.0.113.10:1234,10.8.0.2,,111,222,Thu Aug 13 11:00:00 2026,1690000000,alice
END
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	ss, err := ParseStatusFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(ss) != 1 || ss[0].Name != "alice" || ss[0].RealIP != "203.0.113.10" || ss[0].BytesReceived != 111 {
		t.Fatalf("%+v", ss)
	}
}

func TestParseStatusV3UNDEFUsername(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "status.log")
	body := `TITLE,OpenVPN 2.5.11
TIME,2026-09-09 15:38:17,1788957497
HEADER,CLIENT_LIST,Common Name,Real Address,Virtual Address,Virtual IPv6 Address,Bytes Received,Bytes Sent,Connected Since,Connected Since (time_t),Username,Client ID,Peer ID,Data Channel Cipher
CLIENT_LIST,fuck,192.168.15.122:59338,10.8.0.3,,4212,4047,2026-09-09 15:38:17,1788957497,UNDEF,0,0,AES-256-GCM
HEADER,ROUTING_TABLE,Virtual Address,Common Name,Real Address,Last Ref,Last Ref (time_t)
ROUTING_TABLE,10.8.0.3,fuck,192.168.15.122:59338,2026-09-09 15:38:17,1788957497
GLOBAL_STATS,Max bcast/mcast queue length,0
END
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	ss, err := ParseStatusFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(ss) != 1 || ss[0].Name != "fuck" || ss[0].VirtualIP != "10.8.0.3" || ss[0].RealIP != "192.168.15.122" {
		t.Fatalf("%+v", ss)
	}
}

func TestRuntimeStatusFile(t *testing.T) {
	if got := RuntimeStatusFile("openvpn-server@server"); got != "/run/openvpn-server/status-server.log" {
		t.Fatalf("%s", got)
	}
	if RuntimeStatusFile("openvpn.service") != "" {
		t.Fatal("no instance")
	}
}

func TestParseBestStatusPrefersPopulated(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty.log")
	live := filepath.Join(dir, "live.log")
	if err := os.WriteFile(empty, []byte("TITLE,OpenVPN\nEND\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(live, []byte("CLIENT_LIST,alice,203.0.113.10:1,10.8.0.2,,1,2,now,1,alice\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ss, used, err := ParseBestStatus([]string{empty, live})
	if err != nil {
		t.Fatal(err)
	}
	if used != live || len(ss) != 1 || ss[0].Name != "alice" {
		t.Fatalf("%s %+v", used, ss)
	}
}
