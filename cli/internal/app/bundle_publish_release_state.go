package app

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/plugininstall/domain"
)

func resolveExistingPublishRelease(ctx context.Context, client bundleGitHubPublisher, owner, repo, tag string, wantDraft bool, release *domain.Release) (*domain.Release, string, error) {
	if shouldPromotePublishDraft(release, wantDraft) {
		return promotePublishRelease(ctx, client, owner, repo, tag, release)
	}
	if release.Draft {
		return release, "reused existing draft release", nil
	}
	return release, "reused existing published release", nil
}

func shouldPromotePublishDraft(release *domain.Release, wantDraft bool) bool {
	return !wantDraft && release != nil && release.Draft
}

func promotePublishRelease(ctx context.Context, client bundleGitHubPublisher, owner, repo, tag string, release *domain.Release) (*domain.Release, string, error) {
	if release.ID == 0 {
		return nil, "", fmt.Errorf("bundle publish cannot promote draft release %q because GitHub release id is missing", tag)
	}
	next, err := client.UpdateReleaseDraftState(ctx, owner, repo, release.ID, false)
	if err != nil {
		return nil, "", err
	}
	return next, "promoted draft release to published", nil
}

func shouldCreatePublishRelease(err error) bool {
	de, ok := err.(*domain.Error)
	return ok && de.Code == domain.ExitRelease
}

func createPublishRelease(ctx context.Context, client bundleGitHubPublisher, owner, repo, tag string, wantDraft bool) (*domain.Release, string, error) {
	release, err := client.CreateRelease(ctx, owner, repo, tag, wantDraft)
	if err != nil {
		return nil, "", err
	}
	if wantDraft {
		return release, "created draft release", nil
	}
	return release, "created published release", nil
}
