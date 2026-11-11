package securityscan

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// transientNetworkError is the narrow adapter contract for classifying a
// transport failure. The caller still retries only errors with this contract.
type transientNetworkError interface {
	error
	Timeout() bool
	Temporary() bool
}

func retryableNetworkFailure(err error) bool {
	var networkErr transientNetworkError
	return errors.As(err, &networkErr) && (networkErr.Timeout() || networkErr.Temporary())
}

// downloadReleaseArchive retries only bounded transient acquisition failures.
// All successful bytes still pass the caller's pinned archive SHA-256 check.
func downloadReleaseArchive(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	for attempt := range 3 {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
		if err != nil {
			return nil, err
		}
		response, err := client.Do(request)
		if err != nil {
			if ctx.Err() != nil {
				return nil, fmt.Errorf("download lintai security scanner: %w", ctx.Err())
			}
			if attempt == 2 || !retryableNetworkFailure(err) {
				return nil, fmt.Errorf("download lintai security scanner: %w", err)
			}
		} else {
			if response.StatusCode == http.StatusOK {
				body, readErr := io.ReadAll(io.LimitReader(response.Body, maxReleaseAsset+1))
				_ = response.Body.Close()
				if readErr != nil || len(body) > maxReleaseAsset {
					return nil, errors.New("downloaded lintai release asset is unreadable or oversized")
				}
				return body, nil
			}
			status := response.StatusCode
			_ = response.Body.Close()
			if (status != http.StatusBadGateway && status != http.StatusServiceUnavailable &&
				status != http.StatusGatewayTimeout) || attempt == 2 {
				return nil, fmt.Errorf("download lintai security scanner: HTTP %d", status)
			}
		}
		// Two context-cancellable pauses at most, never another origin.
		wait := time.NewTimer(time.Duration(attempt+1) * 200 * time.Millisecond)
		select {
		case <-ctx.Done():
			wait.Stop()
			return nil, fmt.Errorf("download lintai security scanner: %w", ctx.Err())
		case <-wait.C:
		}
	}
	return nil, errors.New("download lintai security scanner: retries exhausted")
}
