// Package skillscatalog owns the public, provider-independent listing contract.
package skillscatalog

import (
	"context"
	"time"
)

type View string

const (
	Popular  View = "popular"
	Trending View = "trending"
	Hot      View = "hot"
)

func (v View) Valid() bool { return v == Popular || v == Trending || v == Hot }

type Source struct {
	Kind       string `json:"kind"`
	Repository string `json:"repository,omitempty"`
	URL        string `json:"url"`
}
type Metrics struct {
	Installs int64  `json:"installs"`
	Window   string `json:"window"`
}
type SkillListing struct {
	ID            string  `json:"id"`
	ProviderID    string  `json:"providerId"`
	ProviderURL   string  `json:"providerUrl"`
	Title         string  `json:"title"`
	SkillNameHint string  `json:"skillNameHint"`
	Source        Source  `json:"source"`
	Metrics       Metrics `json:"metrics"`
	Availability  string  `json:"availability"`
}
type CacheInfo struct {
	Status    string    `json:"status"`
	FetchedAt time.Time `json:"fetchedAt"`
}
type Page struct {
	Items    []SkillListing `json:"items"`
	Total    *int64         `json:"total,omitempty"`
	HasMore  bool           `json:"hasMore"`
	Page     int            `json:"page"`
	Query    string         `json:"query,omitempty"`
	View     View           `json:"view"`
	Cache    CacheInfo      `json:"cache"`
	Warnings []string       `json:"warnings,omitempty"`
}

// Provider returns validated listings. Search returns a bounded result set, not
// a complete census; it must omit Total unless the provider proves completeness.
type Provider interface {
	ID() string
	Browse(context.Context, View, int) (Page, error)
	Search(context.Context, string, int) (Page, error)
}

type ProviderError struct{ RetryAfter time.Duration }

func (*ProviderError) Error() string { return "skills provider unavailable" }
