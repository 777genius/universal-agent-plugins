package skillscatalog

import (
	"context"
	"errors"
	"time"
)

func (s *Service) fetch(r Request) (Page, error) {
	// A refresh is shared across callers and bounded independently of the
	// initiating browser, whose abort must not poison this key for everyone.
	ctx, cancel := context.WithTimeout(context.Background(), 9*time.Second)
	defer cancel()
	if r.Query == "" {
		return s.provider.Browse(ctx, r.View, r.Page)
	}
	return s.provider.Search(ctx, r.Query, r.Limit)
}
func (s *Service) refresh(key string, r Request, done chan struct{}) (Page, error) {
	result, err := s.fetch(r)
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
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
		err = s.recordFailureLocked(key, now, err)
	}
	delete(s.pending, key)
	close(done)
	<-s.slots
	return result, err
}
func (s *Service) recordFailureLocked(key string, now time.Time, err error) error {
	retry := 30 * time.Second
	var providerErr *ProviderError
	if errors.As(err, &providerErr) && providerErr.RetryAfter > 0 {
		retry = min(providerErr.RetryAfter, time.Hour)
		if deadline := now.Add(retry); deadline.After(s.providerRetry) {
			s.providerRetry = deadline
		}
	}
	s.trimFailuresLocked(now)
	s.failures[key] = failure{until: now.Add(retry)}
	return &ProviderError{RetryAfter: retry}
}
func (s *Service) trimFailuresLocked(now time.Time) {
	if len(s.failures) < maxEntries {
		return
	}
	for key, f := range s.failures {
		if !now.Before(f.until) {
			delete(s.failures, key)
		}
	}
	if len(s.failures) < maxEntries {
		return
	}
	for key := range s.failures {
		delete(s.failures, key)
		return
	}
}
