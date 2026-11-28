package skillsource

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

// RequestedGitIntent parses known source authority and the original selector
// without resolving a ref, opening a repository or touching the network.
// It deliberately shares the picker's existing source grammar.
func RequestedGitIntent(input, explicitRef string) (sourcespec.Git, error) {
	request, err := parseSourceInput(input)
	if err != nil {
		return sourcespec.Git{}, err
	}
	var git sourcespec.Git
	if request.git != nil {
		git = *request.git
	} else {
		git = sourcespec.Git{Repository: request.source.Repository, Path: request.source.RequestedPath}
		if request.urlTail != "" {
			git.Browser = &sourcespec.BrowserTree{Suffix: request.urlTail}
		} else if sourcespec.ValidCommit(request.source.RequestedSelector) {
			git.Commit = request.source.RequestedSelector
		} else {
			git.Ref = request.source.RequestedSelector
		}
	}
	if explicitRef != "" {
		if (git.Commit != "" && git.Commit != explicitRef) || (git.Ref != "" && git.Ref != explicitRef) {
			return sourcespec.Git{}, fmt.Errorf("source selector conflicts with --ref")
		}
		git.Commit, git.Ref = "", ""
		if sourcespec.ValidCommit(explicitRef) {
			git.Commit = explicitRef
		} else {
			git.Ref = explicitRef
		}
	}
	return git, nil
}
