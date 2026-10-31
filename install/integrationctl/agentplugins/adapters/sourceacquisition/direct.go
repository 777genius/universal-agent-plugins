package sourceacquisition

import (
	"context"
	"errors"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

// ResolvedGit contains final persistable coordinates. The pure source grammar
// never imports acquisition or performs remote-ref discovery.
type ResolvedGit struct {
	Git        sourcespec.Git
	Resolution RefResolution
}

// DirectAcquirer is separate from the signed GitHub source contract. AcquireGit
// requires a full commit; archive pins bind original bytes as well as tree bytes.
// Archive implementers own only archive*.go and return sealed cleanup-owned
// snapshots, with logical Path and ArchiveSHA256 in Source metadata.
type DirectAcquirer interface {
	ResolveGitSource(context.Context, sourcespec.Git) (ResolvedGit, error)
	ResolveGit(context.Context, sourcespec.Git, string, bool) (RefResolution, error)
	DiscoverGitPackages(context.Context, sourcespec.Git, string) ([]string, error)
	AcquireGit(context.Context, sourcespec.Git, string, string) (domain.PackageSnapshot, error)
	AcquireArchive(context.Context, sourcespec.Archive, string, string) (domain.PackageSnapshot, error)
}

// ErrNotArchive permits HTTPS auto-classification to try native Git only when
// no archive signature was recognized. Corrupt/oversize recognized archives
// must never return this sentinel.
var ErrNotArchive = errors.New("HTTPS response is not a package archive")
