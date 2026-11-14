package app

import (
	"fmt"
	"strings"
)

type bundlePublishInput struct {
	root     string
	platform string
	ref      string
	tag      string
	owner    string
	repo     string
	draft    bool
	force    bool
}

func resolveBundlePublishInput(opts PluginBundlePublishOptions, deps bundlePublishDeps) (bundlePublishInput, error) {
	root := strings.TrimSpace(opts.Root)
	if root == "" {
		root = "."
	}
	platform := strings.TrimSpace(opts.Platform)
	if platform == "" {
		return bundlePublishInput{}, fmt.Errorf("bundle publish requires --platform")
	}
	ref := strings.TrimSpace(opts.Repo)
	if ref == "" {
		return bundlePublishInput{}, fmt.Errorf("bundle publish requires --repo owner/repo")
	}
	tag := strings.TrimSpace(opts.Tag)
	if tag == "" {
		return bundlePublishInput{}, fmt.Errorf("bundle publish requires --tag")
	}
	if deps.GitHub == nil {
		return bundlePublishInput{}, fmt.Errorf("bundle publish GitHub client is required")
	}
	if deps.Export == nil {
		return bundlePublishInput{}, fmt.Errorf("bundle publish export dependency is required")
	}
	owner, repo, err := splitOwnerRepo(ref)
	if err != nil {
		return bundlePublishInput{}, fmt.Errorf("bundle publish %w", err)
	}
	return bundlePublishInput{
		root:     root,
		platform: platform,
		ref:      ref,
		tag:      tag,
		owner:    owner,
		repo:     repo,
		draft:    opts.Draft,
		force:    opts.Force,
	}, nil
}
