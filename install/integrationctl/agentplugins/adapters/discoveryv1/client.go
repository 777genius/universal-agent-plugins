package discoveryv1

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	pathpkg "path"
	"strings"
	"time"
)

type Client struct {
	Origin     string
	HTTPClient *http.Client
	Trust      TrustStore
	Cache      Cache
	Now        func() time.Time
	// AllowHTTPForTests is never populated by production wiring.
	AllowHTTPForTests bool
}

func (client Client) Load(ctx context.Context, installedFloor uint64) (VerifiedBundle, error) {
	if len(client.Trust.Keys) == 0 {
		return VerifiedBundle{}, ErrNoTrustedKeys
	}
	now := time.Now().UTC()
	if client.Now != nil {
		now = client.Now().UTC()
	}
	cached, cacheErr := client.Cache.Load(client.Trust)
	floor := installedFloor
	if cacheErr == nil && cached.Snapshot.Sequence > floor {
		floor = cached.Snapshot.Sequence
	}
	remote, remoteErr := client.fetch(ctx)
	if remoteErr == nil {
		if remote.Snapshot.Sequence < floor {
			remoteErr = fmt.Errorf("%w: got %d, floor %d", ErrRollback, remote.Snapshot.Sequence, floor)
		} else if cacheErr == nil && sameSequenceDifferentDigest(remote, cached) {
			return VerifiedBundle{}, fmt.Errorf("%w: sequence %d", ErrSequenceConflict, remote.Snapshot.Sequence)
		} else if err := ValidityError(remote.Snapshot, now); err == nil {
			authoritative, err := client.Cache.Reconcile(remote, client.Trust)
			if err != nil {
				return VerifiedBundle{}, fmt.Errorf("persist verified Discovery snapshot: %w", err)
			}
			if authoritative.Snapshot.Sequence < floor {
				return VerifiedBundle{}, fmt.Errorf("%w: got %d, floor %d", ErrRollback, authoritative.Snapshot.Sequence, floor)
			}
			if err := ValidityError(authoritative.Snapshot, now); err != nil {
				return VerifiedBundle{}, err
			}
			if authoritative.Snapshot.Sequence == remote.Snapshot.Sequence && authoritative.Digest == remote.Digest {
				authoritative.Source = BundleSourceRemote
			}
			return authoritative, nil
		} else {
			remoteErr = ValidityError(remote.Snapshot, now)
		}
	}
	if cacheErr == nil && cached.Snapshot.Sequence >= floor {
		if err := ValidityError(cached.Snapshot, now); err != nil {
			return VerifiedBundle{}, err
		}
		authoritative, err := client.Cache.Observe(cached, client.Trust)
		if err != nil {
			return VerifiedBundle{}, err
		}
		if authoritative.Snapshot.Sequence < floor {
			return VerifiedBundle{}, fmt.Errorf("%w: got %d, floor %d", ErrRollback, authoritative.Snapshot.Sequence, floor)
		}
		return authoritative, ValidityError(authoritative.Snapshot, now)
	}
	if remoteErr != nil {
		return VerifiedBundle{}, remoteErr
	}
	return VerifiedBundle{}, fmt.Errorf("%w: cache=%v", ErrUnavailable, cacheErr)
}

func (client Client) LoadLocal(installedFloor uint64) (VerifiedBundle, error) {
	bundle, err := client.Cache.Load(client.Trust)
	if err != nil {
		return VerifiedBundle{}, err
	}
	if bundle.Snapshot.Sequence < installedFloor {
		return VerifiedBundle{}, fmt.Errorf("%w: got %d, floor %d", ErrRollback, bundle.Snapshot.Sequence, installedFloor)
	}
	return bundle, nil
}

func (client Client) fetch(ctx context.Context) (VerifiedBundle, error) {
	origin, err := client.originURL()
	if err != nil {
		return VerifiedBundle{}, err
	}
	httpClient := client.safeHTTPClient(origin, 0)
	pointerBytes, err := fetchBounded(ctx, httpClient, origin.ResolveReference(&url.URL{Path: "latest.json"}).String(), MaxLatestBytes, 3)
	if err != nil {
		return VerifiedBundle{}, err
	}
	pointer, err := ParsePointer(pointerBytes)
	if err != nil {
		return VerifiedBundle{}, err
	}
	if len(pointerBytes) > pointer.FetchContract.LatestMaxBytes {
		return VerifiedBundle{}, fmt.Errorf("%w: latest", ErrResponseTooLarge)
	}
	httpClient = client.safeHTTPClient(origin, pointer.FetchContract.MaxRedirects)
	fetch := func(relative string, maximum int) ([]byte, error) {
		return fetchBounded(ctx, httpClient, origin.ResolveReference(&url.URL{Path: relative}).String(), maximum, pointer.FetchContract.RetryAttempts)
	}
	snapshot, err := fetch(pointer.SnapshotPath, pointer.FetchContract.SnapshotMaxBytes)
	if err != nil {
		return VerifiedBundle{}, err
	}
	envelope, err := fetch(pointer.EnvelopePath, pointer.FetchContract.EnvelopeMaxBytes)
	if err != nil {
		return VerifiedBundle{}, err
	}
	search, err := fetch(pointer.SearchPath, pointer.FetchContract.SearchMaxBytes)
	if err != nil {
		return VerifiedBundle{}, err
	}
	bundle, err := VerifyBundle(pointerBytes, snapshot, envelope, search, client.Trust)
	if err == nil {
		bundle.Source = BundleSourceRemote
	}
	return bundle, err
}

var (
	ErrUnsafeOrigin     = errors.New("unsafe Discovery origin")
	ErrUnsafeRedirect   = errors.New("unsafe Discovery redirect")
	ErrResponseTooLarge = errors.New("Discovery response exceeds declared limit")
)

func (client Client) originURL() (*url.URL, error) {
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

func (client Client) safeHTTPClient(origin *url.URL, maxRedirects int) *http.Client {
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
		request.Header.Set("User-Agent", "agentplugins-discovery/1")
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
				last = fmt.Errorf("Discovery origin returned HTTP %d", response.StatusCode)
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

func sameSequenceDifferentDigest(left, right VerifiedBundle) bool {
	return left.Snapshot.Sequence > 0 && left.Snapshot.Sequence == right.Snapshot.Sequence && left.Digest != right.Digest
}
