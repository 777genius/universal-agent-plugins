package github

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/777genius/plugin-kit-ai/plugininstall/domain"
)

func (c *Client) fetchRelease(ctx context.Context, apiPath, notFoundDetail string, allowDraft bool) (*domain.Release, error) {
	u := fmt.Sprintf("%s/%s", strings.TrimSuffix(c.BaseURL, "/"), apiPath)
	req, err := c.newJSONAPIRequest(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, domain.NewError(domain.ExitNetwork, "github api request: "+err.Error())
	}
	resp, err := c.apiClient().Do(req)
	if err != nil {
		return nil, domain.NewError(domain.ExitNetwork, "github api: "+err.Error())
	}
	defer resp.Body.Close()
	body, err := c.readReleaseJSONBody(resp)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, domain.NewError(domain.ExitRelease, notFoundDetail)
	}
	if resp.StatusCode == http.StatusForbidden {
		return nil, domain.NewError(domain.ExitNetwork, "github api forbidden (set GITHUB_TOKEN or --github-token for private repos and higher rate limits)")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, domain.NewError(domain.ExitNetwork, fmt.Sprintf("github api: status %s: %s", resp.Status, truncate(string(body), 200)))
	}
	dto, err := decodeReleaseDTO(body)
	if err != nil {
		return nil, err
	}
	if dto.Draft && !allowDraft {
		return nil, domain.NewError(domain.ExitRelease, "release is draft (refused)")
	}
	return releaseFromDTO(dto), nil
}
