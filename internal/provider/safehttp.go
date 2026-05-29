package provider

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// allowedWebhookHosts defines the strict allowlist for tenant-supplied webhook URLs.
// Exact matches are checked against the full host; suffix matches use the dot-prefixed
// form to prevent subdomain hijacking (e.g. ".webhook.office.com" matches
// "tenant.webhook.office.com" but not "evilwebhook.office.com").
//
// MS Teams "Workflows" connector URLs use different subdomains under office.com;
// add additional suffixes here when Microsoft publishes new endpoint patterns.
var allowedWebhookHosts = []struct {
	exact  string
	suffix string
}{
	{exact: "hooks.slack.com"},
	{suffix: ".webhook.office.com"},
}

// WebhookClient is the shared HTTP client for all tenant-controlled webhook
// deliveries. It enforces a 10-second total timeout and uses guardedDial to
// block SSRF to private, loopback, link-local, and ULA addresses.
//
// TLS correctness: only DialContext is customised. DialTLSContext is left nil
// so the standard Transport handles TLS, using the original request hostname
// for SNI and certificate verification. Dialling the resolved IP via DialContext
// does not affect certificate validation — the Transport still validates against
// the hostname in the URL.
var WebhookClient = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		DialContext:           guardedDial,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
	},
}

// guardedDial resolves addr, rejects any candidate IP that is loopback, private,
// link-local, ULA (fc00::/7), or unspecified, then dials the first allowed IP.
func guardedDial(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("guardedDial: invalid address %q: %w", addr, err)
	}

	ips, err := resolveIPs(ctx, host)
	if err != nil {
		return nil, err
	}

	for _, ip := range ips {
		if err := checkIP(ip); err != nil {
			return nil, err
		}
	}

	dialer := &net.Dialer{Timeout: 5 * time.Second}
	return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
}

// resolveIPs returns the IP addresses for host. If host is already an IP
// literal it is returned directly; otherwise DNS is queried.
func resolveIPs(ctx context.Context, host string) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}, nil
	}

	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("guardedDial: DNS lookup failed for %q: %w", host, err)
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("guardedDial: no addresses for %q", host)
	}

	ips := make([]net.IP, len(addrs))
	for i, a := range addrs {
		ips[i] = a.IP
	}
	return ips, nil
}

// checkIP returns an error if ip must be blocked for SSRF reasons.
func checkIP(ip net.IP) error {
	switch {
	case ip.IsLoopback():
		return fmt.Errorf("blocked address: %s (loopback)", ip)
	case ip.IsPrivate():
		return fmt.Errorf("blocked address: %s (private)", ip)
	case ip.IsLinkLocalUnicast():
		return fmt.Errorf("blocked address: %s (link-local unicast)", ip)
	case ip.IsLinkLocalMulticast():
		return fmt.Errorf("blocked address: %s (link-local multicast)", ip)
	case ip.IsUnspecified():
		return fmt.Errorf("blocked address: %s (unspecified)", ip)
	case isULA(ip):
		return fmt.Errorf("blocked address: %s (ULA)", ip)
	}
	return nil
}

// isULA reports whether ip is a IPv6 Unique Local Address (fc00::/7).
// net.IP.IsPrivate covers RFC 1918 and RFC 4193 in Go 1.17+, but we check
// explicitly to be defensive against any future stdlib changes.
func isULA(ip net.IP) bool {
	if ip4 := ip.To4(); ip4 != nil {
		return false
	}
	ip6 := ip.To16()
	if ip6 == nil {
		return false
	}
	return ip6[0]&0xfe == 0xfc
}

// ValidateWebhookURL ensures a tenant-supplied webhook URL is safe to store and deliver to.
// It requires HTTPS, a non-empty host, membership in the allowedWebhookHosts list,
// and that the host is not a private/loopback/link-local IP literal.
func ValidateWebhookURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid webhook URL: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("webhook URL must use https (got %q)", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("webhook URL must have a non-empty host")
	}

	// Reject IP literals that are private/loopback/link-local even when the
	// allowlist is later extended to cover broader patterns.
	if ip := net.ParseIP(host); ip != nil {
		if err := checkIP(ip); err != nil {
			return fmt.Errorf("webhook URL host rejected: %w", err)
		}
		return fmt.Errorf("webhook URL must be an allowlisted hostname, not an IP literal")
	}

	if !isAllowedHost(host) {
		return fmt.Errorf("webhook URL host %q is not in the allowed list (allowed: hooks.slack.com, *.webhook.office.com)", host)
	}
	return nil
}

// isAllowedHost returns true if host matches any entry in allowedWebhookHosts.
func isAllowedHost(host string) bool {
	host = strings.ToLower(host)
	for _, entry := range allowedWebhookHosts {
		if entry.exact != "" && host == entry.exact {
			return true
		}
		if entry.suffix != "" && strings.HasSuffix(host, entry.suffix) {
			return true
		}
	}
	return false
}
