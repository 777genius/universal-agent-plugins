package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/777genius/plugin-kit-ai/plugininstall/domain"
)

func (c *Client) uploadReleaseAsset(ctx context.Context, uploadURL, name string, body []byte, contentType string) (*domain.Asset, error) {
	if strings.TrimSpace(uploadURL) == "" {
		return nil, domain.NewError(domain.ExitRelease, "release upload_url missing")
	}
	u, err := releaseUploadURL(uploadURL, name)
	if err != nil {
		return nil, domain.NewError(domain.ExitNetwork, "github upload url: "+err.Error())
	}
	req, err := c.newJSONAPIRequest(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return nil, domain.NewError(domain.ExitNetwork, "github upload request: "+err.Error())
	}
	if strings.TrimSpace(contentType) != "" {
		req.Header.Set("Content-Type", contentType)
	} else {
		req.Header.Set("Content-Type", "application/octet-stream")
	}
	resp, err := c.apiClient().Do(req)
	if err != nil {
		return nil, domain.NewError(domain.ExitNetwork, "github upload: "+err.Error())
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, domain.NewError(domain.ExitNetwork, "github upload: read body: "+err.Error())
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, domain.NewError(domain.ExitNetwork, fmt.Sprintf("github upload: status %s: %s", resp.Status, truncate(string(respBody), 200)))
	}
	var dto assetDTO
	if err := json.Unmarshal(respBody, &dto); err != nil {
		return nil, domain.NewError(domain.ExitNetwork, "github upload: invalid json: "+err.Error())
	}
	return &domain.Asset{
		ID:                 dto.ID,
		Name:               dto.Name,
		BrowserDownloadURL: dto.BrowserDownloadURL,
		Size:               dto.Size,
	}, nil
}

func (c *Client) deleteReleaseAsset(ctx context.Context, owner, repo string, assetID int64) error {
	u := fmt.Sprintf("%s/repos/%s/%s/releases/assets/%d", strings.TrimSuffix(c.BaseURL, "/"), owner, repo, assetID)
	req, err := c.newJSONAPIRequest(ctx, http.MethodDelete, u, nil)
	if err != nil {
		return domain.NewError(domain.ExitNetwork, "github delete request: "+err.Error())
	}
	resp, err := c.apiClient().Do(req)
	if err != nil {
		return domain.NewError(domain.ExitNetwork, "github delete: "+err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return domain.NewError(domain.ExitNetwork, fmt.Sprintf("github delete: status %s: %s", resp.Status, truncate(string(body), 200)))
	}
	return nil
}
