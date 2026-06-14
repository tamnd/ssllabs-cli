// Package ssllabs is the library behind the ssllabs command line:
// the HTTP client, request shaping, and the typed data models for the
// SSL Labs API (api.ssllabs.com/api/v3).
//
// The Client sets a real User-Agent, paces requests at 2 s intervals
// (SSL Labs enforces concurrency limits), and retries transient 429 / 5xx
// failures. Build your endpoint calls on top of Get.
package ssllabs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// DefaultUserAgent identifies the client to SSL Labs.
const DefaultUserAgent = "ssllabs-cli/dev (+https://github.com/tamnd/ssllabs-cli)"

// Host is the domain this driver claims in the URI scheme.
const Host = "api.ssllabs.com"

// BaseURL is the root every API request is built from.
const BaseURL = "https://api.ssllabs.com/api/v3"

// SiteURL is the human-readable SSL Labs test page used by Locate.
const SiteURL = "https://www.ssllabs.com/ssltest/analyze.html"

// Config carries tunable client parameters.
type Config struct {
	UserAgent string
	Rate      time.Duration
	Timeout   time.Duration
	Retries   int
}

// DefaultConfig returns the recommended defaults.
func DefaultConfig() Config {
	return Config{
		UserAgent: DefaultUserAgent,
		Rate:      2 * time.Second, // SSL Labs enforces concurrency limits
		Timeout:   60 * time.Second,
		Retries:   5,
	}
}

// Client talks to the SSL Labs API over HTTPS.
type Client struct {
	HTTP      *http.Client
	UserAgent string
	// Rate is the minimum gap between requests.
	Rate    time.Duration
	Retries int

	last time.Time
}

// NewClient returns a Client with the default configuration.
func NewClient() *Client {
	cfg := DefaultConfig()
	return &Client{
		HTTP:      &http.Client{Timeout: cfg.Timeout},
		UserAgent: cfg.UserAgent,
		Rate:      cfg.Rate,
		Retries:   cfg.Retries,
	}
}

// Get fetches url and returns the response body. It paces and retries
// according to the client's settings.
func (c *Client) Get(ctx context.Context, rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, rawURL)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", rawURL, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) (body []byte, retry bool, err error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.UserAgent)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

// pace blocks until at least Rate has passed since the previous request.
func (c *Client) pace() {
	if c.Rate <= 0 {
		return
	}
	if wait := c.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

// --- Output types ---

// Info holds metadata returned by the SSL Labs /info endpoint.
type Info struct {
	Version            string `kit:"id" json:"version"`
	CriteriaVersion    string `json:"criteria_version"`
	MaxAssessments     int    `json:"max_assessments"`
	CurrentAssessments int    `json:"current_assessments"`
}

// Assessment is the top-level result from /analyze.
type Assessment struct {
	Host      string     `kit:"id" json:"host"`
	Port      int        `json:"port"`
	Status    string     `json:"status"`
	StartTime int64      `json:"start_time"`
	Endpoints []Endpoint `json:"endpoints"`
}

// Endpoint is one IP address inside an Assessment.
type Endpoint struct {
	IPAddress string `json:"ip_address"`
	Grade     string `json:"grade"`
	Status    string `json:"status"`
	Duration  int    `json:"duration_ms"`
	Progress  int    `json:"progress"`
}

// --- wire shapes (match the actual API JSON field names) ---

type wireInfo struct {
	Version            string `json:"version"`
	CriteriaVersion    string `json:"criteriaVersion"`
	MaxAssessments     int    `json:"maxAssessments"`
	CurrentAssessments int    `json:"currentAssessments"`
}

type wireAssessment struct {
	Host      string         `json:"host"`
	Port      int            `json:"port"`
	Status    string         `json:"status"`
	StartTime int64          `json:"startTime"`
	Endpoints []wireEndpoint `json:"endpoints"`
}

type wireEndpoint struct {
	IPAddress     string `json:"ipAddress"`
	Grade         string `json:"grade"`
	StatusMessage string `json:"statusMessage"`
	Duration      int    `json:"duration"`
	Progress      int    `json:"progress"`
}

// --- API methods ---

// Info calls /info and returns API metadata.
func (c *Client) Info(ctx context.Context) (*Info, error) {
	b, err := c.Get(ctx, BaseURL+"/info")
	if err != nil {
		return nil, err
	}
	var w wireInfo
	if err := json.Unmarshal(b, &w); err != nil {
		return nil, fmt.Errorf("info: decode: %w", err)
	}
	return &Info{
		Version:            w.Version,
		CriteriaVersion:    w.CriteriaVersion,
		MaxAssessments:     w.MaxAssessments,
		CurrentAssessments: w.CurrentAssessments,
	}, nil
}

// Analyze calls /analyze for the given hostname.
// When fromCache is true it passes startNew=off&fromCache=on&maxAge=12.
// When fromCache is false it passes startNew=on.
func (c *Client) Analyze(ctx context.Context, host string, fromCache bool) (*Assessment, error) {
	var rawURL string
	if fromCache {
		rawURL = BaseURL + "/analyze?" + url.Values{
			"host":      {host},
			"startNew":  {"off"},
			"fromCache": {"on"},
			"maxAge":    {"12"},
		}.Encode()
	} else {
		rawURL = BaseURL + "/analyze?" + url.Values{
			"host":     {host},
			"startNew": {"on"},
		}.Encode()
	}

	b, err := c.Get(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	var w wireAssessment
	if err := json.Unmarshal(b, &w); err != nil {
		return nil, fmt.Errorf("analyze: decode: %w", err)
	}
	a := &Assessment{
		Host:      w.Host,
		Port:      w.Port,
		Status:    w.Status,
		StartTime: w.StartTime,
	}
	for _, e := range w.Endpoints {
		a.Endpoints = append(a.Endpoints, Endpoint{
			IPAddress: e.IPAddress,
			Grade:     e.Grade,
			Status:    e.StatusMessage,
			Duration:  e.Duration,
			Progress:  e.Progress,
		})
	}
	return a, nil
}
