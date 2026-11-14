package github

import (
	"net/http"
	"strconv"
	"time"

	"github.com/777genius/plugin-kit-ai/plugininstall/internal/httpconfig"
)

func (c *Client) downloadClient() *http.Client {
	if c.DLClient == nil {
		c.DLClient = httpconfig.DownloadClient()
	}
	return c.DLClient
}

func (c *Client) downloadMaxBytes() int64 {
	max := c.MaxBytes
	if max <= 0 {
		max = httpconfig.DefaultMaxDownloadBytes
	}
	return max
}

func waitRetryAfter(value string) {
	if sec, _ := strconv.Atoi(value); sec > 0 {
		time.Sleep(time.Duration(sec) * time.Second)
	}
}

func backoff(attempt int) time.Duration {
	d := time.Duration(1<<uint(attempt-1)) * time.Second
	if d > 30*time.Second {
		d = 30 * time.Second
	}
	return d + time.Duration(attempt*100)*time.Millisecond
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
