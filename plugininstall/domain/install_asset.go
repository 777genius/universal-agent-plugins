package domain

import (
	"fmt"
	"strings"
)

// PickInstallAsset selects a single release asset for goos/goarch: GoReleaser tarball or raw binary.
// Returns a pointer into the assets slice, whether the payload is a .tar.gz archive, or a domain error.
func PickInstallAsset(assets []Asset, goos, goarch string) (payload *Asset, fromTarGz bool, err error) {
	tarAssets := pickTarGzAssets(assets, goos, goarch)
	rawAssets := pickRawBinaryAssets(assets, goos, goarch)

	switch {
	case len(tarAssets) == 1:
		return &tarAssets[0], true, nil
	case len(tarAssets) > 1:
		var names []string
		for _, a := range tarAssets {
			names = append(names, a.Name)
		}
		return nil, false, NewError(ExitAmbiguous, fmt.Sprintf("multiple matching archives: %v", names))
	case len(rawAssets) == 1:
		return &rawAssets[0], false, nil
	case len(rawAssets) > 1:
		var names []string
		for _, a := range rawAssets {
			names = append(names, a.Name)
		}
		return nil, false, NewError(ExitAmbiguous, fmt.Sprintf("multiple matching raw binaries: %v", names))
	default:
		hint := fmt.Sprintf("*-%s-%s", goos, goarch)
		if goos == "windows" {
			hint += ".exe"
		}
		return nil, false, NewError(ExitRelease,
			fmt.Sprintf("no installable asset for %s/%s (expected *_%s_%s.tar.gz GoReleaser tarball, or raw binary named %s; checksums.txt must list that file)",
				goos, goarch, goos, goarch, hint))
	}
}

func pickTarGzAssets(assets []Asset, goos, goarch string) []Asset {
	suffix := strings.ToLower(fmt.Sprintf("_%s_%s.tar.gz", goos, goarch))
	var out []Asset
	for _, a := range assets {
		n := strings.ToLower(a.Name)
		if !strings.HasSuffix(n, ".tar.gz") {
			continue
		}
		if strings.HasSuffix(n, suffix) {
			out = append(out, a)
		}
	}
	return out
}

// rawUtilityPrefixes skips companion binaries that share the same -GOOS-GOARCH suffix as the main plugin
// (e.g. claude-notifications-go also ships sound-preview-*, list-devices-*, list-sounds-*).
var rawUtilityPrefixes = []string{
	"sound-preview-",
	"list-devices-",
	"list-sounds-",
}

func isRawUtilityBinaryName(lower string) bool {
	for _, p := range rawUtilityPrefixes {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	return false
}

// pickRawBinaryAssets matches release assets like claude-notifications-darwin-arm64 or *-linux-amd64.exe (Windows).
func pickRawBinaryAssets(assets []Asset, goos, goarch string) []Asset {
	goos = strings.ToLower(goos)
	goarch = strings.ToLower(goarch)
	suffixPlain := fmt.Sprintf("-%s-%s", goos, goarch)
	suffixExe := suffixPlain + ".exe"
	var out []Asset
	for _, a := range assets {
		n := strings.ToLower(a.Name)
		if n == "checksums.txt" {
			continue
		}
		if strings.HasSuffix(n, ".tar.gz") || strings.HasSuffix(n, ".zip") {
			continue
		}
		if strings.HasSuffix(n, ".txt") || strings.HasSuffix(n, ".md") || strings.HasSuffix(n, ".sha256") {
			continue
		}
		if goos != "windows" && strings.HasSuffix(n, ".exe") {
			continue
		}
		matched := false
		if goos == "windows" {
			matched = strings.HasSuffix(n, suffixExe)
		} else if strings.HasSuffix(n, suffixExe) {
			matched = false
		} else {
			matched = strings.HasSuffix(n, suffixPlain)
		}
		if !matched {
			continue
		}
		if isRawUtilityBinaryName(n) {
			continue
		}
		out = append(out, a)
	}
	return out
}
