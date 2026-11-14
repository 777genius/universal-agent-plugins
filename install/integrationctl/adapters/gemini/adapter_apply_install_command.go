package gemini

import (
	"context"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (a Adapter) installCommand(ctx context.Context, in ports.ApplyInput) ([]string, string, func(), string, error) {
	if in.ResolvedSource == nil {
		return nil, "", nil, "", domain.NewError(domain.ErrMutationApply, "Gemini install requires resolved source", nil)
	}
	switch kind := strings.TrimSpace(in.ResolvedSource.Kind); kind {
	case "local_path", "github_repo_path":
		path, err := a.syncManagedLocalSource(ctx, in.Manifest, in.ResolvedSource.LocalPath)
		if err != nil {
			return nil, "", nil, "", err
		}
		return []string{"gemini", "extensions", "link", path}, "", nil, path, nil
	case "git_url":
		return a.remoteInstallCommand(in), "", nil, "", nil
	default:
		return nil, "", nil, "", domain.NewError(domain.ErrMutationApply, "Gemini does not support source kind "+kind, nil)
	}
}

func (a Adapter) remoteInstallCommand(in ports.ApplyInput) []string {
	argv := []string{"gemini", "extensions", "install", in.Manifest.RequestedRef.Value}
	if in.Policy.AutoUpdate {
		argv = append(argv, "--auto-update")
	}
	if in.Policy.AllowPrerelease {
		argv = append(argv, "--pre-release")
	}
	return argv
}
