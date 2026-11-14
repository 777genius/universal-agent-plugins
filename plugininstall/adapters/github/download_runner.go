package github

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/777genius/plugin-kit-ai/plugininstall/domain"
	"github.com/777genius/plugin-kit-ai/plugininstall/internal/httpconfig"
)

func (c *Client) downloadAsset(ctx context.Context, url string) ([]byte, string, error) {
	max := c.downloadMaxBytes()
	for attempt := 0; attempt <= httpconfig.MaxRetries429; attempt++ {
		if err := waitForRetryWindow(ctx, attempt); err != nil {
			return nil, "", err
		}
		data, contentType, retry, err := c.downloadAttempt(ctx, url, max, attempt)
		if retry {
			continue
		}
		if err != nil {
			return nil, "", err
		}
		return data, contentType, nil
	}
	return nil, "", domain.NewError(domain.ExitNetwork, "download failed")
}

func (c *Client) downloadAttempt(ctx context.Context, url string, max int64, attempt int) ([]byte, string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", false, domain.NewError(domain.ExitNetwork, "download request: "+err.Error())
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.downloadClient().Do(req)
	if err != nil {
		return nil, "", false, domain.NewError(domain.ExitNetwork, "download: "+err.Error())
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		defer resp.Body.Close()
		waitRetryAfter(resp.Header.Get("Retry-After"))
		if attempt == httpconfig.MaxRetries429 {
			return nil, "", false, domain.NewError(domain.ExitNetwork, "github rate limited (429); set GITHUB_TOKEN or retry later")
		}
		return nil, "", true, nil
	}
	return readDownloadResponse(resp, max)
}

func readDownloadResponse(resp *http.Response, max int64) ([]byte, string, bool, error) {
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, "", false, domain.NewError(domain.ExitNetwork, fmt.Sprintf("download: status %s: %s", resp.Status, truncate(string(b), 200)))
	}
	cl := resp.ContentLength
	if cl > max {
		return nil, "", false, domain.NewError(domain.ExitNetwork, fmt.Sprintf("download: content-length %d exceeds limit %d", cl, max))
	}
	lim := max
	if cl > 0 {
		lim = cl
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, lim+1))
	if err != nil {
		return nil, "", false, domain.NewError(domain.ExitNetwork, "download read: "+err.Error())
	}
	if int64(len(data)) > max {
		return nil, "", false, domain.NewError(domain.ExitNetwork, fmt.Sprintf("download exceeds limit %d bytes", max))
	}
	return data, resp.Header.Get("Content-Type"), false, nil
}

func waitForRetryWindow(ctx context.Context, attempt int) error {
	if attempt == 0 {
		return nil
	}
	select {
	case <-ctx.Done():
		return domain.NewError(domain.ExitNetwork, ctx.Err().Error())
	case <-time.After(backoff(attempt)):
		return nil
	}
}
