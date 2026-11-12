package mcpskills

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func normalizeEndpoint(endpoint string) (string, error) {
	normalized, err := domain.NormalizeWebSkillURL(endpoint)
	if err != nil || normalized != endpoint {
		return "", ErrProtocol
	}
	parsed, err := url.Parse(normalized)
	if err != nil || parsed.Hostname() == "" {
		return "", ErrProtocol
	}
	if port := parsed.Port(); port != "" && port != "443" {
		return "", ErrProtocol
	}
	// No credentials, raw query or fragment; NormalizeWebSkillURL checks those.
	return strings.ToLower(parsed.Scheme + "://" + parsed.Host), nil
}

// publicHTTPClient has the same public-network-only purpose as the catalogue's
// SafePublisherClient. No ambient proxies, authorization, cookies or redirects.
func publicHTTPClient() *http.Client {
	dialer := &net.Dialer{Timeout: 6 * time.Second}
	transport := &http.Transport{
		Proxy:                 nil,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		DisableCompression:    true,
		MaxIdleConns:          2,
		ResponseHeaderTimeout: 8 * time.Second,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil || port != "443" {
				return nil, errors.New("invalid public MCP destination")
			}
			// Bound DNS and all connection attempts by one shared budget.
			dialCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
			defer cancel()
			resolved, err := net.DefaultResolver.LookupIPAddr(dialCtx, host)
			if err != nil || len(resolved) == 0 {
				return nil, errors.New("MCP public DNS unavailable")
			}
			return dialPublicAddresses(dialCtx, port, resolved, dialer.DialContext)
		},
	}
	return &http.Client{Timeout: 12 * time.Second, Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

// dialPublicAddresses validates the entire DNS answer before attempting a
// connection. Every fallback uses the already validated literal address,
// never the original hostname (which could resolve differently on retry).
func dialPublicAddresses(ctx context.Context, port string, resolved []net.IPAddr, dial func(context.Context, string, string) (net.Conn, error)) (net.Conn, error) {
	if len(resolved) == 0 {
		return nil, errors.New("MCP public DNS unavailable")
	}
	for _, entry := range resolved {
		if !publicAddress(entry.IP) {
			return nil, errors.New("nonpublic MCP destination")
		}
	}
	var lastErr error
	for _, entry := range resolved {
		conn, err := dial(ctx, "tcp", net.JoinHostPort(entry.IP.String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	return nil, lastErr
}

func publicAddress(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	addr = addr.Unmap()
	if !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() ||
		addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() {
		return false
	}
	if addr.Is6() && !netip.MustParsePrefix("2000::/3").Contains(addr) {
		return false
	}
	for _, prefix := range nonPublicRanges {
		if prefix.Contains(addr) {
			return false
		}
	}
	return true
}

var nonPublicRanges = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"), netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"), netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"), netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"), netip.MustParsePrefix("2001::/32"),
	netip.MustParsePrefix("2001:10::/28"), netip.MustParsePrefix("2001:20::/28"),
	netip.MustParsePrefix("2001:db8::/32"), netip.MustParsePrefix("::/96"),
	netip.MustParsePrefix("::ffff:0:0:0/96"), netip.MustParsePrefix("100::/64"),
	netip.MustParsePrefix("2001:2::/48"), netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("3fff::/20"), netip.MustParsePrefix("5f00::/16"),
}
