package directoryv1

import (
	"context"
	"net/url"
)

// fetchV2 uses the existing bounded, same-origin transport but verifies
// the independent v2 signed pointer and artifacts. This low-level fetch is
// intentionally unexported: it does not authorize new installations or
// replace the production v1 bootstrap/cache/rollback lifecycle.
func (client Client) fetchV2(ctx context.Context) (VerifiedBundle, error) {
	origin, err := client.originURL()
	if err != nil {
		return VerifiedBundle{}, err
	}
	latestURL := origin.ResolveReference(&url.URL{Path: "latest.json"})
	httpClient := client.safeHTTPClient(origin, 2)
	latest, err := fetchBounded(ctx, httpClient, latestURL.String(), MaxLatestBytes, 3)
	if err != nil {
		return VerifiedBundle{}, err
	}
	pointer, err := ParseV2Pointer(latest)
	if err != nil {
		return VerifiedBundle{}, err
	}
	if len(latest) > pointer.FetchContract.LatestMaxBytes {
		return VerifiedBundle{}, ErrResponseTooLarge
	}
	httpClient = client.safeHTTPClient(origin, pointer.FetchContract.MaxRedirects)
	snapshotURL := origin.ResolveReference(&url.URL{Path: pointer.SnapshotPath})
	envelopeURL := origin.ResolveReference(&url.URL{Path: pointer.EnvelopePath})
	snapshot, err := fetchBounded(ctx, httpClient, snapshotURL.String(),
		pointer.FetchContract.SnapshotMaxBytes, pointer.FetchContract.RetryAttempts)
	if err != nil {
		return VerifiedBundle{}, err
	}
	envelope, err := fetchBounded(ctx, httpClient, envelopeURL.String(),
		pointer.FetchContract.EnvelopeMaxBytes, pointer.FetchContract.RetryAttempts)
	if err != nil {
		return VerifiedBundle{}, err
	}
	bundle, err := VerifyV2PointerBundle(pointer, snapshot, envelope, client.Trust)
	if err != nil {
		return VerifiedBundle{}, err
	}
	bundle.Source = BundleSourceRemote
	return bundle, nil
}
