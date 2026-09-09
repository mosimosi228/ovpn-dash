package httpserver

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode"
)

func validEmail(s string) bool {
	s = strings.TrimSpace(s)
	at := strings.LastIndex(s, "@")
	if at < 1 || at >= len(s)-1 {
		return false
	}
	if strings.ContainsAny(s, " \t\r\n") {
		return false
	}
	for _, r := range s {
		if r > unicode.MaxASCII {
			return false
		}
	}
	return strings.Contains(s[at+1:], ".")
}

func normalizeEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func hashOpaque(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func randomDigits(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		out[i] = '0' + (b[i] % 10)
	}
	return string(out), nil
}

func randomHex(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func publicOrigin(rHost, protoHdr, hostHdr string) string {
	proto := strings.TrimSpace(protoHdr)
	if proto == "" {
		proto = "https"
	}
	host := strings.TrimSpace(hostHdr)
	if host == "" {
		host = strings.TrimSpace(rHost)
	}
	if host == "" {
		return ""
	}
	return proto + "://" + host
}
