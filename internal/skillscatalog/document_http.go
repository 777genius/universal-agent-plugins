package skillscatalog

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/internal/skillsresolve"
)

type WebsiteDocumentReader interface {
	PreviewHTTPSkill(context.Context, string, string) (string, error)
}

type documentResponse struct {
	Status       string `json:"status"`
	Markdown     string `json:"markdown,omitempty"`
	SHA256       string `json:"sha256,omitempty"`
	Commit       string `json:"commit,omitempty"`
	Source       string `json:"source"`
	HasResources bool   `json:"hasResources,omitempty"`
}

type cachedDocument struct {
	response documentResponse
	expires  time.Time
}

type documentHandler struct {
	git     SourceResolver
	website WebsiteDocumentReader
	origins map[string]bool
	mu      sync.Mutex
	active  bool
	tokens  float64
	tokenAt time.Time
	cache   map[string]cachedDocument
}

// NewDocumentHandler exposes only bounded, text-only public previews.
// The service-only immutable resolver remains separate and authenticated.
func NewDocumentHandler(git SourceResolver, website WebsiteDocumentReader, origins []string) (http.Handler, error) {
	h := &documentHandler{git: git, website: website, origins: map[string]bool{},
		tokens: 2, tokenAt: time.Now(), cache: map[string]cachedDocument{}}
	for _, origin := range origins {
		if !validOrigin(origin) {
			return nil, fmt.Errorf("invalid document CORS origin")
		}
		h.origins[origin] = true
	}
	return h, nil
}

