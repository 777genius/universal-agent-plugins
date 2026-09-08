package directoryv1

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var directRepositoryPattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,38}[A-Za-z0-9])?/[A-Za-z0-9](?:[A-Za-z0-9._-]{0,98}[A-Za-z0-9])?$`)

type EmbeddedBundle struct {
	// Snapshot and Envelope are separate so production can populate them with
	// //go:embed without changing verification or cache code.
	Snapshot []byte
	Envelope []byte
}

func (embedded EmbeddedBundle) Verify(trust TrustStore) (VerifiedBundle, error) {
	if len(embedded.Snapshot) == 0 || len(embedded.Envelope) == 0 {
		return VerifiedBundle{}, errors.New("embedded directory bundle is empty")
	}
	bundle, err := VerifyBundle(embedded.Snapshot, embedded.Envelope, trust)
	if err == nil {
		bundle.Source = BundleSourceEmbedded
	}
	return bundle, err
}

type Client struct {
	// Origin is the configured schema-1 directory (for example,
	// https://host/registry/schemas/1/). Artifact URLs never come from signed or
	// unsigned metadata beyond validated relative names.
	Origin     string
	HTTPClient *http.Client
	Trust      TrustStore
	Embedded   EmbeddedBundle
	Cache      Cache
	Now        func() time.Time
	// AllowHTTPForTests is an explicit in-memory test switch. It is never read
	// from or written to the cache.
	AllowHTTPForTests bool
	// RequireEmbeddedBootstrap is enabled by production wiring. It keeps short
	// names closed until a release has bound the first signed publication into
	// the binary; test/local clients may leave it false.
	RequireEmbeddedBootstrap bool
}

// LoadLocal returns the highest authenticated embedded/cache publication at or
// above installedFloor without performing network I/O. Expiry is deliberately
// not an error here: this path is for read-only provenance and best-effort
// safety warnings, never for authorizing installation or rematerialization.
func (client Client) LoadLocal(installedFloor uint64) (VerifiedBundle, error) {
	if len(client.Trust.Keys) == 0 {
		return VerifiedBundle{}, ErrNoTrustedKeys
	}
	var embedded, cached VerifiedBundle
	embeddedErr := errors.New("embedded directory bundle is empty")
	if len(client.Embedded.Snapshot) > 0 || len(client.Embedded.Envelope) > 0 {
		embedded, embeddedErr = client.Embedded.Verify(client.Trust)
	}
	cached, cacheErr := client.Cache.Load(client.Trust)
	if embeddedErr == nil && cacheErr == nil && sameSequenceDifferentDigest(embedded, cached) {
		return VerifiedBundle{}, fmt.Errorf("%w: sequence %d in cache and embedded bundle", ErrSequenceConflict, cached.Snapshot.Sequence)
	}
	best := VerifiedBundle{}
	for _, candidate := range []VerifiedBundle{embedded, cached} {
		if candidate.Snapshot.Sequence >= installedFloor && candidate.Snapshot.Sequence > best.Snapshot.Sequence {
			best = candidate
		}
	}
	if best.Snapshot.Sequence > 0 {
		return best, nil
	}
	if embedded.Snapshot.Sequence > 0 || cached.Snapshot.Sequence > 0 {
		return VerifiedBundle{}, fmt.Errorf("%w: local floor %d", ErrRollback, installedFloor)
	}
	return VerifiedBundle{}, fmt.Errorf("%w: no authenticated local bundle", ErrUnavailable)
}

var (
	ErrUnsafeOrigin      = errors.New("unsafe directory origin")
	ErrUnsafeRedirect    = errors.New("unsafe directory redirect")
	ErrResponseTooLarge  = errors.New("directory response exceeds declared limit")
	ErrRollback          = errors.New("directory snapshot sequence is below local floor")
	ErrSequenceConflict  = errors.New("directory snapshot sequence has conflicting authenticated bytes")
	ErrExpired           = errors.New("directory snapshot is expired")
	ErrClockSkew         = errors.New("local clock is before directory generation time")
	ErrUnavailable       = errors.New("no valid directory snapshot is available")
	ErrBootstrapNotReady = errors.New("production directory bootstrap is not release-ready")
)

// Load obtains an unexpired schema-1 snapshot. installedFloor must be the
// highest sequence recorded by Directory-managed installations, so cache loss
// cannot lower rollback protection.
func (client Client) Load(ctx context.Context, installedFloor uint64) (VerifiedBundle, error) {
	if len(client.Trust.Keys) == 0 {
		return VerifiedBundle{}, ErrNoTrustedKeys
	}
	if client.RequireEmbeddedBootstrap {
		if _, err := client.Embedded.Verify(client.Trust); err != nil {
			return VerifiedBundle{}, fmt.Errorf("%w: %v", ErrBootstrapNotReady, err)
		}
	}
	now := time.Now().UTC()
	if client.Now != nil {
		now = client.Now().UTC()
	}
	floor := installedFloor
	var embedded VerifiedBundle
	var embeddedErr error
	if len(client.Embedded.Snapshot) > 0 || len(client.Embedded.Envelope) > 0 {
		embedded, embeddedErr = client.Embedded.Verify(client.Trust)
		if embeddedErr == nil && embedded.Snapshot.Sequence > floor {
			floor = embedded.Snapshot.Sequence
		}
	}
	var cached VerifiedBundle
	var cacheErr error
	cached, cacheErr = client.Cache.Load(client.Trust)
	if cacheErr == nil && cached.Snapshot.Sequence > floor {
		floor = cached.Snapshot.Sequence
	}
	// Two locally authenticated publications for one schema sequence are an
	// equivocation, not interchangeable fallback candidates. Detect this before
	// attempting the network so the invariant also holds fully offline.
	if cacheErr == nil && embeddedErr == nil && sameSequenceDifferentDigest(cached, embedded) {
		return VerifiedBundle{}, fmt.Errorf("%w: sequence %d in cache and embedded bundle", ErrSequenceConflict, cached.Snapshot.Sequence)
	}

	remote, remoteErr := client.fetch(ctx)
	if remoteErr == nil {
		if remote.Snapshot.Sequence < floor {
			remoteErr = fmt.Errorf("%w: got %d, floor %d", ErrRollback, remote.Snapshot.Sequence, floor)
		} else if sameSequenceDifferentDigest(remote, cached) || sameSequenceDifferentDigest(remote, embedded) {
			// Never hide authenticated same-sequence equivocation by returning a
			// cached or embedded copy. The caller needs the safety diagnostic.
			return VerifiedBundle{}, fmt.Errorf("%w: sequence %d in remote and local bundle", ErrSequenceConflict, remote.Snapshot.Sequence)
		} else if validityErr(remote.Snapshot, now) == nil {
			// Reconcile every usable remote result, including one equal to our
			// initial cache read. This is the linearization point that prevents a
			// concurrent writer from advancing or equivocating the cache between
			// the initial read and this load's return.
			authoritative, reconcileErr := client.Cache.Reconcile(remote, client.Trust)
			if reconcileErr != nil {
				return VerifiedBundle{}, fmt.Errorf("persist verified directory snapshot: %w", reconcileErr)
			}
			if authoritative.Snapshot.Sequence < floor {
				return VerifiedBundle{}, fmt.Errorf("%w: got %d, floor %d", ErrRollback, authoritative.Snapshot.Sequence, floor)
			}
			if sameSequenceDifferentDigest(authoritative, embedded) {
				return VerifiedBundle{}, fmt.Errorf("%w: sequence %d in cache and embedded bundle", ErrSequenceConflict, authoritative.Snapshot.Sequence)
			}
			if err := validityErr(authoritative.Snapshot, now); err != nil {
				return VerifiedBundle{}, err
			}
			if authoritative.Snapshot.Sequence == remote.Snapshot.Sequence && authoritative.Digest == remote.Digest {
				authoritative.Source = BundleSourceRemote
			}
			return authoritative, nil
		} else {
			remoteErr = validityErr(remote.Snapshot, now)
		}
	}

	best, found := VerifiedBundle{}, false
	for _, candidate := range []VerifiedBundle{cached, embedded} {
		if candidate.Snapshot.Sequence >= floor && candidate.Snapshot.Sequence > 0 && validityErr(candidate.Snapshot, now) == nil && (!found || candidate.Snapshot.Sequence > best.Snapshot.Sequence) {
			best = candidate
			found = true
		}
	}
	if found {
		// Linearize offline/local fallback too. A different process may have
		// advanced or equivocated the cache while this load's network request was
		// failing, and returning the stale initial read would violate the floor.
		authoritative, err := client.Cache.Observe(best, client.Trust)
		if err != nil {
			return VerifiedBundle{}, fmt.Errorf("reconcile local directory snapshot: %w", err)
		}
		if authoritative.Snapshot.Sequence < floor {
			return VerifiedBundle{}, fmt.Errorf("%w: got %d, floor %d", ErrRollback, authoritative.Snapshot.Sequence, floor)
		}
		if sameSequenceDifferentDigest(authoritative, embedded) {
			return VerifiedBundle{}, fmt.Errorf("%w: sequence %d in cache and embedded bundle", ErrSequenceConflict, authoritative.Snapshot.Sequence)
		}
		if err := validityErr(authoritative.Snapshot, now); err != nil {
			return VerifiedBundle{}, err
		}
		return authoritative, nil
	}
	// Preserve the specific safety diagnostic when all authenticated local data
	// fails solely because of time.
	for _, candidate := range []VerifiedBundle{cached, embedded} {
		if candidate.Snapshot.Sequence >= floor && candidate.Snapshot.Sequence > 0 {
			if err := validityErr(candidate.Snapshot, now); err != nil {
				return VerifiedBundle{}, err
			}
		}
	}
	if remoteErr != nil {
		return VerifiedBundle{}, remoteErr
	}
	return VerifiedBundle{}, fmt.Errorf("%w: remote=%v cache=%v embedded=%v", ErrUnavailable, remoteErr, cacheErr, embeddedErr)
}

func validityErr(snapshot domain.DirectorySnapshot, now time.Time) error {
	generated, err := parseTimestamp(snapshot.GeneratedAt)
	if err != nil {
		return err
	}
	expires, err := parseTimestamp(snapshot.ExpiresAt)
	if err != nil {
		return err
	}
	if now.Before(generated) {
		return fmt.Errorf("%w: generated_at=%s now=%s", ErrClockSkew, snapshot.GeneratedAt, now.Format(time.RFC3339))
	}
	if !now.Before(expires) {
		return fmt.Errorf("%w: expires_at=%s now=%s", ErrExpired, snapshot.ExpiresAt, now.Format(time.RFC3339))
	}
	return nil
}

func (client Client) fetch(ctx context.Context) (VerifiedBundle, error) {
	origin, err := client.originURL()
	if err != nil {
		return VerifiedBundle{}, err
	}
	latestURL := origin.ResolveReference(&url.URL{Path: "latest.json"})
	httpClient := client.safeHTTPClient(origin, 2)
	latestBody, err := fetchBounded(ctx, httpClient, latestURL.String(), MaxLatestBytes, 3)
	if err != nil {
		return VerifiedBundle{}, err
	}
	pointer, err := ParsePointer(latestBody)
	if err != nil {
		return VerifiedBundle{}, err
	}
	if len(latestBody) > pointer.FetchContract.LatestMaxBytes {
		return VerifiedBundle{}, ErrResponseTooLarge
	}
	httpClient = client.safeHTTPClient(origin, pointer.FetchContract.MaxRedirects)
	snapshotURL := origin.ResolveReference(&url.URL{Path: pointer.SnapshotPath})
	envelopeURL := origin.ResolveReference(&url.URL{Path: pointer.EnvelopePath})
	snapshot, err := fetchBounded(ctx, httpClient, snapshotURL.String(), pointer.FetchContract.SnapshotMaxBytes, pointer.FetchContract.RetryAttempts)
	if err != nil {
		return VerifiedBundle{}, err
	}
	envelope, err := fetchBounded(ctx, httpClient, envelopeURL.String(), pointer.FetchContract.EnvelopeMaxBytes, pointer.FetchContract.RetryAttempts)
	if err != nil {
		return VerifiedBundle{}, err
	}
	bundle, err := VerifyPointerBundle(pointer, snapshot, envelope, client.Trust)
	if err == nil {
		bundle.Source = BundleSourceRemote
	}
	return bundle, err
}

func (client Client) originURL() (*url.URL, error) {
	parsed, err := url.Parse(client.Origin)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, ErrUnsafeOrigin
	}
	if parsed.Scheme != "https" && !(client.AllowHTTPForTests && parsed.Scheme == "http") {
		return nil, ErrUnsafeOrigin
	}
	if !strings.HasSuffix(parsed.Path, "/") {
		parsed.Path += "/"
	}
	escaped := strings.ToLower(parsed.EscapedPath())
	cleanPath := strings.TrimSuffix(parsed.Path, "/")
	if cleanPath == "" {
		cleanPath = "/"
	}
	if strings.Contains(escaped, "%2f") || strings.Contains(escaped, "%5c") || strings.Contains(escaped, "%2e") || pathpkg.Clean(parsed.Path) != cleanPath {
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
	// The public Directory never needs ambient cookie credentials. In
	// particular, a CookieJar must not repopulate cookies after redirect checks.
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

func sameSequenceDifferentDigest(a, b VerifiedBundle) bool {
	return a.Snapshot.Sequence > 0 && a.Snapshot.Sequence == b.Snapshot.Sequence && a.Digest != b.Digest
}

func fetchBounded(ctx context.Context, client *http.Client, address string, limit, attempts int) ([]byte, error) {
	var last error
	for attempt := 0; attempt < attempts; attempt++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
		if err != nil {
			return nil, err
		}
		request.Header.Set("Accept", "application/json")
		response, err := client.Do(request)
		if err != nil {
			last = err
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, int64(limit)+1))
		closeErr := response.Body.Close()
		if readErr != nil {
			last = readErr
			continue
		}
		if closeErr != nil {
			last = closeErr
			continue
		}
		if len(body) > limit {
			last = ErrResponseTooLarge
			continue
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			last = fmt.Errorf("directory HTTP status %d", response.StatusCode)
			continue
		}
		return body, nil
	}
	return nil, last
}

// DirectSelection is the explicit no-Directory path for exact local and full
// SHA sources. Calling it cannot fetch, inspect cache, or apply Directory policy.
type DirectSelection struct {
	Source domain.SourceIdentity `json:"source"`
	Label  string                `json:"label"`
}

func ResolveDirectExact(source domain.SourceIdentity) (DirectSelection, error) {
	canonical := strings.TrimSpace(source.CanonicalSource)
	requested := strings.TrimSpace(source.RequestedSource)
	if canonical == "" {
		canonical = requested
	}
	local := strings.HasPrefix(canonical, "./") || strings.HasPrefix(canonical, "../") || strings.HasPrefix(canonical, `.\`) || strings.HasPrefix(canonical, `..\`) || filepath.IsAbs(canonical)
	exactGit := false
	if shaPattern.MatchString(source.ResolvedRevision) && directRepositoryPattern.MatchString(source.Repository) {
		expected := domain.DirectorySource{Repository: source.Repository, Revision: source.ResolvedRevision, Path: source.PackageSubpath}
		if canonicalIdentity, ok := parseImmutableGitHubIdentity(canonical); ok && canonicalIdentity == expected {
			exactGit = true
			if requested != "" {
				requestedIdentity, requestedOK := parseImmutableGitHubIdentity(requested)
				exactGit = requestedOK && requestedIdentity == expected
			}
		}
	}
	if !local && !exactGit {
		return DirectSelection{}, errors.New("direct source must be a local path or immutable repository revision")
	}
	return DirectSelection{Source: source, Label: "direct source"}, nil
}

// parseImmutableGitHubIdentity converts the documented shorthand and HTTPS
// display forms into the same structured identity. URL syntax is deliberately
// strict so alternate authorities and query/fragment aliases cannot acquire a
// meaning distinct from the repository, commit, and package path we validate.
func parseImmutableGitHubIdentity(value string) (domain.DirectorySource, bool) {
	value = strings.TrimSpace(value)
	switch {
	case strings.HasPrefix(value, "https://"):
		if strings.ContainsAny(value, "?#") {
			return domain.DirectorySource{}, false
		}
		parsed, err := url.Parse(value)
		if err != nil || parsed.Scheme != "https" || parsed.Host != "github.com" || parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || parsed.RawPath != "" || parsed.Opaque != "" || !strings.HasPrefix(parsed.Path, "/") || strings.HasPrefix(parsed.Path, "//") {
			return domain.DirectorySource{}, false
		}
		value = strings.TrimPrefix(parsed.Path, "/")
	case strings.HasPrefix(value, "github:"):
		value = strings.TrimPrefix(value, "github:")
	}
	if strings.Contains(value, "://") || strings.Count(value, "//") > 1 {
		return domain.DirectorySource{}, false
	}
	repositoryRevision := value
	subpath := ""
	if strings.Contains(value, "//") {
		repositoryRevision, subpath, _ = strings.Cut(value, "//")
		if subpath == "" {
			return domain.DirectorySource{}, false
		}
	}
	if strings.Count(repositoryRevision, "@") != 1 {
		return domain.DirectorySource{}, false
	}
	repository, revision, ok := strings.Cut(repositoryRevision, "@")
	identity := domain.DirectorySource{Repository: repository, Revision: revision, Path: subpath}
	if !ok || !directRepositoryPattern.MatchString(identity.Repository) || !shaPattern.MatchString(identity.Revision) || (identity.Path != "" && validateSourcePath(identity.Path) != nil) {
		return domain.DirectorySource{}, false
	}
	return identity, true
}
