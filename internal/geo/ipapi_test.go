package geo

import "testing"

func TestLookupSkipsPrivate(t *testing.T) {
	a := New()
	if a.Lookup("10.8.0.2") != nil {
		t.Fatal("private")
	}
	if a.Lookup("127.0.0.1") != nil {
		t.Fatal("loopback")
	}
	if a.Lookup("not-an-ip") != nil {
		t.Fatal("garbage")
	}
}
