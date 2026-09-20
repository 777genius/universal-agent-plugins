package app

import (
	"crypto/tls"
	"net"
	"net/http"
	"strings"
)

func newBundleHTTPTransport(cfg bundleHTTPClientConfig) (*http.Transport, error) {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.DialContext = (&net.Dialer{Timeout: defaultBundleFetchConnect}).DialContext
	if strings.TrimSpace(cfg.AdditionalRootsFile) == "" {
		return t, nil
	}
	pool, err := loadBundleFetchAdditionalRoots(cfg.AdditionalRootsFile)
	if err != nil {
		return nil, err
	}
	if t.TLSClientConfig == nil {
		t.TLSClientConfig = &tls.Config{}
	}
	t.TLSClientConfig.RootCAs = pool
	return t, nil
}

func bundleFetchRedirectPolicy(req *http.Request, via []*http.Request) error {
	if len(via) >= defaultBundleFetchMaxRedirects {
		return http.ErrUseLastResponse
	}
	if req.URL.Scheme != "https" {
		return http.ErrUseLastResponse
	}
	prev := via[len(via)-1].URL
	if prev.Scheme != "https" {
		return http.ErrUseLastResponse
	}
	return nil
}
