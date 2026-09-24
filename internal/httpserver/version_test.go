package httpserver

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSemverNewer(t *testing.T) {
	if semverNewer("v1.2.3", "dev") {
		t.Fatal("dev should not compare")
	}
	if !semverNewer("v1.2.3", "v1.2.2") {
		t.Fatal("expected newer")
	}
	if semverNewer("v1.2.2", "1.2.2") {
		t.Fatal("same")
	}
	if semverNewer("v1.2.3-rc1", "v1.2.0") {
		t.Fatal("prerelease tag is ignored")
	}
}

func TestVersionEndpoint(t *testing.T) {
	prev := fetchLatestRelease
	fetchLatestRelease = func() string { return "v9.9.9" }
	t.Cleanup(func() { fetchLatestRelease = prev })

	h, _ := newTestHandler(t)
	h.Version = "v1.0.0"
	srv := httptest.NewServer(h.Routes())
	t.Cleanup(srv.Close)
	res, err := http.Get(srv.URL + "/api/v1/version")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("%d %s", res.StatusCode, raw)
	}
	var body map[string]string
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if body["version"] != "v1.0.0" || body["update"] != "v9.9.9" {
		t.Fatalf("%s", raw)
	}
	if body["repo"] != repoURL {
		t.Fatalf("repo %s", body["repo"])
	}
}
