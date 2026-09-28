package clientdetect

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type Detector struct {
	HomeDir                     string
	WorkingDir                  string
	GOOS                        string
	Environment                 map[string]string
	SystemApplicationsDir       string
	WindowsProgramFiles         []string
	LinuxApplicationDirs        []string
	LookPath                    func(string) (string, error)
	Lstat                       func(string) (fs.FileInfo, error)
	ReadDir                     func(string) ([]os.DirEntry, error)
	EvalSymlinks                func(string) (string, error)
	ProbeVersionWithEnvironment func(context.Context, string, []string) (string, error)
	// An explicitly injected legacy probe overrides the OS probe. It cannot
	// carry a profile environment, so clients requiring one remain unprobed.
	// Use ProbeVersionWithEnvironment and leave ProbeVersion nil for those clients.
	ProbeVersion func(context.Context, string) (string, error)
	VersionTimeout              time.Duration
	TargetedVersionTimeout      time.Duration
	// Registry supplies the client adapters that know where each client keeps
	// its surfaces. It is injected by the composition root and never defaulted
	// to "every client": that would link every adapter into any binary that
	// merely detects clients.
	Registry *clients.Registry
}

// NewOS builds the detector for the machine it runs on. The registry stays
// unset on purpose - only the composition root decides which clients a binary
// knows about, so it assigns Registry before the detector is used.
func NewOS(homeDir string) Detector {
	environment := environmentSnapshot()
	workingDir, _ := os.Getwd()
	xdgApplications := ""
	if root := strings.TrimSpace(environment["XDG_DATA_HOME"]); root != "" {
		xdgApplications = filepath.Join(root, "applications")
	}
	userApplications := ""
	if root := strings.TrimSpace(homeDir); root != "" {
		userApplications = filepath.Join(root, ".local", "share", "applications")
	}
	return Detector{
		HomeDir:               homeDir,
		WorkingDir:            workingDir,
		GOOS:                  runtime.GOOS,
		Environment:           environment,
		SystemApplicationsDir: "/Applications",
		WindowsProgramFiles:   compactPaths(environment["ProgramFiles"], environment["ProgramW6432"], environment["ProgramFiles(x86)"]),
		LinuxApplicationDirs: compactPaths(
			xdgApplications,
			userApplications,
			"/usr/local/share/applications", "/usr/share/applications",
		),
		LookPath:                   exec.LookPath,
		Lstat:                      os.Lstat,
		ReadDir:                    os.ReadDir,
		EvalSymlinks:               filepath.EvalSymlinks,
		ProbeVersionWithEnvironment: probeExecutableVersionWithEnvironment,
		VersionTimeout:             2 * time.Second,
		TargetedVersionTimeout:     10 * time.Second,
	}
}

func (detector Detector) Detect(ctx context.Context) ([]domain.DetectedClient, error) {
	return detector.detect(ctx, false, nil)
}

// DetectWithVersionProbe is reserved for explicit lifecycle resolution that
// needs the installed client version to bind signed Directory evidence. Detect
// itself is strictly observational and never executes a discovered binary.
func (detector Detector) DetectWithVersionProbe(ctx context.Context) ([]domain.DetectedClient, error) {
	return detector.detect(ctx, true, nil)
}

func (detector Detector) DetectTargetsWithVersionProbe(ctx context.Context, targets []domain.ClientID) ([]domain.DetectedClient, error) {
	selected := make(map[domain.ClientID]struct{}, len(targets))
	for _, target := range targets {
		selected[target] = struct{}{}
	}
	timeout := detector.TargetedVersionTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	detector.VersionTimeout = timeout
	return detector.detect(ctx, true, selected)
}

func (detector Detector) detect(ctx context.Context, probeVersion bool, selected map[domain.ClientID]struct{}) ([]domain.DetectedClient, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	probe := func(clientID domain.ClientID) bool {
		if !probeVersion {
			return false
		}
		if selected == nil {
			return true
		}
		_, ok := selected[clientID]
		return ok
	}
	if strings.TrimSpace(detector.HomeDir) == "" {
		return nil, fmt.Errorf("client detector home directory is required")
	}
	if detector.LookPath == nil || detector.Lstat == nil {
		return nil, fmt.Errorf("client detector probes are required")
	}
	if detector.Registry == nil {
		return nil, clients.ErrRegistryRequired
	}
	adapters := detector.Registry.All()
	host := detector.host()
	result := make([]domain.DetectedClient, 0, len(adapters))
	for _, adapter := range adapters {
		client, err := detector.detectClient(ctx, host, adapter, probe(adapter.ID()))
		if err != nil {
			return nil, fmt.Errorf("detect %s: %w", adapter.ID(), err)
		}
		result = append(result, client)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ClientID < result[j].ClientID })
	return result, nil
}

