package skillscatalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const maxEntries = 128
const maxStale = 7 * 24 * time.Hour

type Request struct {
	View  View   `json:"view"`
	Page  int    `json:"page"`
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

func (r Request) key(provider string) string {
	operation := "browse"
	if r.Query != "" {
		operation = "search"
	}
	b, _ := json.Marshal([]any{provider, operation, r.View, r.Page, r.Query, r.Limit})
	digest := sha256.Sum256(b)
	return hex.EncodeToString(digest[:])
}

type entry struct {
	Key     string  `json:"key"`
	Request Request `json:"request"`
	Result  Page    `json:"result"`
}
type failure struct{ until time.Time }
type Service struct {
	provider      Provider
	dir           string
	now           func() time.Time
	mu            sync.Mutex
	entries       map[string]entry
	failures      map[string]failure
	pending       map[string]chan struct{}
	slots         chan struct{}
	tokens        float64
	tokenAt       time.Time
	providerRetry time.Time
}

// NewService loads only bounded, validated cache records belonging to this
// provider. A cache directory is private service state, not a content endpoint.
func NewService(provider Provider, dir string) (*Service, error) {
	if provider == nil || !safeText(provider.ID(), 100) {
		return nil, fmt.Errorf("provider required")
	}
	if dir == "" {
		return nil, fmt.Errorf("cache directory required")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	s := &Service{provider: provider, dir: dir, now: time.Now, entries: map[string]entry{}, failures: map[string]failure{}, pending: map[string]chan struct{}{}, slots: make(chan struct{}, 4), tokens: 4, tokenAt: time.Now()}
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	// This directory contains only our hex-key records; other files are untouched.
	for _, file := range files {
		name := file.Name()
		if len(name) != 69 || !strings.HasSuffix(name, ".json") {
			continue
		}
		if _, err := hex.DecodeString(name[:64]); err != nil {
			continue
		}
		path := filepath.Join(dir, name)
		info, err := file.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		if info.Size() > maxBodyBytes {
			_ = os.Remove(path)
			continue
		}
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var e entry
		if !validStoredJSON(b) || json.Unmarshal(b, &e) != nil || e.Key != name[:64] || e.Key != e.Request.key(provider.ID()) || !validRequest(e.Request) || !validPage(e.Result, e.Request, provider.ID()) || !s.usable(e.Result, s.now()) {
			_ = os.Remove(path)
			continue
		}
		s.entries[e.Key] = e
	}
	s.trimLocked()
	return s, nil
}
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
	if p.Items == nil || p.View != r.View || p.Page != r.Page || p.Query != r.Query || p.Cache.FetchedAt.IsZero() || (p.Cache.Status != "fresh" && p.Cache.Status != "stale") || (p.Total != nil && *p.Total < 0) {
		return false
	}
	maxItems := 200
	if r.Query != "" {
		maxItems = r.Limit
		if p.Total != nil || p.HasMore {
			return false
		}
	}
	if len(p.Items) > maxItems || (r.Query == "" && (p.Total == nil || *p.Total < int64(len(p.Items)))) {
		return false
	}
	seen := map[string]bool{}
	for _, i := range p.Items {
		if i.ProviderID != provider || !safeText(i.ID, 1024) || !strings.HasPrefix(i.ID, provider+":") || seen[i.ID] || !safeText(i.Title, 512) || !safeText(i.SkillNameHint, 512) || i.Metrics.Installs < 0 {
			return false
		}
		seen[i.ID] = true
		if i.Metrics.Window != "all-time" && i.Metrics.Window != "24h" && i.Metrics.Window != "1h" {
			return false
		}
		u, err := url.Parse(i.ProviderURL)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || !safeText(i.ProviderURL, 2048) {
			return false
		}
		switch i.Source.Kind {
		case "github":
			if !repository(i.Source.Repository) || i.Source.URL != "https://github.com/"+i.Source.Repository || i.Availability != "unverified" {
				return false
			}
		case "external":
			if i.Source.Repository != "" || !safeHTTPSURL(i.Source.URL) || i.Availability != "unsupported-source" {
				return false
			}
		default:
			return false
		}
	}
	return true
}
func (s *Service) usable(p Page, now time.Time) bool {
	age := now.Sub(p.Cache.FetchedAt)
	return age >= 0 && age <= maxStale
}
func (s *Service) Get(ctx context.Context, r Request) (Page, error) {
	if !validRequest(r) {
		return Page{}, fmt.Errorf("invalid catalog request")
	}
	key := r.key(s.provider.ID())
	for {
		s.mu.Lock()
		now := s.now()
		e, exists := s.entries[key]
		ttl := 30 * time.Minute
		if r.Query != "" {
			ttl = 10 * time.Minute
		}
		if exists && s.usable(e.Result, now) && now.Sub(e.Result.Cache.FetchedAt) < ttl {
			p := e.Result
			p.Cache.Status = "fresh"
			s.mu.Unlock()
			return p, nil
		}
		if waiting, ok := s.pending[key]; ok {
			s.mu.Unlock()
			select {
			case <-waiting:
				continue
			case <-ctx.Done():
				return Page{}, &ProviderError{}
			}
		}
		unavailable := func(retry time.Duration) (Page, error) {
			s.mu.Unlock()
			if exists && s.usable(e.Result, now) {
				p := e.Result
				p.Cache.Status = "stale"
				p.Warnings = append(append([]string{}, p.Warnings...), "provider_unavailable")
				return p, nil
			}
			return Page{}, &ProviderError{RetryAfter: retry}
		}
		until := s.failures[key].until
		if s.providerRetry.After(until) {
			until = s.providerRetry
		}
		if now.Before(until) {
			return unavailable(until.Sub(now))
		}
		s.tokens = min(4, s.tokens+now.Sub(s.tokenAt).Seconds()*2)
		s.tokenAt = now
		if s.tokens < 1 {
			return unavailable(time.Second)
		}
		select {
		case s.slots <- struct{}{}:
		default:
			return unavailable(time.Second)
		}
		s.tokens--
		done := make(chan struct{})
		s.pending[key] = done
		s.mu.Unlock()
		// A refresh is shared across callers and bounded independently of the
		// initiating browser, whose abort must not poison this key for everyone.
		fetchCtx, cancel := context.WithTimeout(context.Background(), 9*time.Second)
		var result Page
		var err error
		if r.Query == "" {
			result, err = s.provider.Browse(fetchCtx, r.View, r.Page)
		} else {
			result, err = s.provider.Search(fetchCtx, r.Query, r.Limit)
		}
		cancel()
		s.mu.Lock()
		now = s.now()
		result.Cache = CacheInfo{Status: "fresh", FetchedAt: now}
		if err == nil && !validPage(result, r, s.provider.ID()) {
			err = &ProviderError{}
		}
		if err == nil {
			s.entries[key] = entry{Key: key, Request: r, Result: result}
			delete(s.failures, key)
			// Persistence failure never replaces a good on-disk record with bad bytes.
			if !s.persist(s.entries[key]) {
				result.Warnings = append(result.Warnings, "cache_persistence_unavailable")
			}
			s.trimLocked()
		} else {
			retry := 30 * time.Second
			var providerErr *ProviderError
			if errors.As(err, &providerErr) && providerErr.RetryAfter > 0 {
				retry = min(providerErr.RetryAfter, time.Duration(1<<63-1)-time.Second)
				s.providerRetry = now.Add(retry)
			}
			if len(s.failures) >= maxEntries {
				for k, f := range s.failures {
					if !now.Before(f.until) {
						delete(s.failures, k)
					}
				}
				if len(s.failures) >= maxEntries {
					for k := range s.failures {
						delete(s.failures, k)
						break
					}
				}
			}
			s.failures[key] = failure{until: now.Add(retry)}
			err = &ProviderError{RetryAfter: retry}
		}
		delete(s.pending, key)
		close(done)
		<-s.slots
		s.mu.Unlock()
		if err == nil {
			return result, nil
		}
		if exists && s.usable(e.Result, now) {
			p := e.Result
			p.Cache.Status = "stale"
			p.Warnings = append(append([]string{}, p.Warnings...), "provider_unavailable")
			return p, nil
		}
		return Page{}, err
	}
}
func (s *Service) persist(e entry) bool {
	b, err := json.Marshal(e)
	if err != nil || len(b) > maxBodyBytes {
		return false
	}
	f, err := os.CreateTemp(s.dir, ".catalog-*")
	if err != nil {
		return false
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(b); err != nil {
		f.Close()
		return false
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return false
	}
	if f.Close() != nil {
		return false
	}
	return os.Rename(name, filepath.Join(s.dir, e.Key+".json")) == nil
}
func (s *Service) trimLocked() {
	if len(s.entries) <= maxEntries {
		return
	}
	keys := make([]string, 0, len(s.entries))
	for k := range s.entries {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return s.entries[keys[i]].Result.Cache.FetchedAt.Before(s.entries[keys[j]].Result.Cache.FetchedAt)
	})
	for _, k := range keys[:len(keys)-maxEntries] {
		delete(s.entries, k)
		_ = os.Remove(filepath.Join(s.dir, k+".json"))
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
		item, ok := requiredObject(raw, "id", "providerId", "providerUrl", "title", "skillNameHint", "source", "metrics", "availability")
		if !ok {
			return false
		}
		if _, ok = requiredObject(item["source"], "kind", "url"); !ok {
			return false
		}
		if _, ok = requiredObject(item["metrics"], "installs", "window"); !ok {
			return false
		}
	}
	return true
}

func safeHTTPSURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && safeText(raw, 2048)
}
