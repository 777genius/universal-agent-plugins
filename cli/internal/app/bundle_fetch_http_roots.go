package app

import (
	"crypto/x509"
	"fmt"
	"os"
)

func loadBundleFetchAdditionalRoots(path string) (*x509.CertPool, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("bundle fetch test root CA file %q: %w", path, err)
	}
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM(body) {
		return nil, fmt.Errorf("bundle fetch test root CA file %q does not contain valid PEM certificates", path)
	}
	return pool, nil
}
