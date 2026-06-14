package ssllabs

import (
	"testing"

	"github.com/tamnd/any-cli/kit"
)

// domain_test.go covers the URI driver's pure string functions and host wiring.
// No network is touched.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "ssllabs" {
		t.Errorf("Scheme = %q, want ssllabs", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "ssllabs" {
		t.Errorf("Identity.Binary = %q, want ssllabs", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		in  string
		typ string
		id  string
	}{
		{"google.com", "host", "google.com"},
		{"google.com:443", "host", "google.com"},
		{"https://google.com", "host", "google.com"},
		{"https://example.com/some/path", "host", "example.com"},
		{"github.com", "host", "github.com"},
	}
	for _, tc := range cases {
		typ, id, err := Domain{}.Classify(tc.in)
		if err != nil || typ != tc.typ || id != tc.id {
			t.Errorf("Classify(%q) = (%q, %q, %v), want (%q, %q, nil)",
				tc.in, typ, id, err, tc.typ, tc.id)
		}
	}
}

func TestClassifyEmpty(t *testing.T) {
	_, _, err := Domain{}.Classify("")
	if err == nil {
		t.Error("Classify(\"\") should return an error")
	}
}

func TestLocate(t *testing.T) {
	got, err := Domain{}.Locate("host", "google.com")
	want := SiteURL + "?d=google.com"
	if err != nil || got != want {
		t.Errorf("Locate(host, google.com) = (%q, %v), want (%q, nil)", got, err, want)
	}
}

func TestLocateInfo(t *testing.T) {
	got, err := Domain{}.Locate("info", "2.2.0")
	if err != nil || got == "" {
		t.Errorf("Locate(info, ...) = (%q, %v), want non-empty url", got, err)
	}
}

func TestLocateUnknown(t *testing.T) {
	_, err := Domain{}.Locate("unknown", "foo")
	if err == nil {
		t.Error("Locate(unknown, ...) should return an error")
	}
}

func TestNormalizeHost(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"google.com", "google.com"},
		{"google.com:443", "google.com"},
		{"https://google.com", "google.com"},
		{"https://google.com:8443/path", "google.com"},
		{" example.com ", "example.com"},
	}
	for _, tc := range cases {
		got := normalizeHost(tc.in)
		if got != tc.want {
			t.Errorf("normalizeHost(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestHostWiring(t *testing.T) {
	h, err := kit.Open()
	if err != nil {
		t.Fatal(err)
	}

	a := &Assessment{Host: "google.com", Port: 443, Status: "READY"}
	u, err := h.Mint(a)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if want := "ssllabs://host/google.com"; u.String() != want {
		t.Errorf("Mint = %q, want %q", u.String(), want)
	}

	got, err := h.ResolveOn("ssllabs", "github.com")
	if err != nil {
		t.Fatalf("ResolveOn: %v", err)
	}
	if got.String() != "ssllabs://host/github.com" {
		t.Errorf("ResolveOn = %q, want ssllabs://host/github.com", got.String())
	}
}
