package httpconfig

import (
	"net"
	"net/http"
	"time"
)

// Defaults for GitHub API and release asset downloads.
const (
	DefaultAPITimeout       = 30 * time.Second
	DefaultDownloadTimeout  = 15 * time.Minute
	DefaultConnectTimeout   = 15 * time.Second
	DefaultMaxRedirects     = 10
	DefaultMaxDownloadBytes = 256 << 20 // 256 MiB
	DefaultGitHubAPIVersion = "2022-11-28"
	MaxRetries429           = 3
)

// Client returns an http.Client for GitHub API JSON calls.
func APIClient() *http.Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.DialContext = (&net.Dialer{Timeout: DefaultConnectTimeout}).DialContext
	return &http.Client{
		Timeout:   DefaultAPITimeout,
		Transport: t,
	}
}

// DownloadClient returns a client for large binary downloads with redirect following.
func DownloadClient() *http.Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.DialContext = (&net.Dialer{Timeout: DefaultConnectTimeout}).DialContext
	return &http.Client{
		Timeout:   DefaultDownloadTimeout,
		Transport: t,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= DefaultMaxRedirects {
				return http.ErrUseLastResponse
			}
			prev := via[len(via)-1].URL
			switch {
			case req.URL.Scheme == "https":
				return nil
			case prev.Scheme == "http" && req.URL.Scheme == "http":
				return nil
			default:
				return http.ErrUseLastResponse
			}
		},
	}
}
