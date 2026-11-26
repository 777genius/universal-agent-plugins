package sourceacquisition

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

// AcquireGitSkill uses the native Git transport with complete raw subtree
// semantics. Even a repository-root Skill cannot silently omit Git submodules.
func (acquirer Acquirer) AcquireGitSkill(ctx context.Context, source sourcespec.Git, commit, expectedTreeDigest string) (domain.PackageSnapshot, error) {
	endpoint, repository, err := acquirer.gitEndpoint(source)
	if err != nil {
		return domain.PackageSnapshot{}, err
	}
	if !sourcespec.ValidCommit(commit) || source.Browser != nil {
		return domain.PackageSnapshot{}, errors.New("raw Git Skill requires resolved immutable coordinates")
	}
	if err = ValidateSkillPath(source.Path); err != nil {
		return domain.PackageSnapshot{}, err
	}
	snapshot, err := acquirer.acquireGitTransport(ctx, acquirer.directEndpoint(endpoint, repository), endpoint, commit, source.Path, true, false)
	if err != nil {
		return domain.PackageSnapshot{}, ctxOrError(ctx, gitFailure(err, "acquire raw Git Skill failed; verify native Git credentials, repository access and complete Skill resources"))
	}
	if expectedTreeDigest != "" {
		return verifyAcquiredTree(snapshot, expectedTreeDigest)
	}
	return snapshot, nil
}

// OpenGitTree reads a complete immutable repository through native credentials.
// Fetching all objects avoids unauthenticated lazy fetches during bounded reads.
// The signed/hosted OpenGitHubTree contract remains anonymous and unchanged.
func (acquirer Acquirer) OpenGitTree(ctx context.Context, source sourcespec.Git, commit string) (*GitTree, error) {
	endpoint, repository, err := acquirer.gitEndpoint(source)
	if err != nil {
		return nil, err
	}
	if !sourcespec.ValidCommit(commit) || source.Browser != nil {
		return nil, errors.New("immutable Git tree requires a resolved commit")
	}
	return withGitConnectRetry(ctx, func(ctx context.Context) (*GitTree, error) {
		return acquirer.openNativeGitTree(ctx, acquirer.directEndpoint(endpoint, repository), commit)
	})
}

func (acquirer Acquirer) openNativeGitTree(ctx context.Context, endpoint, commit string) (*GitTree, error) {
	root, err := os.MkdirTemp(acquirer.TempRoot, "agentplugins-native-tree-*")
	if err != nil {
		return nil, err
	}
	tree := &GitTree{root: root, repository: endpoint, revision: commit, runner: acquirer.runner()}
	fail := func(cause error) (*GitTree, error) {
		_ = tree.Close()
		return nil, directRefError(ctx, cause)
	}
	if _, err = tree.run(ctx, 4096, "init", "--quiet", root); err != nil {
		return fail(err)
	}
	_, err = acquirer.runner().Run(ctx, Command{Dir: root, NativeCredentials: true,
		mappedLocal: acquirer.mappedLocalEndpoint(endpoint), MaxOutputBytes: 4096,
		Args: []string{"fetch", "--quiet", "--depth=1", "--no-tags", "--no-recurse-submodules", "--", endpoint, commit}})
	if err != nil {
		return fail(err)
	}
	out, err := tree.run(ctx, 128, "rev-parse", "--verify", "FETCH_HEAD^{commit}")
	if err != nil || strings.TrimSpace(string(out)) != commit {
		return fail(errors.New("git tree did not match the requested immutable commit"))
	}
	return tree, nil
}
