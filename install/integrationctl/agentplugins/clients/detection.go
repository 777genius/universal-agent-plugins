package clients

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"io/fs"
	"os"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// HostDetector is the read-only surface probe of one client. It returns what it
// observed; assembling a domain.DetectedClient (status, display name, version
// probe) stays generic in adapters/clientdetect.
type HostDetector interface {
	DetectSurfaces(host Host) Detection
}

// Detection is the raw outcome of probing one client's surfaces.
type Detection struct {
	// Err prevents an invalid selected profile from becoming mutation authority.
	Err            error
	ConfigRoot     string
	ExecutablePath string
	Surfaces       []domain.ClientSurface
	// SelectionSurfaceIDs narrows the surfaces that decide detection status to
	// the ones that also make the client safe to act on. An empty list means any
	// detected surface counts. Claude is the only client that needs it today: a
	// configuration directory stays evidence, but only the CLI selects it.
	SelectionSurfaceIDs []string
}

// Host is the probing environment handed to an adapter. The surface
// constructors stay here rather than in each client package: they encode how
// evidence strings are produced, which is a cross-client output contract.
type Host interface {
	HomeDir() string
	WorkingDir() string
	CanonicalDirectory(path string) (string, error)
	GOOS() string
	Env(name string) string
	SystemApplicationsDir() string
	WindowsProgramFiles() []string
	LinuxApplicationDirs() []string

	// LookPath returns the resolved executable path, or "" when the binary is
	// not on PATH. It never reports the lookup error: absence is the answer.
	LookPath(binary string) string
	Lstat(path string) (fs.FileInfo, error)
	ReadDir(path string) ([]os.DirEntry, error)

	BinarySurface(id, binary string) domain.ClientSurface
	// ResolvedBinarySurface reports the same surface for an executable the
	// adapter already resolved, so a client that also returns that path as its
	// ExecutablePath probes PATH once instead of twice.
	ResolvedBinarySurface(id, executablePath string) domain.ClientSurface
	DirectorySurface(id, path string) domain.ClientSurface
	AppSurface(id, appName string) domain.ClientSurface
	WindowsAppSurface(id, userRelativePath, systemRelativePath string) domain.ClientSurface
	LinuxDesktopSurface(id string, filenames ...string) domain.ClientSurface
	ExtensionSurface(id, root string, extensionIDs ...string) domain.ClientSurface

	XDGConfigRoot(name string) string
	EditorChannelConfigRoot(channel string) string
	VSCodeConfigRoot() string
}

// ProfileResolver normalizes an explicit profile at a composition boundary.
// Implementations must reject ambiguous input before resolving symlink aliases.
type ProfileResolver interface {
	ResolveProfileRoot(root string) (string, error)
}

// VersionProbeEnvironment pins a native version probe to the selected profile.
// The detector still controls the isolated cwd, timeout and output limit.
type VersionProbeEnvironment interface {
	VersionProbeEnvironment(configRoot string) ([]string, error)
}

// ProfileBindingValidator checks persisted profile authority before lifecycle
// work, including removal paths that do not inspect a native registry.
type ProfileBindingValidator interface {
	ValidateBindingProfile(root string, binding domain.ClientBinding) error
}

// PhysicalProfileAuthority is the single optional port, shared with the planner.
type PhysicalProfileAuthority = ports.PhysicalProfileAuthority
