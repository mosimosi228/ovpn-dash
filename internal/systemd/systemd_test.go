package systemd

import "testing"

func TestValidateUnit(t *testing.T) {
	if err := ValidateUnit("openvpn-server@server"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateUnit("openvpn@server.service"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateUnit("bad unit;rm"); err == nil {
		t.Fatal("expected error")
	}
	if err := ValidateUnit(""); err == nil {
		t.Fatal("expected error")
	}
}
