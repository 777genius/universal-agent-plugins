package app

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

func downloadBundleHTTPResponse(client *http.Client, req *http.Request, max int64) ([]byte, string, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("bundle fetch download: %w", err)
	}
	defer resp.Body.Close()
	if err := validateBundleHTTPResponse(resp, max); err != nil {
		return nil, "", err
	}
	body, err := readBundleHTTPBody(resp, max)
	if err != nil {
		return nil, "", err
	}
	return body, resp.Header.Get("Content-Type"), nil
}

func validateBundleHTTPResponse(resp *http.Response, max int64) error {
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("bundle fetch download: status %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	if resp.ContentLength > max {
		return fmt.Errorf("bundle fetch download: content-length %d exceeds limit %d", resp.ContentLength, max)
	}
	return nil
}

func readBundleHTTPBody(resp *http.Response, max int64) ([]byte, error) {
	limit := max
	if resp.ContentLength > 0 {
		limit = resp.ContentLength
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("bundle fetch download read: %w", err)
	}
	if int64(len(body)) > max {
		return nil, fmt.Errorf("bundle fetch download exceeds limit %d bytes", max)
	}
	return body, nil
}
