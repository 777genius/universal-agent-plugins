package securityv1

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	pathpkg "path"
	"strings"
	"sync"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type Client struct {
	Origin     string
	HTTPClient *http.Client
	Trust      TrustStore
	Now        func() time.Time
	// AllowHTTPForTests is never populated by production wiring.
	AllowHTTPForTests bool

	mu        sync.Mutex
	attempted bool
	snapshot  Snapshot
	loadErr   error
}

// Lookup returns an authenticated assessment only for the exact package
// subject. A missing, stale, or unavailable assessment is not an install
// failure: the caller must run its pinned local scanner instead.
func (client *Client) Lookup(ctx context.Context, subject domain.SecuritySubject) (*domain.SecurityAssessment, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if !client.attempted {
		client.attempted = true
		client.snapshot, client.loadErr = client.fetch(ctx)
	}
	if client.loadErr != nil {
		return nil, client.loadErr
	}
	return Lookup(client.snapshot, subject), nil
}

func (client *Client) fetch(ctx context.Context) (Snapshot, error) {
	if len(client.Trust.PublicKey) == 0 {
		return Snapshot{}, ErrUnknownKey
	}
	origin, err := client.originURL()
	if err != nil {
		return Snapshot{}, err
	}
	httpClient := client.safeHTTPClient(origin, 0)
	pointerBytes, err := fetchBounded(ctx, httpClient, origin.ResolveReference(&url.URL{Path: "latest.json"}).String(), MaxLatestBytes, 3)
	if err != nil {
		return Snapshot{}, err
	}
	pointer, err := ParsePointer(pointerBytes)
	if err != nil {
		return Snapshot{}, err
	}
	httpClient = client.safeHTTPClient(origin, pointer.FetchContract.MaxRedirects)
	snapshotBytes, err := fetchBounded(ctx, httpClient, origin.ResolveReference(&url.URL{Path: pointer.SnapshotPath}).String(), pointer.FetchContract.SnapshotMaxBytes, pointer.FetchContract.RetryAttempts)
	if err != nil {
		return Snapshot{}, err
	}
	envelopeBytes, err := fetchBounded(ctx, httpClient, origin.ResolveReference(&url.URL{Path: pointer.EnvelopePath}).String(), pointer.FetchContract.EnvelopeMaxBytes, pointer.FetchContract.RetryAttempts)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot, err := Verify(pointerBytes, snapshotBytes, envelopeBytes, client.Trust)
	if err != nil {
		return Snapshot{}, err
	}
	now := time.Now().UTC()
	if client.Now != nil {
		now = client.Now().UTC()
	}
	if err := ValidityError(snapshot, now); err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}

var (
	ErrUnsafeOrigin     = errors.New("unsafe Security Index origin")
	ErrUnsafeRedirect   = errors.New("unsafe Security Index redirect")
	ErrResponseTooLarge = errors.New("Security Index response exceeds declared limit")
)

func (client *Client) originURL() (*url.URL, error) {
	parsed, err := url.Parse(client.Origin)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, ErrUnsafeOrigin
	}
	if parsed.Scheme != "https" && (!client.AllowHTTPForTests || parsed.Scheme != "http") {
		return nil, ErrUnsafeOrigin
	}
	if !strings.HasSuffix(parsed.Path, "/") {
		parsed.Path += "/"
	}
	escaped := strings.ToLower(parsed.EscapedPath())
	clean := strings.TrimSuffix(parsed.Path, "/")
	if clean == "" {
		clean = "/"
	}
	if strings.Contains(escaped, "%2f") || strings.Contains(escaped, "%5c") || strings.Contains(escaped, "%2e") || pathpkg.Clean(parsed.Path) != clean {
		return nil, ErrUnsafeOrigin
	}
	return parsed, nil
}

func (client *Client) safeHTTPClient(origin *url.URL, maxRedirects int) *http.Client {
	result := http.Client{Timeout: 20 * time.Second}
	if client.HTTPClient != nil {
		result = *client.HTTPClient
		if result.Timeout == 0 {
			result.Timeout = 20 * time.Second
		}
	}
	result.Jar = nil
	prior := result.CheckRedirect
	result.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if len(via) > maxRedirects {
			return ErrUnsafeRedirect
		}
		escaped := strings.ToLower(request.URL.EscapedPath())
		unsafePath := strings.Contains(escaped, "%2f") || strings.Contains(escaped, "%5c") || strings.Contains(escaped, "%2e") || pathpkg.Clean(request.URL.Path) != request.URL.Path
		if request.URL.User != nil || request.URL.Scheme != origin.Scheme || !strings.EqualFold(request.URL.Host, origin.Host) || !strings.HasPrefix(request.URL.Path, origin.Path) || unsafePath {
			return ErrUnsafeRedirect
		}
		if prior != nil {
			if err := prior(request, via); err != nil {
				return err
			}
		}
		request.Header.Del("Authorization")
		request.Header.Del("Proxy-Authorization")
		request.Header.Del("Cookie")
		return nil
	}
	return &result
}

func fetchBounded(ctx context.Context, client *http.Client, target string, maximum, attempts int) ([]byte, error) {
	var last error
	for attempt := 0; attempt < attempts; attempt++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			return nil, err
		}
		request.Header.Set("Accept", "application/json")
		request.Header.Set("User-Agent", "agentplugins-security-index/1")
		response, err := client.Do(request)
		if err == nil {
			body, readErr := io.ReadAll(io.LimitReader(response.Body, int64(maximum)+1))
			closeErr := response.Body.Close()
			if readErr == nil && closeErr == nil && response.StatusCode == http.StatusOK && len(body) <= maximum {
				return body, nil
			}
			if len(body) > maximum {
				return nil, ErrResponseTooLarge
			}
			if readErr != nil {
				last = readErr
			} else if closeErr != nil {
				last = closeErr
			} else {
				last = fmt.Errorf("Security Index origin returned HTTP %d", response.StatusCode)
			}
		} else {
			last = err
		}
		if attempt+1 < attempts {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt+1) * 100 * time.Millisecond):
			}
		}
	}
	return nil, last
}
