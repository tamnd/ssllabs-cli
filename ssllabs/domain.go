package ssllabs

import (
	"context"
	"net/url"
	"strings"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes the SSL Labs API as a kit Domain. A multi-domain host (ant)
// enables it with a single blank import:
//
//	import _ "github.com/tamnd/ssllabs-cli/ssllabs"
//
// The init below registers it; the host routes ssllabs:// URIs to the ops
// Register installs. The same Domain also builds the standalone ssllabs binary
// (see cmd/), so the binary and a host share one source of truth.
func init() { kit.Register(Domain{}) }

// Domain is the SSL Labs driver.
type Domain struct{}

// Info describes the scheme, the matched hostnames, and the binary identity.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "ssllabs",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "ssllabs",
			Short:  "SSL Labs API command line.",
			Long: `A command line for the SSL Labs API (api.ssllabs.com/api/v3).

ssllabs analyzes TLS configurations of public HTTPS servers and returns
structured results. No API key required.`,
			Site: "www.ssllabs.com",
			Repo: "https://github.com/tamnd/ssllabs-cli",
		},
	}
}

// Register installs the client factory and every operation.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{
		Name: "info", Group: "read", Single: true,
		Summary: "Show SSL Labs API info and current rate limits",
		URIType: "info", Resolver: true,
	}, handleInfo)

	kit.Handle(app, kit.OpMeta{
		Name: "analyze", Group: "read", Single: true,
		Summary:  "Analyze the TLS configuration of a hostname",
		URIType:  "host", Resolver: true,
		Args: []kit.Arg{{Name: "host", Help: "hostname to analyze (e.g. google.com)"}},
	}, handleAnalyze)
}

// newClient builds the injected *Client from kit.Config values.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := NewClient()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.HTTP.Timeout = cfg.Timeout
	}
	return c, nil
}

// --- inputs ---

type infoInput struct {
	Client *Client `kit:"inject"`
}

type analyzeInput struct {
	Host      string  `kit:"arg"  help:"hostname to analyze (e.g. google.com)"`
	FromCache bool    `kit:"flag" help:"use cached results if available" default:"true"`
	Client    *Client `kit:"inject"`
}

// --- handlers ---

func handleInfo(ctx context.Context, in infoInput, emit func(*Info) error) error {
	info, err := in.Client.Info(ctx)
	if err != nil {
		return mapErr(err)
	}
	return emit(info)
}

func handleAnalyze(ctx context.Context, in analyzeInput, emit func(*Assessment) error) error {
	host := normalizeHost(in.Host)
	a, err := in.Client.Analyze(ctx, host, in.FromCache)
	if err != nil {
		return mapErr(err)
	}
	return emit(a)
}

// --- Resolver: pure string functions, no network ---

// Classify turns a hostname or ssllabs.com URL into (type, id).
// "google.com" or "google.com:443" → ("host", "google.com")
func (Domain) Classify(input string) (uriType, id string, err error) {
	input = strings.TrimSpace(input)
	if u, err2 := url.Parse(input); err2 == nil &&
		(u.Scheme == "http" || u.Scheme == "https") {
		h := u.Hostname()
		if h != "" {
			return "host", h, nil
		}
	}
	// bare hostname or host:port
	host, _, _ := strings.Cut(input, ":")
	host = strings.Trim(host, "/")
	if host == "" {
		return "", "", errs.Usage("unrecognized SSL Labs reference: %q", input)
	}
	return "host", host, nil
}

// Locate returns the human-readable SSL Labs test URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "host":
		return SiteURL + "?d=" + url.QueryEscape(id), nil
	case "info":
		return "https://www.ssllabs.com/projects/ssllabs-apis/", nil
	default:
		return "", errs.Usage("ssllabs has no resource type %q", uriType)
	}
}

// --- helpers ---

// normalizeHost strips a scheme and any port from a user-supplied host string.
func normalizeHost(input string) string {
	input = strings.TrimSpace(input)
	if u, err := url.Parse(input); err == nil &&
		(u.Scheme == "http" || u.Scheme == "https") && u.Hostname() != "" {
		return u.Hostname()
	}
	host, _, _ := strings.Cut(input, ":")
	if host != "" {
		return strings.Trim(host, "/")
	}
	return strings.Trim(input, "/")
}

// mapErr converts library errors to kit error kinds with the right exit codes.
func mapErr(err error) error {
	return err
}
