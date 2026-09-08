package agentpluginscli

import (
	"context"
	"io"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/discoveryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statemigration"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type DirectoryClient interface {
	Load(context.Context, uint64) (directoryv1.VerifiedBundle, error)
}

type DiscoveryClient interface {
	Load(context.Context, uint64) (discoveryv1.VerifiedBundle, error)
}

type SecurityIndex interface {
	Lookup(context.Context, domain.SecuritySubject) (*domain.SecurityAssessment, error)
}

type SourceAcquirer interface {
	AcquireLocal(context.Context, string) (domain.PackageSnapshot, error)
	DiscoverGitHubPackages(context.Context, string, string) ([]string, error)
	AcquireGitHub(context.Context, string, string, string) (domain.PackageSnapshot, error)
	AcquireGitHubVerified(context.Context, string, string, string, string) (domain.PackageSnapshot, error)
}

type App struct {
	Version             string
	UserHome            string
	ManagedRoot         string
	StateStore          transaction.StateStore
	Detector            ports.ClientDetector
	DirectoryClient     DirectoryClient
	DiscoveryClient     DiscoveryClient
	SourceAcquirer      SourceAcquirer
	PackageLoader       ports.PackageLoader
	NativePackageLoader ports.PackageLoader
	SecurityIndex       SecurityIndex
	SecurityEvaluator   ports.PackageSecurityEvaluator
	Lifecycle           usecase.Service
	StateMigrator       *statemigration.Migrator
	LegacyLifecycle     ports.LegacyLifecycle
	LegacyStateLock     legacyports.LockManager
	Input               io.Reader
	Output              io.Writer
	ErrorOutput         io.Writer
	Terminal            bool
}

type options struct {
	target              string
	scope               string
	dryRun              bool
	format              string
	noColor             bool
	externalUninstalled bool
	purgeData           bool
	acceptSecurityRisk  bool
	securityDetails     bool
}

func (app App) input() io.Reader {
	if app.Input != nil {
		return app.Input
	}
	return strings.NewReader("")
}

func (app App) output() io.Writer {
	if app.Output != nil {
		return app.Output
	}
	return io.Discard
}

func (app App) errorOutput() io.Writer {
	if app.ErrorOutput != nil {
		return app.ErrorOutput
	}
	return io.Discard
}
