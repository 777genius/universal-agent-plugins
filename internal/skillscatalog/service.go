package skillscatalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
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
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}
func (s *Service) usable(p Page, now time.Time) bool {
	age := now.Sub(p.Cache.FetchedAt)
	return age >= 0 && age <= maxStale
}
func (s *Service) fresh(e entry, exists bool, r Request, now time.Time) bool {
	ttl := 30 * time.Minute
	if r.Query != "" {
		ttl = 10 * time.Minute
	}
	return exists && s.usable(e.Result, now) && now.Sub(e.Result.Cache.FetchedAt) < ttl
}
func (s *Service) unavailable(e entry, exists bool, now time.Time, err error) (Page, error) {
	if exists && s.usable(e.Result, now) {
		p := e.Result
		p.Cache.Status = "stale"
		p.Warnings = append(append([]string{}, p.Warnings...), "provider_unavailable")
		return p, nil
	}
	return Page{}, err
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
		if s.fresh(e, exists, r, now) {
			p := e.Result
			p.Cache.Status = "fresh"
			s.mu.Unlock()
			return p, nil
		}
		if waiting, ok := s.pending[key]; ok {
			s.mu.Unlock()
			if err := waitForRefresh(ctx, waiting); err != nil {
				return Page{}, err
			}
			continue
		}
		if retry := s.admitLocked(key, now); retry > 0 {
			s.mu.Unlock()
			return s.unavailable(e, exists, now, &ProviderError{RetryAfter: retry})
		}
		done := make(chan struct{})
		s.pending[key] = done
		s.mu.Unlock()
		result, err := s.refresh(key, r, done)
		if err == nil {
			return result, nil
		}
		return s.unavailable(e, exists, result.Cache.FetchedAt, err)
	}
}
func waitForRefresh(ctx context.Context, waiting <-chan struct{}) error {
	select {
	case <-waiting:
		return nil
	case <-ctx.Done():
		return &ProviderError{}
	}
}

// admitLocked bounds upstream demand without adding a per-client state table.
func (s *Service) admitLocked(key string, now time.Time) time.Duration {
	until := s.failures[key].until
	if s.providerRetry.After(until) {
		until = s.providerRetry
	}
	if now.Before(until) {
		return until.Sub(now)
	}
	s.tokens = min(4, s.tokens+now.Sub(s.tokenAt).Seconds()*2)
	s.tokenAt = now
	if s.tokens < 1 {
		return time.Second
	}
	select {
	case s.slots <- struct{}{}:
	default:
		return time.Second
	}
	s.tokens--
	return 0
}
