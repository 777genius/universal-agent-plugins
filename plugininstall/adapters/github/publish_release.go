package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/777genius/plugin-kit-ai/plugininstall/domain"
)

func (c *Client) createRelease(ctx context.Context, owner, repo, tag string, draft bool) (*domain.Release, error) {
	payload := map[string]any{
		"tag_name": tag,
		"name":     tag,
		"draft":    draft,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, domain.NewError(domain.ExitNetwork, "github api: marshal release payload: "+err.Error())
	}
	u := fmt.Sprintf("%s/repos/%s/%s/releases", strings.TrimSuffix(c.BaseURL, "/"), owner, repo)
	return c.doReleaseMutation(ctx, http.MethodPost, u, bytes.NewReader(body), http.StatusCreated)
}

func (c *Client) updateReleaseDraftState(ctx context.Context, owner, repo string, releaseID int64, draft bool) (*domain.Release, error) {
	payload := map[string]any{"draft": draft}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, domain.NewError(domain.ExitNetwork, "github api: marshal release patch payload: "+err.Error())
	}
	u := fmt.Sprintf("%s/repos/%s/%s/releases/%d", strings.TrimSuffix(c.BaseURL, "/"), owner, repo, releaseID)
	return c.doReleaseMutation(ctx, http.MethodPatch, u, bytes.NewReader(body), http.StatusOK)
}
