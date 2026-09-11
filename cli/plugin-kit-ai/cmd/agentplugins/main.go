package main

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/commands"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoringcli"
	"github.com/777genius/plugin-kit-ai/cli/internal/exitx"
	"github.com/777genius/plugin-kit-ai/cli/internal/terminalprompts"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/locks"
	processadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/clientdetect"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/discoveryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/loader"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/securityscan"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/securityv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statemigration"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	clientplanner "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	version                   = "0.1.0-development"
	defaultDirectoryOrigin    = "https://777genius.github.io/universal-agent-plugins-registry/registry/schemas/1/"
	defaultDirectoryKeyID     = "uap-directory-2026-01"
	defaultDirectoryPublicKey = "HalXARjat+v3ylTPLMAnvuavRo4ZfrF+DbWwsjlp2bI="
	defaultDiscoveryOrigin    = "https://777genius.github.io/universal-agent-plugins-registry/discovery/"
	defaultDiscoveryKeyID     = "uap-discovery-2026-01"
	defaultDiscoveryPublicKey = "IxWvGuscXR9crlCrGyBQZNqroYNVPbBA1B3pnjSffhc="
	defaultSecurityOrigin     = "https://777genius.github.io/universal-agent-plugins-registry/security/"
	defaultSecurityKeyID      = "uap-discovery-2026-01"
	defaultSecurityPublicKey  = "IxWvGuscXR9crlCrGyBQZNqroYNVPbBA1B3pnjSffhc="
	directoryClientFactory    = newDirectoryClient
	discoveryClientFactory    = newDiscoveryClient
	securityClientFactory     = newSecurityClient
)

func main() {
	if commands.IsRelease() {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if err := executeRelease(ctx, os.Args[1:], authoringcli.Streams{In: os.Stdin, Out: os.Stdout, Err: os.Stderr}, run); err != nil {
			os.Exit(exitx.Code(err))
		}
		return
	}
	if handled, code := managedstdio.Dispatch(os.Args[1:], os.Stderr); handled {
		os.Exit(code)
	}
	if commands.IsEnabled() && commands.IsAuthorInvocation(os.Args[1:], agentpluginscli.NewRoot(agentpluginscli.App{Version: version})) {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		app := commands.App{Projects: project.Service{Scratch: os.TempDir()}, Revision: commands.Revision}
		err := app.Execute(ctx, os.Args[1:], authoringcli.Streams{In: os.Stdin, Out: os.Stdout, Err: os.Stderr}, func(factories ...authoringcli.Factory) (*cobra.Command, error) {
			// Construct the ENTIRE root and installer options on every invocation.
			// Installer dependencies are deliberately unconfigured on this author route.
			root := agentpluginscli.NewRoot(agentpluginscli.App{Version: version})
			author, err := authoringcli.NewAuthorCommand(factories...)
			if err != nil {
				return nil, err
			}
			root.AddCommand(author)
			return root, nil
		})
		if err != nil {
			os.Exit(exitx.Code(err))
		}
		return
	}
	if err := run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "agentplugins:", agentpluginscli.ErrorText(os.Args[1:], os.Stderr, err))
		os.Exit(1)
	}
}

