package ssllabs_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tamnd/ssllabs-cli/ssllabs"
)

// ssllabs_test.go covers the HTTP client and API decoding with local test servers.
// No real network calls are made.

func newTestClient(srv *httptest.Server) *ssllabs.Client {
	c := ssllabs.NewClient()
	c.Rate = 0 // no pacing in tests
	c.HTTP = &http.Client{Timeout: 5 * time.Second}
	return c
}

func TestGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request carried no User-Agent")
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := ssllabs.NewClient()
	c.Rate = 0

	body, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "ok" {
		t.Errorf("body = %q, want %q", body, "ok")
	}
}

func TestGetRetriesOn503(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("recovered"))
	}))
	defer srv.Close()

	c := ssllabs.NewClient()
	c.Rate = 0
	c.Retries = 5

	start := time.Now()
	body, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "recovered" {
		t.Errorf("body = %q after retries", body)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Error("retries did not back off")
	}
}

func TestGetNonRetryableError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := ssllabs.NewClient()
	c.Rate = 0

	_, err := c.Get(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}

func TestInfoDecode(t *testing.T) {
	payload := `{
		"version":"2.2.0",
		"criteriaVersion":"2009q",
		"maxAssessments":25,
		"currentAssessments":3
	}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/info" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	// patch the base URL via a custom client whose HTTP transport redirects to srv
	c := &ssllabs.Client{
		HTTP:    &http.Client{Timeout: 5 * time.Second},
		Rate:    0,
		Retries: 0,
	}

	// We call the raw Get to validate the wire decoding path.
	b, err := c.Get(context.Background(), srv.URL+"/api/v3/info")
	if err != nil {
		t.Fatal(err)
	}
	var raw struct {
		Version            string `json:"version"`
		CriteriaVersion    string `json:"criteriaVersion"`
		MaxAssessments     int    `json:"maxAssessments"`
		CurrentAssessments int    `json:"currentAssessments"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if raw.Version != "2.2.0" {
		t.Errorf("Version = %q, want 2.2.0", raw.Version)
	}
	if raw.MaxAssessments != 25 {
		t.Errorf("MaxAssessments = %d, want 25", raw.MaxAssessments)
	}
	if raw.CurrentAssessments != 3 {
		t.Errorf("CurrentAssessments = %d, want 3", raw.CurrentAssessments)
	}
}

func TestAnalyzeDecode(t *testing.T) {
	payload := `{
		"host":"google.com",
		"port":443,
		"status":"READY",
		"startTime":1718300000000,
		"testTime":1718300005000,
		"endpoints":[
			{
				"ipAddress":"142.251.36.14",
				"grade":"B",
				"statusMessage":"Ready",
				"duration":5124,
				"progress":100
			}
		]
	}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/analyze" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		host := r.URL.Query().Get("host")
		if host != "google.com" {
			t.Errorf("host query param = %q, want google.com", host)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	c := &ssllabs.Client{
		HTTP:    &http.Client{Timeout: 5 * time.Second},
		Rate:    0,
		Retries: 0,
	}
	b, err := c.Get(context.Background(), srv.URL+"/api/v3/analyze?host=google.com&startNew=off&fromCache=on&maxAge=12")
	if err != nil {
		t.Fatal(err)
	}
	var raw struct {
		Host      string `json:"host"`
		Port      int    `json:"port"`
		Status    string `json:"status"`
		Endpoints []struct {
			IPAddress     string `json:"ipAddress"`
			Grade         string `json:"grade"`
			StatusMessage string `json:"statusMessage"`
			Duration      int    `json:"duration"`
			Progress      int    `json:"progress"`
		} `json:"endpoints"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if raw.Host != "google.com" {
		t.Errorf("Host = %q, want google.com", raw.Host)
	}
	if raw.Status != "READY" {
		t.Errorf("Status = %q, want READY", raw.Status)
	}
	if len(raw.Endpoints) != 1 {
		t.Fatalf("len(Endpoints) = %d, want 1", len(raw.Endpoints))
	}
	ep := raw.Endpoints[0]
	if ep.IPAddress != "142.251.36.14" {
		t.Errorf("IPAddress = %q, want 142.251.36.14", ep.IPAddress)
	}
	if ep.Grade != "B" {
		t.Errorf("Grade = %q, want B", ep.Grade)
	}
	if ep.Progress != 100 {
		t.Errorf("Progress = %d, want 100", ep.Progress)
	}
}

func TestNewClientDefaults(t *testing.T) {
	c := ssllabs.NewClient()
	if c.Rate != 2*time.Second {
		t.Errorf("Rate = %v, want 2s", c.Rate)
	}
	if c.Retries != 5 {
		t.Errorf("Retries = %d, want 5", c.Retries)
	}
	if c.HTTP == nil {
		t.Error("HTTP client is nil")
	}
}
