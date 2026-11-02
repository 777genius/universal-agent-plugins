package skillscatalog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const maxBodyBytes = 2 << 20
const skillsOrigin = "https://www.skills.sh"

var sourceDomain = regexp.MustCompile(`(?i)^[a-z0-9]([a-z0-9.-]*[a-z0-9])?\.[a-z]{2,}$`)
var githubOwner = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]{0,37}[A-Za-z0-9])?$`)
var githubRepository = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,100}$`)

type SkillsSH struct {
	client *http.Client
	origin string
}

// NewSkillsSH pins the anonymous upstream origin. Redirects are disabled so
// neither a redirect nor caller headers can turn this into an arbitrary proxy.
func NewSkillsSH() *SkillsSH { return newSkillsSH(skillsOrigin, http.DefaultTransport) }
func newSkillsSH(origin string, transport http.RoundTripper) *SkillsSH {
	return &SkillsSH{origin: origin, client: &http.Client{Timeout: 8 * time.Second, Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (*SkillsSH) ID() string { return "skills-sh" }

type upstreamSkill struct {
	Source   *string `json:"source"`
	SkillID  *string `json:"skillId"`
	Name     *string `json:"name"`
	Installs *int64  `json:"installs"`
}
type upstreamPage struct {
	Skills  *[]upstreamSkill `json:"skills"`
	Total   *int64           `json:"total"`
	HasMore *bool            `json:"hasMore"`
	Page    *int             `json:"page"`
	Count   *int64           `json:"count"`
}

func (p *SkillsSH) Browse(ctx context.Context, view View, page int) (Page, error) {
	route := map[View]string{Popular: "all-time", Trending: "trending", Hot: "hot"}[view]
	if route == "" || page < 0 {
		return Page{}, &ProviderError{}
	}
	raw, err := p.fetch(ctx, "/api/skills/"+route+"/"+strconv.Itoa(page))
	if err != nil {
		return Page{}, err
	}
	if raw.Total == nil || *raw.Total < 0 || raw.HasMore == nil || raw.Page == nil || *raw.Page != page {
		return Page{}, &ProviderError{}
	}
	result, err := normalize(raw, view)
	if err != nil {
		return Page{}, err
	}
	if int64(len(result.Items)) > *raw.Total || len(result.Items) > 200 {
		return Page{}, &ProviderError{}
	}
	result.Total, result.HasMore, result.Page = raw.Total, *raw.HasMore, page
	return result, nil
}
func (p *SkillsSH) Search(ctx context.Context, q string, limit int) (Page, error) {
	raw, err := p.fetch(ctx, "/api/search?"+url.Values{"q": {q}, "limit": {strconv.Itoa(limit)}}.Encode())
	if err != nil {
		return Page{}, err
	}
	if raw.Count == nil || *raw.Count < 0 || raw.Skills == nil || int64(len(*raw.Skills)) > *raw.Count || len(*raw.Skills) > limit {
		return Page{}, &ProviderError{}
	}
	result, err := normalize(raw, Popular)
	result.Query = q
	result.Warnings = []string{"search_results_limited"}
	return result, err
}
func (p *SkillsSH) fetch(ctx context.Context, path string) (upstreamPage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.origin+path, http.NoBody)
	if err != nil {
		return upstreamPage{}, &ProviderError{}
	}
	req.Header.Set("Accept", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return upstreamPage{}, &ProviderError{}
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		retry := time.Duration(0)
		if resp.StatusCode == 429 {
			retry = parseRetryAfter(resp.Header.Get("Retry-After"), time.Now())
		}
		return upstreamPage{}, &ProviderError{RetryAfter: retry}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
	if err != nil || len(data) > maxBodyBytes || !utf8.Valid(data) {
		return upstreamPage{}, &ProviderError{}
	}
	var raw upstreamPage
	if json.Unmarshal(data, &raw) != nil || raw.Skills == nil {
		return upstreamPage{}, &ProviderError{}
	}
	return raw, nil
}
func parseRetryAfter(s string, now time.Time) time.Duration {
	if seconds, err := strconv.Atoi(s); err == nil && seconds > 0 {
		return time.Duration(min(seconds, int((time.Duration(1<<63-1)-time.Second)/time.Second))) * time.Second
	}
	if date, err := http.ParseTime(s); err == nil && date.After(now) {
		return date.Sub(now)
	}
	return 30 * time.Second
}
func safeText(s string, maxLen int) bool {
	return strings.TrimSpace(s) != "" && len(s) <= maxLen && !strings.ContainsFunc(s, unicode.IsControl)
}
func repository(source string) bool {
	parts := strings.Split(source, "/")
	if len(parts) != 2 {
		return false
	}
	return githubOwner.MatchString(parts[0]) && githubRepository.MatchString(parts[1]) && parts[1] != "." && parts[1] != ".." && !strings.HasSuffix(parts[1], ".git")
}
func normalize(raw upstreamPage, view View) (Page, error) {
	result := Page{Items: make([]SkillListing, 0), View: view}
	if raw.Skills == nil {
		return Page{}, &ProviderError{}
	}
	window := map[View]string{Popular: "all-time", Trending: "24h", Hot: "1h"}[view]
	seen := map[string]bool{}
	for _, s := range *raw.Skills {
		if s.Source == nil || s.SkillID == nil || s.Name == nil || s.Installs == nil || *s.Installs < 0 || !safeText(*s.Source, 256) || !safeText(*s.SkillID, 256) || !safeText(*s.Name, 512) {
			return Page{}, &ProviderError{}
		}
		source, skillID := *s.Source, *s.SkillID
		// The provider slug identifies the listing page; the raw name is the
		// CLI selection hint. Preserve it exactly, leaving admissibility to CLI.
		hint := *s.Name
		providerURL := skillsOrigin + "/"
		if repository(source) {
			providerURL = skillsOrigin + "/" + source + "/" + url.PathEscape(skillID)
		} else if !strings.Contains(source, "/") && sourceDomain.MatchString(source) {
			providerURL = skillsOrigin + "/site/" + strings.ToLower(source) + "/" + url.PathEscape(strings.ToLower(skillID))
		}
		item := SkillListing{ID: fmt.Sprintf("skills-sh:%s:%s", source, skillID), ProviderID: "skills-sh", ProviderURL: providerURL, Title: *s.Name, SkillNameHint: hint, Source: Source{Kind: "external", URL: providerURL}, Metrics: Metrics{Installs: *s.Installs, Window: window}, Availability: "unsupported-source"}
		if repository(source) {
			item.Source = Source{Kind: "github", Repository: source, URL: "https://github.com/" + source}
			item.Availability = "unverified"
		} else if !strings.Contains(source, "/") && sourceDomain.MatchString(source) {
			// The listing page is never an install URL. Publisher discovery uses
			// its own HTTPS origin and the CLI requires an exact --skill name.
			// This is an unverified source hint, not a security endorsement.
			item.Source.URL = "https://" + strings.ToLower(source)
			item.Availability = "unverified"
		}
		if seen[item.ID] {
			return Page{}, &ProviderError{}
		}
		seen[item.ID] = true
		result.Items = append(result.Items, item)
	}
	return result, nil
}