func run() error {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return fmt.Errorf("resolve user home: %w", err)
	}
	dataRoot, err := agentpluginsHome()
	if err != nil {
		return err
	}
	directoryClient, err := directoryClientFactory(dataRoot)
	if err != nil {
		return err
	}
	discoveryClient, err := discoveryClientFactory(dataRoot)
	if err != nil {
		return err
	}
	securityClient, err := securityClientFactory(dataRoot)
	if err != nil {
		return err
	}
	registry, err := specregistry.New()
	if err != nil {
		return err
	}
	packageLoader := loader.Loader{Registry: registry}
	runner := processadapter.OS{}
	v2Store := statev2.Store{Path: filepath.Join(dataRoot, "state-v2.json")}
	directoryManager := dirswap.Manager{JournalDir: filepath.Join(dataRoot, "operations-v2")}
	mutationLock := processlock.Lock{Path: filepath.Join(dataRoot, "mutation.lock")}
	stager := providers.Stager{}
	if executable, err := os.Executable(); err == nil {
		stager.LauncherSource, _ = managedstdio.NewSource(executable, version)
	}
	activator := providers.Activator{Runner: runner}
	planner := clientplanner.Planner{ManagedRoot: filepath.Join(dataRoot, "managed"), Detected: map[domain.ClientID]domain.DetectedClient{}}
	lifecycle := usecase.Service{
		StateStore: v2Store, Planner: planner, Targets: planner, Stager: stager, Activator: activator,
		Lock: mutationLock, Kernel: transaction.Kernel{StateStore: v2Store, Directory: directoryManager},
		NativeObserver: providers.NativeIdentityObserver{Stager: stager, Runner: runner}, PluginData: providers.PluginDataManager{Base: filepath.Join(dataRoot, "plugin-data")},
	}
	legacyStatePath := filepath.Join(home, ".plugin-kit-ai", "state.json")
	migrator := statemigration.Migrator{
		LegacyPath:     legacyStatePath,
		V2Store:        v2Store,
		Lock:           mutationLock,
		RecoverJournal: lifecycle.Kernel.Recover,
	}
	app := agentpluginscli.App{
		Version:             version,
		UserHome:            home,
		ManagedRoot:         filepath.Join(dataRoot, "managed"),
		StateStore:          v2Store,
		StateMigrator:       &migrator,
		LegacyLifecycle:     agentpluginscli.NewLegacyLifecycle(legacyStatePath),
		LegacyStateLock:     locks.FileLock{BaseDir: filepath.Join(home, ".plugin-kit-ai", "locks")},
		Detector:            clientdetect.NewOS(home),
		DirectoryClient:     directoryClient,
		DiscoveryClient:     discoveryClient,
		SourceAcquirer:      lazySourceAcquirer{dataRoot: dataRoot, acquirer: sourceacquisition.Acquirer{TempRoot: dataRoot}},
		PackageLoader:       packageLoader,
		NativePackageLoader: loader.OpenAILoader{Loader: packageLoader},
		SecurityIndex:       securityClient,
		SecurityEvaluator: securityscan.Evaluator{
			Scanner: securityscan.ReleaseScanner{Root: filepath.Join(dataRoot, "security", "lintai"), HTTPClient: lintaiReleaseHTTPClient()},
			Cache:   securityscan.FileCache{Root: filepath.Join(dataRoot, "security", "assessments")}, Requirement: securityscan.DefaultRequirement(),
		},
		Lifecycle:     lifecycle,
		Input:         os.Stdin,
		Output:        os.Stdout,
		ErrorOutput:   os.Stderr,
		PromptFactory: terminalprompts.New,
		Terminal:      term.IsTerminal(int(os.Stdin.Fd())),
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return agentpluginscli.NewRoot(app).ExecuteContext(ctx)
}

func newSecurityClient(_ string) (*securityv1.Client, error) {
	publicKey, err := base64.StdEncoding.Strict().DecodeString(defaultSecurityPublicKey)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("decode Security Index public key")
	}
	origin, err := productionFeedOrigin(os.Getenv("AGENTPLUGINS_SECURITY_ORIGIN"), defaultSecurityOrigin, "AGENTPLUGINS_SECURITY_ORIGIN")
	if err != nil {
		return nil, err
	}
	return &securityv1.Client{
		Origin: origin, HTTPClient: hardenedHTTPClient("Security Index"),
		Trust: securityv1.TrustStore{KeyID: defaultSecurityKeyID, PublicKey: ed25519.PublicKey(publicKey)},
	}, nil
}

func newDiscoveryClient(dataRoot string) (*discoveryv1.Client, error) {
	publicKey, err := base64.StdEncoding.Strict().DecodeString(defaultDiscoveryPublicKey)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("decode Discovery public key")
	}
	origin, err := productionFeedOrigin(os.Getenv("AGENTPLUGINS_DISCOVERY_ORIGIN"), defaultDiscoveryOrigin, "AGENTPLUGINS_DISCOVERY_ORIGIN")
	if err != nil {
		return nil, err
	}
	return &discoveryv1.Client{
		Origin: origin, HTTPClient: hardenedHTTPClient("Discovery"),
		Trust: discoveryv1.TrustStore{Keys: []discoveryv1.TrustedKey{{ID: defaultDiscoveryKeyID, PublicKey: ed25519.PublicKey(publicKey), State: discoveryv1.KeyCurrent}}},
		Cache: discoveryv1.Cache{Path: filepath.Join(dataRoot, "discovery-v1-cache.json")},
	}, nil
}

// lazySourceAcquirer creates the private temporary root only once a validated
// command actually begins source acquisition. CLI construction and rejected
// flags/scopes therefore leave AGENTPLUGINS_HOME untouched.
type lazySourceAcquirer struct {
	dataRoot string
	acquirer sourceacquisition.Acquirer
}

func (acquirer lazySourceAcquirer) prepare() error {
	if err := os.MkdirAll(acquirer.dataRoot, 0o700); err != nil {
		return fmt.Errorf("create agentplugins data directory: %w", err)
	}
	return nil
}

func (acquirer lazySourceAcquirer) AcquireLocal(ctx context.Context, source string) (domain.PackageSnapshot, error) {
	if err := acquirer.prepare(); err != nil {
		return domain.PackageSnapshot{}, err
	}
	return acquirer.acquirer.AcquireLocal(ctx, source)
}

func (acquirer lazySourceAcquirer) DiscoverGitHubPackages(ctx context.Context, repository, revision string) ([]string, error) {
	if err := acquirer.prepare(); err != nil {
		return nil, err
	}
	return acquirer.acquirer.DiscoverGitHubPackages(ctx, repository, revision)
}

