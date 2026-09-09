package pki

import (
	"os"
	"testing"

	"github.com/mosimosi228/ovpn-dash/internal/setup"
)

func TestLivePKIReadable(t *testing.T) {
	if _, err := os.Stat(setup.DefaultPKIDir + "/ca.crt"); err != nil {
		t.Skip("live OpenVPN PKI not present")
	}
	st := &Store{Dir: setup.DefaultPKIDir}
	if _, _, err := st.loadCA(); err != nil {
		t.Fatalf("load live CA: %v", err)
	}
	if _, err := st.List(); err != nil {
		t.Fatalf("list live clients: %v", err)
	}
}
