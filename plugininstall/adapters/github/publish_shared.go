package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"strings"

	"github.com/777genius/plugin-kit-ai/plugininstall/domain"
	"github.com/777genius/plugin-kit-ai/plugininstall/internal/httpconfig"
)

func (c *Client) doReleaseMutation(ctx context.Context, method, url string, body io.Reader, okStatus int) (*domain.Release, error) {
	req, err := c.newJSONAPIRequest(ctx, method, url, body)
	if err != nil {
		return nil, domain.NewError(domain.ExitNetwork, "github api request: "+err.Error())
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.apiClient().Do(req)
	if err != nil {
		return nil, domain.NewError(domain.ExitNetwork, "github api: "+err.Error())
	}
	defer resp.Body.Close()
	respBody, err := c.readReleaseJSONBody(resp)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusForbidden {
		return nil, domain.NewError(domain.ExitNetwork, "github api forbidden (set GITHUB_TOKEN or --github-token for release publishing)")
	}
	if resp.StatusCode != okStatus && resp.StatusCode != http.StatusOK {
		return nil, domain.NewError(domain.ExitNetwork, fmt.Sprintf("github api: status %s: %s", resp.Status, truncate(string(respBody), 200)))
	}
	dto, err := decodeReleaseDTO(respBody)
	if err != nil {
		return nil, err
	}
	return releaseFromDTO(dto), nil
}

func (c *Client) newJSONAPIRequest(ctx context.Context, method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if c.APIVersion != "" {
		req.Header.Set("X-GitHub-Api-Version", c.APIVersion)
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	return req, nil
}

func (c *Client) apiClient() *http.Client {
	if c.APIClient == nil {
		c.APIClient = httpconfig.APIClient()
	}
	return c.APIClient
}

func (c *Client) readReleaseJSONBody(resp *http.Response) ([]byte, error) {
	maxJSON := c.releaseJSONMaxBytes()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxJSON+1))
	if err != nil {
		return nil, domain.NewError(domain.ExitNetwork, "github api: read body: "+err.Error())
	}
	if int64(len(respBody)) > maxJSON {
		return nil, domain.NewError(domain.ExitNetwork, fmt.Sprintf("github api: release response exceeds %d bytes", maxJSON))
	}
	return respBody, nil
}

func decodeReleaseDTO(body []byte) (releaseDTO, error) {
	var dto releaseDTO
	if err := json.Unmarshal(body, &dto); err != nil {
		return releaseDTO{}, domain.NewError(domain.ExitNetwork, "github api: invalid json: "+err.Error())
	}
	return dto, nil
}

func releaseUploadURL(raw, name string) (string, error) {
	base := raw
	if idx := strings.Index(base, "{"); idx >= 0 {
		base = base[:idx]
	}
	u, err := neturl.Parse(base)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("name", name)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func releaseFromDTO(dto releaseDTO) *domain.Release {
	out := &domain.Release{
		ID:         dto.ID,
		TagName:    dto.TagName,
		Draft:      dto.Draft,
		Prerelease: dto.Prerelease,
		UploadURL:  dto.UploadURL,
		Assets:     make([]domain.Asset, 0, len(dto.Assets)),
	}
	for _, a := range dto.Assets {
		out.Assets = append(out.Assets, domain.Asset{
			ID:                 a.ID,
			Name:               a.Name,
			BrowserDownloadURL: a.BrowserDownloadURL,
			Size:               a.Size,
		})
	}
	return out
}