// detectClient turns one adapter's raw observation into the domain view.
// Identity and display name come from the declarative registry, so an adapter
// cannot rename the client it serves. An adapter that does not probe surfaces
// at all reports as not detected rather than disappearing from the listing.
func (detector Detector) detectClient(ctx context.Context, host clients.Host, adapter clients.Adapter, probeVersion bool) (domain.DetectedClient, error) {
	definition, _ := domain.ClientDefinitionFor(adapter.ID())
	client := domain.DetectedClient{
		ClientID:    adapter.ID(),
		DisplayName: definition.DisplayName,
		Status:      domain.DetectionNotDetected,
	}
	hostDetector, ok := clients.As[clients.HostDetector](detector.Registry, adapter.ID())
	if !ok {
		return client, nil
	}
	detection := hostDetector.DetectSurfaces(host)
	if detection.Err != nil {
		return client, detection.Err
	}
	client.Surfaces = detection.Surfaces
	client.ExecutablePath = detection.ExecutablePath
	client.ConfigRoot = detection.ConfigRoot
	client.Status = detectionStatus(detection)
	if probeVersion && client.Status == domain.DetectionDetected && client.ExecutablePath != "" && (detector.ProbeVersion != nil || detector.ProbeVersionWithEnvironment != nil) {
		timeout := detector.VersionTimeout
		if timeout <= 0 {
			timeout = 2 * time.Second
		}
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		var environment []string
		if profile, ok := adapter.(clients.VersionProbeEnvironment); ok {
			var err error
			environment, err = profile.VersionProbeEnvironment(client.ConfigRoot)
			if err != nil {
				return client, err
			}
		}
		var version string
		var err error
		if detector.ProbeVersion != nil {
			if len(environment) == 0 {
				version, err = detector.ProbeVersion(ctx, client.ExecutablePath)
			}
		} else if detector.ProbeVersionWithEnvironment != nil {
			version, err = detector.ProbeVersionWithEnvironment(ctx, client.ExecutablePath, environment)
		}
		// A legacy probe cannot carry profile authority. Leave the version unknown
		// rather than invoking Codex against a different or ambient profile.
		if err == nil {
			client.Version = normalizeVersion(version)
		}
	}
	return client, nil
}

// detectionStatus separates read-only discovery evidence from the surfaces that
// make a client safe to select for lifecycle work. An empty selection list
// means any detected surface is actionable. Clients with stricter authority
// requirements can retain ambient evidence without becoming an automatic or
// explicit mutation target.
func detectionStatus(detection clients.Detection) domain.DetectionStatus {
	selection := make(map[string]struct{}, len(detection.SelectionSurfaceIDs))
	for _, id := range detection.SelectionSurfaceIDs {
		selection[id] = struct{}{}
	}
	for _, surface := range detection.Surfaces {
		_, selectable := selection[surface.ID]
		if surface.Detected && (len(selection) == 0 || selectable) {
			return domain.DetectionDetected
		}
	}
	return domain.DetectionNotDetected
}

func environmentSnapshot() map[string]string {
	values := map[string]string{}
	for _, name := range []string{
		"APPDATA", "LOCALAPPDATA", "ProgramFiles", "ProgramW6432", "ProgramFiles(x86)",
		"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME",
		"CODEX_HOME", "CLAUDE_CONFIG_DIR", "GEMINI_CLI_HOME", "CLINE_DATA_DIR", "CLINE_MCP_SETTINGS_PATH",
	} {
		if value, ok := os.LookupEnv(name); ok {
			values[name] = value
		}
	}
	return values
}

func compactPaths(paths ...string) []string {
	result := make([]string, 0, len(paths))
	seen := map[string]struct{}{}
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		clean := filepath.Clean(path)
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		result = append(result, clean)
	}
	return result
}

func IsNotFound(err error) bool {
	return errors.Is(err, exec.ErrNotFound)
}
