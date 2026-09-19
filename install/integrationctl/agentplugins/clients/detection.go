package clients

import (
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
