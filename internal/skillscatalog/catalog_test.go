package skillscatalog

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const sampleSkill = `{"source":"vercel-labs/skills","skillId":"find-skills","name":"Find skills","installs":1234,"newField":"allowed"}`

func browseJSON(skill string) string {
	return `{"skills":[` + skill + `],"total":1,"hasMore":false,"page":0}`
}
func upstream(t *testing.T, handler http.HandlerFunc) *SkillsSH {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return newSkillsSH(srv.URL, srv.Client().Transport)
}
func TestProviderNormalizationAndSafeLocators(t *testing.T) {
	cases := []struct {
		source string
		github bool
	}{
		{"vercel-labs/skills", true}, {"open.feishu.cn", false}, {"uizze.sh", false}, {"owner/repo/tree/main", false}, {"https://github.com/owner/repo", false}, {"owner/..", false}, {"owner/repo.git", false}, {"owner/repo#frag", false}, {"evil:payload", false},
	}
	for _, tc := range cases {
		t.Run(tc.source, func(t *testing.T) {
			skill := strings.Replace(sampleSkill, "vercel-labs/skills", tc.source, 1)
			p := upstream(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/skills/trending/0" {
					t.Errorf("path %s", r.URL.Path)
				}
				if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
					t.Error("credentials forwarded")
				}
				_, _ = w.Write([]byte(browseJSON(skill)))
			})
			page, err := p.Browse(context.Background(), Trending, 0)
			if err != nil {
				t.Fatal(err)
			}
			i := page.Items[0]
			if i.Metrics.Window != "24h" || i.Metrics.Installs != 1234 || i.SkillNameHint != "Find skills" {
				t.Fatalf("wrong normalized listing: %+v", i)
			}
			if tc.github {
				if i.Source.URL != "https://github.com/vercel-labs/skills" || i.Source.Repository != "vercel-labs/skills" || i.Availability != "unverified" {
					t.Fatal(i)
				}
			} else if sourceDomain.MatchString(tc.source) && !strings.Contains(tc.source, "/") {
				if i.Source.Kind != "external" || i.Source.Repository != "" || i.Availability != "unverified" ||
					i.Source.URL != "https://"+tc.source || !strings.HasPrefix(i.ProviderURL, skillsOrigin+"/site/") {
					t.Fatal(i)
				}
			} else {
				if i.Source.Kind != "external" || i.Source.Repository != "" || i.Availability != "unsupported-source" || !strings.HasPrefix(i.Source.URL, skillsOrigin+"/") {
					t.Fatal(i)
				}
			}
		})
	}
}
func TestMalformedAndBoundedUpstreamNeverBecomeEmptySuccess(t *testing.T) {
	cases := []string{`{}`, `{"skills":null,"total":0,"hasMore":false,"page":0}`, `{"skills":[],"total":0,"page":0}`, browseJSON(`{"source":"owner/repo","skillId":"x","name":"x"}`), browseJSON(strings.Replace(sampleSkill, `"installs":1234`, `"installs":-1`, 1)), browseJSON(strings.Replace(sampleSkill, `"name":"Find skills"`, `"name":"bad\nname"`, 1)), browseJSON(strings.Replace(sampleSkill, `"source":"vercel-labs/skills"`, `"source":"evil\nsource"`, 1)), strings.Repeat(" ", maxBodyBytes+1)}
	for n, body := range cases {
		t.Run(string(rune('a'+n)), func(t *testing.T) {
			p := upstream(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) })
			s, err := NewService(p, t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			_, err = s.Get(context.Background(), Request{View: Popular})
			var pe *ProviderError
			if !errors.As(err, &pe) {
				t.Fatalf("malformed success: %v", err)
			}
		})
	}
}
func TestRedirectAnd429Boundaries(t *testing.T) {
	var targetHits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { targetHits.Add(1) }))
	defer target.Close()
	p := upstream(t, func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) })
	if _, err := p.Browse(context.Background(), Popular, 0); err == nil || targetHits.Load() != 0 {
		t.Fatal("followed upstream redirect")
	}
	var hits atomic.Int32
	p = upstream(t, func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(429)
	})
	s, err := NewService(p, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for n := range 2 {
		_, err = s.Get(context.Background(), Request{View: Popular, Page: n})
		var pe *ProviderError
		if !errors.As(err, &pe) || pe.RetryAfter < time.Second*119 {
			t.Fatalf("retry not respected: %v", err)
		}
	}
	if hits.Load() != 1 {
		t.Fatal("429 cooldown did not apply across keys")
	}
}
func TestCacheOutageRestartExpiryAndEmptyResults(t *testing.T) {
	var outage atomic.Bool
	var empty atomic.Bool
	var hits atomic.Int32
	p := upstream(t, func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if outage.Load() {
			w.WriteHeader(503)
			return
		}
		body := browseJSON(sampleSkill)
		if empty.Load() {
			body = `{"skills":[],"total":0,"hasMore":false,"page":0}`
		}
		_, _ = w.Write([]byte(body))
	})
	dir := t.TempDir()
	s, err := NewService(p, dir)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now().UTC()
	now := start
	s.now = func() time.Time { return now }
	s.tokenAt = start
	req := Request{View: Popular}
	fresh, err := s.Get(context.Background(), req)
	if err != nil || fresh.Cache.Status != "fresh" || len(fresh.Items) != 1 {
		t.Fatal(fresh, err)
	}
	outage.Store(true)
	now = start.Add(31 * time.Minute)
	stale, err := s.Get(context.Background(), req)
	if err != nil || stale.Cache.Status != "stale" || !stale.Cache.FetchedAt.Equal(start) || len(stale.Items) != 1 {
		t.Fatal(stale, err)
	}
	prior := hits.Load()
	if _, err = s.Get(context.Background(), req); err != nil || hits.Load() != prior {
		t.Fatal("failure cooldown missing")
	}
	restarted, err := NewService(p, dir)
	if err != nil {
		t.Fatal(err)
	}
	restarted.now = func() time.Time { return now }
	restarted.tokenAt = now
	stale, err = restarted.Get(context.Background(), req)
	if err != nil || stale.Cache.Status != "stale" || len(stale.Items) != 1 {
		t.Fatal(stale, err)
	}
	now = start.Add(7*24*time.Hour + time.Second)
	if _, err = restarted.Get(context.Background(), req); err == nil {
		t.Fatal("served cache past seven days")
	}
	outage.Store(false)
	empty.Store(true)
	now = now.Add(time.Minute)
	result, err := restarted.Get(context.Background(), req)
	if err != nil || len(result.Items) != 0 || result.Items == nil || result.Total == nil || *result.Total != 0 || result.Cache.Status != "fresh" {
		t.Fatal(result, err)
	}
	data, err := os.ReadFile(filepath.Join(dir, req.key(p.ID())+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var stored entry
	if json.Unmarshal(data, &stored) != nil || len(stored.Result.Items) != 0 || !stored.Result.Cache.FetchedAt.Equal(now) {
		t.Fatal("valid empty response did not replace old cache")
	}
}
func TestInvalidRefreshPreservesLastGoodRecord(t *testing.T) {
	var bad atomic.Bool
	p := upstream(t, func(w http.ResponseWriter, r *http.Request) {
		body := browseJSON(sampleSkill)
		if bad.Load() {
			body = `{"skills":[]}`
		}
		_, _ = w.Write([]byte(body))
	})
	dir := t.TempDir()
	s, err := NewService(p, dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	s.now = func() time.Time { return now }
	s.tokenAt = now
	req := Request{View: Popular}
	if _, err = s.Get(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(dir, req.key(p.ID())+".json")
	before, _ := os.ReadFile(filename)
	bad.Store(true)
	now = now.Add(time.Hour)
	page, err := s.Get(context.Background(), req)
	if err != nil || page.Cache.Status != "stale" {
		t.Fatal(page, err)
	}
	after, _ := os.ReadFile(filename)
	if string(before) != string(after) {
		t.Fatal("invalid response replaced good persisted cache")
	}
}
func TestHTTPValidationSearchAndCORS(t *testing.T) {
	var hits atomic.Int32
	p := upstream(t, func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
			t.Error("credentials forwarded")
		}
		if strings.HasPrefix(r.URL.Path, "/api/search") {
			if r.URL.Query().Get("limit") != "100" || r.URL.Query().Get("q") != "find skills" {
				t.Errorf("search query %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"skills":[` + sampleSkill + `],"count":1}`))
			return
		}
		_, _ = w.Write([]byte(browseJSON(sampleSkill)))
	})
	s, err := NewService(p, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	h, err := NewHandler(s, []string{"https://universal-agent-plugins.com", "http://localhost:3000"})
	if err != nil {
		t.Fatal(err)
	}
	invalid := []string{"view=unknown", "page=-1", "page=10001", "page=1&page=2", "q=x", "q=", "q=find&page=1", "q=find&limit=101", "limit=10", "url=https://evil", "q=%ff", "q=%0atest"}
	for _, q := range invalid {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, testRequest(t, "GET", "/v1/skills?"+q))
		if rr.Code != 400 {
			t.Fatalf("query %s: %d %s", q, rr.Code, rr.Body.String())
		}
	}
	if hits.Load() != 0 {
		t.Fatal("invalid query hit provider")
	}
	rr := httptest.NewRecorder()
	r := testRequest(t, "GET", "/v1/skills?q=find+skills&view=hot")
	r.Header.Set("Origin", "https://universal-agent-plugins.com")
	r.Header.Set("Cookie", "private=secret")
	r.Header.Set("Authorization", "Bearer private")
	h.ServeHTTP(rr, r)
	var page Page
	if rr.Code != 200 || json.Unmarshal(rr.Body.Bytes(), &page) != nil || page.Total != nil || page.HasMore || page.Query != "find skills" || page.View != Popular || len(page.Warnings) != 1 {
		t.Fatal(rr.Code, rr.Body.String())
	}
	if rr.Header().Get("Access-Control-Allow-Origin") != "https://universal-agent-plugins.com" || rr.Header().Get("Access-Control-Allow-Credentials") != "" {
		t.Fatal(rr.Header())
	}
	for _, origin := range []string{"https://evil.test", "https://universal-agent-plugins.com.evil.test", "null"} {
		rr = httptest.NewRecorder()
		r = testRequest(t, "GET", "/v1/skills")
		r.Header.Set("Origin", origin)
		h.ServeHTTP(rr, r)
		if rr.Code != 403 || rr.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatal(rr.Code, rr.Header())
		}
	}
	rr = httptest.NewRecorder()
	r = testRequest(t, "OPTIONS", "/v1/skills")
	r.Header.Set("Origin", "http://localhost:3000")
	r.Header.Set("Access-Control-Request-Method", "GET")
	h.ServeHTTP(rr, r)
	if rr.Code != 204 {
		t.Fatal(rr.Code)
	}
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, testRequest(t, "GET", "/healthz"))
	if rr.Code != 200 {
		t.Fatal(rr.Code)
	}
	if _, err = NewHandler(s, []string{"https://example.com/path"}); err == nil {
		t.Fatal("accepted non-origin CORS value")
	}
}
func TestBoundedConcurrentMissesAndCoalescing(t *testing.T) {
	var hits atomic.Int32
	entered := make(chan struct{}, 4)
	release := make(chan struct{})
	p := upstream(t, func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		entered <- struct{}{}
		<-release
		_, _ = w.Write([]byte(browseJSON(sampleSkill)))
	})
	s, err := NewService(p, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for n := range 4 {
		wg.Add(1)
		go func(page int) {
			defer wg.Done()
			_, _ = s.Get(context.Background(), Request{View: Popular, Page: page})
		}(n)
	}
	for range 4 {
		select {
		case <-entered:
		case <-time.After(3 * time.Second):
			t.Fatal("miss did not start")
		}
	}
	if _, err = s.Get(context.Background(), Request{View: Hot}); err == nil {
		t.Fatal("fifth concurrent miss accepted")
	}
	wg.Add(1)
	go func() { defer wg.Done(); _, _ = s.Get(context.Background(), Request{View: Popular}) }()
	close(release)
	wg.Wait()
	if hits.Load() != 4 {
		t.Fatalf("duplicate refresh: %d", hits.Load())
	}
}

type pageProvider struct{}

func (pageProvider) ID() string { return "test-provider" }
func (pageProvider) Browse(_ context.Context, v View, n int) (Page, error) {
	total := int64(0)
	return Page{Items: []SkillListing{}, Total: &total, View: v, Page: n}, nil
}
func (pageProvider) Search(_ context.Context, q string, n int) (Page, error) {
	return Page{Items: []SkillListing{}, View: Popular, Query: q}, nil
}
func TestPersistentCacheBoundAndCorruptRecordRejection(t *testing.T) {
	dir := t.TempDir()
	s, err := NewService(pageProvider{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	s.now = func() time.Time { return now }
	s.tokenAt = now
	for n := range 130 {
		now = now.Add(time.Second)
		if _, err = s.Get(context.Background(), Request{View: Popular, Page: n}); err != nil {
			t.Fatal(err)
		}
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 128 || len(s.entries) != 128 {
		t.Fatalf("unbounded cache: disk %d memory %d err %v", len(files), len(s.entries), err)
	}
	// Persisted records are loaded with real wall time, so set each test timestamp
	// to a past instant before restart while preserving their actual schema.
	for _, file := range files {
		path := filepath.Join(dir, file.Name())
		data, _ := os.ReadFile(path)
		var e entry
		_ = json.Unmarshal(data, &e)
		e.Result.Cache.FetchedAt = time.Now().Add(-time.Hour)
		data, _ = json.Marshal(e)
		if err = os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	corrupt := filepath.Join(dir, files[0].Name())
	data, _ := os.ReadFile(corrupt)
	data = []byte(strings.Replace(string(data), `"hasMore":false,`, "", 1))
	if err = os.WriteFile(corrupt, data, 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := NewService(pageProvider{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.entries) != 127 {
		t.Fatalf("accepted missing required cache field; entries %d", len(loaded.entries))
	}
	if _, err = os.Stat(corrupt); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("corrupt record retained")
	}
}
func TestSearchCacheFreshnessAndCanonicalKeys(t *testing.T) {
	s, err := NewService(pageProvider{}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	s.now = func() time.Time { return now }
	s.tokenAt = now
	req := Request{View: Popular, Query: "find", Limit: 100}
	first, err := s.Get(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(9 * time.Minute)
	cached, err := s.Get(context.Background(), req)
	if err != nil || !cached.Cache.FetchedAt.Equal(first.Cache.FetchedAt) {
		t.Fatal("search missed fresh cache")
	}
	now = now.Add(2 * time.Minute)
	refreshed, err := s.Get(context.Background(), req)
	if err != nil || !refreshed.Cache.FetchedAt.Equal(now) {
		t.Fatal("search TTL exceeded ten minutes")
	}
	other := req
	other.Limit = 50
	if other.key("test-provider") == req.key("test-provider") || req.key("another-provider") == req.key("test-provider") {
		t.Fatal("cache key collision across request/provider")
	}
}

func TestHTTPUnavailableIsStructuredAndDoesNotLeakUpstream(t *testing.T) {
	p := upstream(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Set-Cookie", "upstream-secret=private")
		w.Header().Set("X-Internal", "sensitive")
		w.WriteHeader(500)
		_, _ = w.Write([]byte("secret upstream failure detail"))
	})
	s, err := NewService(p, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	h, err := NewHandler(s, nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, testRequest(t, "GET", "/v1/skills"))
	var envelope errorEnvelope
	if rr.Code != 503 || json.Unmarshal(rr.Body.Bytes(), &envelope) != nil || envelope.Error.Code != "provider_unavailable" || envelope.RetryAfter < 1 || rr.Header().Get("Retry-After") == "" {
		t.Fatal(rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "secret") || rr.Header().Get("Set-Cookie") != "" || rr.Header().Get("X-Internal") != "" {
		t.Fatal("leaked upstream details")
	}
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, testRequest(t, "GET", "/healthz"))
	if rr.Code != 200 {
		t.Fatal("upstream outage broke health")
	}
}

func TestTimeoutDoesNotPersistPartialResponse(t *testing.T) {
	p := upstream(t, func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
	dir := t.TempDir()
	s, err := NewService(p, dir)
	if err != nil {
		t.Fatal(err)
	}
	p.client.Timeout = 20 * time.Millisecond
	if _, err = s.Get(context.Background(), Request{View: Popular}); err == nil {
		t.Fatal("timeout became successful result")
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 0 {
		t.Fatal("timeout persisted a cache entry", err)
	}
}

func TestExactRawNamesAndDomainProviderLinks(t *testing.T) {
	cases := []struct{ source, slug, name, providerURL string }{
		{"pluviobyte/rnskill", "ra", "ra-人话", skillsOrigin + "/pluviobyte/rnskill/ra"},
		{"owner/repo", "ai-video-prompt", "AI_video-prompt", skillsOrigin + "/owner/repo/ai-video-prompt"},
		{"Open.Feishu.Cn", "Agent-Skill", "Agent Skill", skillsOrigin + "/site/open.feishu.cn/agent-skill"},
		{"https://unknown.invalid/path", "slug", "raw name", skillsOrigin + "/"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			installs := int64(1)
			skills := []upstreamSkill{{Source: &tc.source, SkillID: &tc.slug, Name: &tc.name, Installs: &installs}}
			page, err := normalize(upstreamPage{Skills: &skills}, Popular)
			if err != nil {
				t.Fatal(err)
			}
			item := page.Items[0]
			if item.SkillNameHint != tc.name || item.ProviderURL != tc.providerURL {
				t.Fatalf("name/detail locator changed: %+v", item)
			}
			total := int64(1)
			page.Total = &total
			page.Cache = CacheInfo{Status: "fresh", FetchedAt: time.Now()}
			if !validPage(page, Request{View: Popular}, "skills-sh") {
				t.Fatal("valid raw upstream name rejected by persisted schema")
			}
		})
	}
}

func TestSharedRefreshSurvivesFirstCallerCancellation(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var hits atomic.Int32
	p := upstream(t, func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		close(entered)
		select {
		case <-release:
			_, _ = w.Write([]byte(browseJSON(sampleSkill)))
		case <-r.Context().Done():
			return
		}
	})
	s, err := NewService(p, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	firstDone := make(chan error, 1)
	go func() { _, err := s.Get(ctx, Request{View: Popular}); firstDone <- err }()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("refresh did not start")
	}
	cancel()
	// The first browser may abort while a debounced search/browse request shares
	// this refresh. Its cancellation must neither cancel the fetch nor cache a failure.
	close(release)
	page, err := s.Get(context.Background(), Request{View: Popular})
	if err != nil || len(page.Items) != 1 || page.Cache.Status != "fresh" {
		t.Fatalf("caller abort poisoned shared refresh: %+v %v", page, err)
	}
	if err = <-firstDone; err != nil {
		t.Fatalf("shared fetch inherited canceled browser context: %v", err)
	}
	if hits.Load() != 1 {
		t.Fatalf("shared refresh hit upstream %d times", hits.Load())
	}
}

// http.NewRequestWithContext preserves the module's Go 1.22 API floor;
// httptest.NewRequestWithContext was only introduced in Go 1.23.
func testRequest(t *testing.T, method, target string) *http.Request {
	t.Helper()
	request, err := http.NewRequestWithContext(context.Background(), method, target, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	return request
}

func TestProviderCooldownCapsHugeDelayAndNeverShortens(t *testing.T) {
	entered := make(chan struct{}, 2)
	firstRelease, secondRelease := make(chan struct{}), make(chan struct{})
	var firstOnce, secondOnce sync.Once
	releaseFirst := func() { firstOnce.Do(func() { close(firstRelease) }) }
	releaseSecond := func() { secondOnce.Do(func() { close(secondRelease) }) }
	defer releaseFirst()
	defer releaseSecond()
	var hits atomic.Int32
	p := upstream(t, func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		entered <- struct{}{}
		switch r.URL.Path {
		case "/api/skills/all-time/0":
			<-firstRelease
			w.Header().Set("Retry-After", "9223372036")
		case "/api/skills/all-time/1":
			<-secondRelease
			w.Header().Set("Retry-After", "15")
		default:
			t.Error("request escaped the provider cooldown")
		}
		w.WriteHeader(http.StatusTooManyRequests)
	})
	s, err := NewService(p, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	s.now = func() time.Time { return now }
	s.tokenAt = now
	first, second := make(chan error, 1), make(chan error, 1)
	go func() { _, err := s.Get(context.Background(), Request{View: Popular}); first <- err }()
	go func() { _, err := s.Get(context.Background(), Request{View: Popular, Page: 1}); second <- err }()
	for range 2 {
		select {
		case <-entered:
		case <-time.After(3 * time.Second):
			t.Fatal("concurrent requests did not reach provider")
		}
	}
	releaseFirst()
	firstErr := <-first
	releaseSecond()
	secondErr := <-second
	var providerErr *ProviderError
	if !errors.As(firstErr, &providerErr) || providerErr.RetryAfter != time.Hour {
		t.Fatalf("huge cooldown was not capped: %+v", providerErr)
	}
	if !errors.As(secondErr, &providerErr) || providerErr.RetryAfter != 15*time.Second {
		t.Fatalf("short positive retry changed: %+v", providerErr)
	}
	if !s.providerRetry.Equal(now.Add(time.Hour)) {
		t.Fatalf("concurrent smaller delay shortened deadline: %s", s.providerRetry)
	}
	for page, delay := range []time.Duration{time.Hour, 15 * time.Second} {
		key := (Request{View: Popular, Page: page}).key(p.ID())
		if !s.failures[key].until.Equal(now.Add(delay)) {
			t.Fatalf("incorrect failure-key deadline for page %d", page)
		}
	}
	_, err = s.Get(context.Background(), Request{View: Popular, Page: 2})
	if !errors.As(err, &providerErr) || providerErr.RetryAfter != time.Hour || hits.Load() != 2 {
		t.Fatalf("admission ignored global deadline: %v, upstream hits %d", err, hits.Load())
	}
}
