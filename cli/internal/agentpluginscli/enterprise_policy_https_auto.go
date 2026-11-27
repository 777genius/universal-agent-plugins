package agentpluginscli

import (
	"context"
	"errors"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

// Ambiguous HTTPS keeps archive-first discovery when policy permits it. A
// denied archive is never probed: native Git must independently admit the same
// endpoint before resolution. Configuration and other gate failures stop both.
func (app App) acquireHTTPSAutoPackage(ctx context.Context, direct sourceacquisition.DirectAcquirer, archive sourcespec.Archive) (directPackage, error) {
	archiveErr := app.checkEnterpriseArchive(ctx, archive)
	if archiveErr != nil && !enterprisePolicyDenialOnly(archiveErr) {
		return directPackage{}, archiveErr
	}
	if archiveErr == nil {
		snapshot, err := direct.AcquireArchive(ctx, archive, archive.SHA256, "")
		if err == nil {
			return app.loadDirectArchive(ctx, snapshot, archive)
		}
		if !errors.Is(err, sourceacquisition.ErrNotArchive) {
			return directPackage{}, err
		}
	}
	endpoint, repository, err := sourcespec.NormalizeGitURL(archive.URL)
	if err != nil {
		return directPackage{}, errors.Join(archiveErr, err)
	}
	// loadDirectGit checks the actual Git alternative before any remote lookup
	// or acquisition. The original unpinned request is retained by the caller.
	return app.loadDirectGit(ctx, sourcespec.Git{URL: endpoint, Repository: repository})
}

// A joined operational failure must not become permission to try another
// transport merely because one of its other causes is a policy denial.
func enterprisePolicyDenialOnly(err error) bool {
	if cause := errors.Unwrap(err); cause != nil {
		return enterprisePolicyDenialOnly(cause)
	}
	var joined interface{ Unwrap() []error }
	if errors.As(err, &joined) {
		causes := joined.Unwrap()
		if len(causes) == 0 {
			return false
		}
		for _, cause := range causes {
			if !enterprisePolicyDenialOnly(cause) {
				return false
			}
		}
		return true
	}
	var denied *domain.InstallPolicyError
	return errors.As(err, &denied) && denied != nil
}