func (h *documentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Add("Vary", "Origin")
	if r.URL.Path != "/v1/skills/document" {
		writeError(w, 404, "not_found", "Endpoint not found", 0)
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		if !h.origins[origin] {
			writeError(w, 403, "origin_not_allowed", "Origin is not allowed", 0)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
	}
	if r.Method == http.MethodOptions {
		writePreflight(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET, OPTIONS")
		writeError(w, 405, "method_not_allowed", "Only GET is allowed", 0)
		return
	}
	id, source, name, valid := parseDocumentQuery(r.URL.RawQuery)
	if !valid {
		writeError(w, 400, "invalid_request", "Invalid Skill document request", 0)
		return
	}
	key := id + "\x00" + name
	now := time.Now()
	h.mu.Lock()
	if cached, ok := h.cache[key]; ok && now.Before(cached.expires) {
		h.mu.Unlock()
		w.Header().Set("Cache-Control", "public, max-age=300")
		writeJSON(w, 200, cached.response)
		return
	}
	h.tokens = min(2, h.tokens + now.Sub(h.tokenAt).Seconds()/5)
	h.tokenAt = now
	if h.active || h.tokens < 1 {
		h.mu.Unlock()
		w.Header().Set("Retry-After", "5")
		writeError(w, 429, "preview_busy", "Skill document preview is busy", 5)
		return
	}
	h.active, h.tokens = true, h.tokens-1
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		h.active = false
		h.mu.Unlock()
	}()
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	result, err := h.load(ctx, source, name)
	if err != nil || ctx.Err() != nil {
		w.Header().Set("Retry-After", "30")
		writeError(w, 503, "document_unavailable", "Skill source document is temporarily unavailable", 30)
		return
	}
	if result.Status == "resolved" {
		h.mu.Lock()
		if len(h.cache) >= 128 {
			for k := range h.cache {
				delete(h.cache, k)
				break
			}
		}
		h.cache[key] = cachedDocument{response: result, expires: time.Now().Add(10 * time.Minute)}
		h.mu.Unlock()
		w.Header().Set("Cache-Control", "public, max-age=300")
	}
	writeJSON(w, 200, result)
}

func parseDocumentQuery(raw string) (id, source, name string, valid bool) {
	values, err := url.ParseQuery(raw)
	if err != nil || len(values) != 2 || len(values["id"]) != 1 || len(values["name"]) != 1 {
		return "", "", "", false
	}
	id, name = values.Get("id"), values.Get("name")
	if len(id) > 1024 || len(name) == 0 || len(name) > 512 || !safeText(name, 512) ||
		!strings.HasPrefix(id, "skills-sh:") {
		return "", "", "", false
	}
	remainder := strings.TrimPrefix(id, "skills-sh:")
	at := strings.LastIndexByte(remainder, ':')
	if at <= 0 || at == len(remainder)-1 {
		return "", "", "", false
	}
	source, slug := remainder[:at], remainder[at+1:]
	if len(slug) > 256 || !safeText(slug, 256) {
		return "", "", "", false
	}
	if repository(source) {
		return id, source, name, true
	}
	if len(source) <= 253 && !strings.Contains(source, "/") && sourceDomain.MatchString(source) {
		return id, strings.ToLower(source), name, true
	}
	return "", "", "", false
}

func (h *documentHandler) load(ctx context.Context, source, name string) (documentResponse, error) {
	result := documentResponse{Status: "unavailable", Source: source}
	if repository(source) {
		if h.git == nil {
			return result, fmt.Errorf("Git preview unavailable")
		}
		preview, err := h.git.Resolve(ctx, skillsresolve.Request{Repository: source, SkillNameHint: &name})
		if err != nil {
			return result, err
		}
		result.Status, result.Commit = preview.Status, preview.Commit
		if preview.Status == "resolved" && preview.Skill != nil {
			result.Markdown = preview.Skill.RawContent
			result.SHA256 = preview.Skill.RawSHA256
			result.HasResources = preview.Skill.HasResources
		}
		return result, nil
	}
	if h.website == nil {
		return result, fmt.Errorf("website preview unavailable")
	}
	// The external name must satisfy the exact same grammar as the installer.
	if !validDocumentSkillName(name) {
		result.Status = "selection_required"
		return result, nil
	}
	markdown, err := h.website.PreviewHTTPSkill(ctx, "https://"+source, name)
	if err != nil {
		return result, err
	}
	if len(markdown) == 0 || len(markdown) > 64<<10 {
		return result, fmt.Errorf("invalid document length")
	}
	sum := sha256.Sum256([]byte(markdown))
	result.Status, result.Markdown, result.SHA256 = "resolved", markdown, fmt.Sprintf("%x", sum[:])
	return result, nil
}

func validDocumentSkillName(name string) bool { return domain.ValidSkillName(name) }

// SafePublisherClient pins every HTTPS connection to a resolved public IP.
// Rejecting private/reserved destinations before dialing prevents an upstream
// catalog source from becoming a local network read/SSRF primitive.
func SafePublisherClient() *http.Client {
	dialer := &net.Dialer{Timeout: 6 * time.Second}
	transport := &http.Transport{
		Proxy: nil, MaxIdleConns: 4, MaxIdleConnsPerHost: 2,
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		ResponseHeaderTimeout: 8 * time.Second,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil || port != "443" {
				return nil, fmt.Errorf("unsafe publisher destination")
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil || len(ips) == 0 {
				return nil, fmt.Errorf("publisher DNS unavailable")
			}
			for _, ip := range ips {
				if !publicPublisherIP(ip.IP) {
					return nil, fmt.Errorf("nonpublic publisher DNS destination")
				}
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
		},
	}
	return &http.Client{Timeout: 12 * time.Second, Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

var deniedPublisherRanges = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("2001::/32"),
	netip.MustParsePrefix("2001:10::/28"),
	netip.MustParsePrefix("2001:20::/28"),
	netip.MustParsePrefix("2001:db8::/32"),
}

func publicPublisherIP(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	addr = addr.Unmap()
	if !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() ||
		addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() {
		return false
	}
	for _, block := range deniedPublisherRanges {
		if block.Contains(addr) {
			return false
		}
	}
	return true
}
