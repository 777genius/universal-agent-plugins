package homebrewformula

import (
	"fmt"
	"sort"
	"strings"
)

func buildFormula(tag, repo, checksumsPath, downloadBase string) (Formula, error) {
	tag, repo, checksumsPath, downloadBase, err := validateBuildInputs(tag, repo, checksumsPath, downloadBase)
	if err != nil {
		return Formula{}, err
	}
	sumByAsset, err := parseChecksums(checksumsPath)
	if err != nil {
		return Formula{}, err
	}
	version := versionFromTag(tag)
	assets, err := buildFormulaAssets(version, downloadBase, sumByAsset)
	if err != nil {
		return Formula{}, err
	}
	return Formula{
		Tag:       tag,
		Version:   version,
		Repo:      repo,
		ClassName: "PluginKitAi",
		Desc:      "AI CLI plugin runtime with a first-class Go SDK",
		Homepage:  "https://github.com/" + repo,
		Assets:    assets,
	}, nil
}

func validateBuildInputs(tag, repo, checksumsPath, downloadBase string) (string, string, string, string, error) {
	tag = normalizeTag(tag)
	if tag == "" {
		return "", "", "", "", fmt.Errorf("homebrew formula requires a release tag")
	}
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return "", "", "", "", fmt.Errorf("homebrew formula requires repository owner/repo")
	}
	checksumsPath = strings.TrimSpace(checksumsPath)
	if checksumsPath == "" {
		return "", "", "", "", fmt.Errorf("homebrew formula requires checksums.txt path")
	}
	downloadBase = strings.TrimSpace(downloadBase)
	if downloadBase == "" {
		return "", "", "", "", fmt.Errorf("homebrew formula requires download base URL")
	}
	return tag, repo, checksumsPath, downloadBase, nil
}

func buildFormulaAssets(version, downloadBase string, sumByAsset map[string]string) ([]Asset, error) {
	assets := make([]Asset, 0, len(supportedPairs))
	for _, pair := range supportedPairs {
		name := fmt.Sprintf("plugin-kit-ai_%s_%s_%s.tar.gz", version, pair.goos, pair.goarch)
		sum, ok := sumByAsset[name]
		if !ok {
			return nil, fmt.Errorf("checksums.txt missing asset %q", name)
		}
		assets = append(assets, Asset{
			GOOS:   pair.goos,
			GOARCH: pair.goarch,
			Name:   name,
			SHA256: sum,
			URL:    strings.TrimRight(downloadBase, "/") + "/" + name,
		})
	}
	sort.Slice(assets, func(i, j int) bool {
		if assets[i].GOOS == assets[j].GOOS {
			return assets[i].GOARCH < assets[j].GOARCH
		}
		return assets[i].GOOS < assets[j].GOOS
	})
	return assets, nil
}
