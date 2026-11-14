package app

import (
	"context"

	"github.com/777genius/plugin-kit-ai/plugininstall/domain"
)

func executeBundlePublish(ctx context.Context, input bundlePublishInput, deps bundlePublishDeps) (bundlePublishArtifact, *domain.Release, string, error) {
	artifact, err := prepareBundlePublishArtifact(input.root, input.platform, deps)
	if err != nil {
		return bundlePublishArtifact{}, nil, "", err
	}
	release, state, err := ensurePublishRelease(ctx, deps.GitHub, input.owner, input.repo, input.tag, input.draft)
	if err != nil {
		return bundlePublishArtifact{}, nil, "", err
	}
	if err := replaceBundlePublishAssets(ctx, deps.GitHub, input, release, artifact); err != nil {
		return bundlePublishArtifact{}, nil, "", err
	}
	if err := uploadBundlePublishAssets(ctx, deps.GitHub, release, artifact); err != nil {
		return bundlePublishArtifact{}, nil, "", err
	}
	return artifact, release, state, nil
}

func replaceBundlePublishAssets(ctx context.Context, client bundleGitHubPublisher, input bundlePublishInput, release *domain.Release, artifact bundlePublishArtifact) error {
	if err := maybeDeleteReleaseAsset(ctx, client, input.owner, input.repo, release, artifact.BundleName, input.force); err != nil {
		return err
	}
	return maybeDeleteReleaseAsset(ctx, client, input.owner, input.repo, release, artifact.SidecarName, input.force)
}

func uploadBundlePublishAssets(ctx context.Context, client bundleGitHubPublisher, release *domain.Release, artifact bundlePublishArtifact) error {
	if _, err := client.UploadReleaseAsset(ctx, release.UploadURL, artifact.BundleName, artifact.Body, "application/gzip"); err != nil {
		return err
	}
	if _, err := client.UploadReleaseAsset(ctx, release.UploadURL, artifact.SidecarName, artifact.SidecarBody, "text/plain; charset=utf-8"); err != nil {
		return err
	}
	return nil
}
