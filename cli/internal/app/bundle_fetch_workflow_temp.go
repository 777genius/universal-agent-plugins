package app

import "os"

func writeTempBundleArchive(body []byte) (string, func(), error) {
	f, err := os.CreateTemp("", ".plugin-kit-ai-bundle-fetch-*.tar.gz")
	if err != nil {
		return "", nil, err
	}
	if _, err := f.Write(body); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", nil, err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", nil, err
	}
	cleanup := func() { _ = os.Remove(f.Name()) }
	return f.Name(), cleanup, nil
}
