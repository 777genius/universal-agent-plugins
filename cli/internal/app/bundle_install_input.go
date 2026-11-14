package app

import (
	"fmt"
	"strings"
)

type bundleInstallInput struct {
	archivePath string
	dest        string
	force       bool
}

func resolveBundleInstallInput(opts PluginBundleInstallOptions) (bundleInstallInput, error) {
	archivePath := strings.TrimSpace(opts.Archive)
	if archivePath == "" {
		return bundleInstallInput{}, fmt.Errorf("bundle install requires a local .tar.gz bundle path")
	}
	lowerArchivePath := strings.ToLower(archivePath)
	if strings.HasPrefix(lowerArchivePath, "http://") || strings.HasPrefix(lowerArchivePath, "https://") {
		return bundleInstallInput{}, fmt.Errorf("bundle install supports local .tar.gz bundles only; remote URLs are out of scope")
	}
	if !strings.HasSuffix(lowerArchivePath, ".tar.gz") {
		return bundleInstallInput{}, fmt.Errorf("bundle install requires a local .tar.gz bundle path")
	}
	dest := strings.TrimSpace(opts.Dest)
	if dest == "" {
		return bundleInstallInput{}, fmt.Errorf("bundle install requires --dest")
	}
	return bundleInstallInput{
		archivePath: archivePath,
		dest:        dest,
		force:       opts.Force,
	}, nil
}
