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