func (acquirer lazySourceAcquirer) AcquireGitHub(ctx context.Context, repository, revision, subpath string) (domain.PackageSnapshot, error) {
	if err := acquirer.prepare(); err != nil {
		return domain.PackageSnapshot{}, err
	}
	return acquirer.acquirer.AcquireGitHub(ctx, repository, revision, subpath)
}

func (acquirer lazySourceAcquirer) AcquireGitHubVerified(ctx context.Context, repository, revision, subpath, digest string) (domain.PackageSnapshot, error) {
	if err := acquirer.prepare(); err != nil {
		return domain.PackageSnapshot{}, err
	}
	return acquirer.acquirer.AcquireGitHubVerified(ctx, repository, revision, subpath, digest)
}

func newDirectoryClient(dataRoot string) (*directoryv1.Client, error) {
	publicKey, err := base64.StdEncoding.Strict().DecodeString(defaultDirectoryPublicKey)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("decode Directory public key")
	}
	trust := directoryv1.TrustStore{Keys: []directoryv1.TrustedKey{{ID: defaultDirectoryKeyID, PublicKey: ed25519.PublicKey(publicKey), State: directoryv1.KeyCurrent}}}
	embedded, _, err := directoryv1.DecodeReleaseBootstrap(generatedProductionDirectoryBootstrap, trust)
	if err != nil {
		return nil, fmt.Errorf("load generated production Directory bootstrap: %w", err)
	}
	origin, err := productionDirectoryOrigin(os.Getenv("AGENTPLUGINS_DIRECTORY_ORIGIN"))
	if err != nil {
		return nil, err
	}
	return &directoryv1.Client{
		Origin: origin, HTTPClient: hardenedHTTPClient("Directory"),
		Trust: trust,
		// The production bootstrap is intentionally absent until the first
		// post-merge Directory publication. Short-name resolution fails closed
		// until a subsequent release binds that exact signed publication here.
		Embedded: embedded, RequireEmbeddedBootstrap: true,
		Cache: directoryv1.Cache{Path: filepath.Join(dataRoot, "directory-v1-cache.json")},
	}, nil
}

func productionDirectoryOrigin(environmentValue string) (string, error) {
	return productionFeedOrigin(environmentValue, defaultDirectoryOrigin, "AGENTPLUGINS_DIRECTORY_ORIGIN")
}

func productionFeedOrigin(environmentValue, fallback, variable string) (string, error) {
	origin := strings.TrimSpace(environmentValue)
	if origin == "" {
		origin = fallback
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme != "https" || parsed.Opaque != "" || parsed.Host == "" || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return "", fmt.Errorf("%s must be an absolute credential-free HTTPS URL without query or fragment", variable)
	}
	if !strings.HasSuffix(parsed.Path, "/") || parsed.RawPath != "" || strings.Contains(parsed.Path, "\\") || path.Clean(parsed.Path) != strings.TrimSuffix(parsed.Path, "/") {
		return "", fmt.Errorf("%s must have a clean, unescaped directory path ending in /", variable)
	}
	return parsed.String(), nil
}

func agentpluginsHome() (string, error) {
	if explicit := strings.TrimSpace(os.Getenv("AGENTPLUGINS_HOME")); explicit != "" {
		absolute, err := filepath.Abs(explicit)
		if err != nil {
			return "", fmt.Errorf("resolve AGENTPLUGINS_HOME: %w", err)
		}
		return filepath.Clean(absolute), nil
	}
	root, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(root, "agentplugins"), nil
}

func hardenedHTTPClient(feed string) *http.Client {
	return &http.Client{
		Timeout: 20 * time.Second,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) > 2 {
				return fmt.Errorf("too many %s redirects", feed)
			}
			if request.URL.Scheme != "https" || len(via) == 0 ||
				!strings.EqualFold(request.URL.Scheme, via[0].URL.Scheme) ||
				!strings.EqualFold(request.URL.Host, via[0].URL.Host) {
				return fmt.Errorf("%s redirect must remain on the original HTTPS origin", feed)
			}
			request.Header.Del("Authorization")
			request.Header.Del("Cookie")
			request.Header.Del("Proxy-Authorization")
			return nil
		},
	}
}

func lintaiReleaseHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) == 0 || len(via) > 2 || request.URL.Scheme != "https" {
				return fmt.Errorf("invalid LintAI release redirect")
			}
			if request.URL.Hostname() != "github.com" && request.URL.Hostname() != "release-assets.githubusercontent.com" {
				return fmt.Errorf("LintAI release redirect uses an untrusted host")
			}
			request.Header.Del("Authorization")
			request.Header.Del("Cookie")
			request.Header.Del("Proxy-Authorization")
			return nil
		},
	}
}
