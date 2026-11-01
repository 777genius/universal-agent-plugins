package skillscatalog

import (
	"encoding/json"
	"net/url"
	"strings"
)

func validRequest(r Request) bool {
	if !r.View.Valid() || r.Page < 0 || r.Page > 10000 {
		return false
	}
	if r.Query == "" {
		return r.Limit == 0
	}
	return r.Page == 0 && r.View == Popular && len([]rune(r.Query)) >= 2 && len(r.Query) <= 200 && safeText(r.Query, 200) && r.Limit > 0 && r.Limit <= 100
}
func validPage(p Page, r Request, provider string) bool {
	if !validPageMetadata(p, r) {
		return false
	}
	seen := map[string]bool{}
	for _, item := range p.Items {
		if !validListing(item, provider) || seen[item.ID] {
			return false
		}
		seen[item.ID] = true
	}
	return true
}
func validPageMetadata(p Page, r Request) bool {
	if p.Items == nil || p.View != r.View || p.Page != r.Page || p.Query != r.Query || p.Cache.FetchedAt.IsZero() {
		return false
	}
	if p.Cache.Status != "fresh" && p.Cache.Status != "stale" {
		return false
	}
	if p.Total != nil && *p.Total < 0 {
		return false
	}
	if r.Query != "" {
		return len(p.Items) <= r.Limit && p.Total == nil && !p.HasMore
	}
	return len(p.Items) <= 200 && p.Total != nil && *p.Total >= int64(len(p.Items))
}
func validListing(i SkillListing, provider string) bool {
	if i.ProviderID != provider || !safeText(i.ID, 1024) || !strings.HasPrefix(i.ID, provider+":") {
		return false
	}
	if !safeText(i.Title, 512) || !safeText(i.SkillNameHint, 512) || i.Metrics.Installs < 0 {
		return false
	}
	if i.Metrics.Window != "all-time" && i.Metrics.Window != "24h" && i.Metrics.Window != "1h" {
		return false
	}
	return safeHTTPSURL(i.ProviderURL) && validSource(i.Source, i.Availability)
}
func validSource(source Source, availability string) bool {
	switch source.Kind {
	case "github":
		return repository(source.Repository) && source.URL == "https://github.com/"+source.Repository && availability == "unverified"
	case "external":
		if source.Repository != "" || !safeHTTPSURL(source.URL) {
			return false
		}
		if availability == "unsupported-source" {
			return strings.HasPrefix(source.URL, skillsOrigin+"/")
		}
		if availability != "unverified" {
			return false
		}
		// Only a canonical publisher origin is installable: never accept
		// a skills.sh listing, an untrusted path, credentials, or a custom port.
		u, err := url.Parse(source.URL)
		return err == nil && u.Host == u.Hostname() &&
			sourceDomain.MatchString(u.Hostname()) &&
			source.URL == "https://"+strings.ToLower(u.Hostname())
	default:
		return false
	}
}

// Required presence is checked before decoding disk records: Go zero values
// alone cannot distinguish a missing false/zero field from a valid value.
func requiredObject(raw json.RawMessage, names ...string) (map[string]json.RawMessage, bool) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return nil, false
	}
	for _, name := range names {
		value, ok := fields[name]
		if !ok || string(value) == "null" {
			return nil, false
		}
	}
	return fields, true
}
func validStoredJSON(b []byte) bool {
	root, ok := requiredObject(b, "key", "request", "result")
	if !ok {
		return false
	}
	if _, ok = requiredObject(root["request"], "view", "page", "query", "limit"); !ok {
		return false
	}
	page, ok := requiredObject(root["result"], "items", "hasMore", "page", "view", "cache")
	if !ok {
		return false
	}
	if _, ok = requiredObject(page["cache"], "status", "fetchedAt"); !ok {
		return false
	}
	var items []json.RawMessage
	if json.Unmarshal(page["items"], &items) != nil {
		return false
	}
	for _, raw := range items {
		if !validStoredListing(raw) {
			return false
		}
	}
	return true
}
func validStoredListing(raw json.RawMessage) bool {
	item, ok := requiredObject(raw, "id", "providerId", "providerUrl", "title", "skillNameHint", "source", "metrics", "availability")
	if !ok {
		return false
	}
	if _, ok = requiredObject(item["source"], "kind", "url"); !ok {
		return false
	}
	_, ok = requiredObject(item["metrics"], "installs", "window")
	return ok
}
func safeHTTPSURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && safeText(raw, 2048)
}
