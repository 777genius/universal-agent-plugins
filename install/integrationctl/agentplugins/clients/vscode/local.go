package vscode

import (
	"encoding/hex"
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/vscodelocalhooks"
)

// TESTQualification identifies source contracts, never production/native admission.
// The built-in Copilot source is pinned with VS Code; no VSIX version is inferred.
const TESTQualification = "TEST-source-07f806f999227108933c2e30515b26eecc1fda74"

// SourceQualifiedTESTTuple returns the sole admitted source tuple. Windows is
// preparation only; native Windows shell/profile execution needs separate CI.
func SourceQualifiedTESTTuple(goos string) domain.LocalQualifiedTuple {
	shell := string(vscodelocalhooks.LinuxSH)
	if goos == "windows" {
		shell = string(vscodelocalhooks.WindowsPowerShell51)
	}
	return domain.LocalQualifiedTuple{VSCodeVersion: "1.140.0", CopilotVersion: "source-07f806f999227108933c2e30515b26eecc1fda74", TargetOS: goos, TargetShell: shell, QualificationID: TESTQualification}
}

// DarwinTESTQualification binds only the retained B17 public-SDK Stop receipt
// on macOS 15.6.1 arm64/APFS. It is not installed-product or release proof.
const DarwinTESTQualification = "TEST-B17-macos-15.6.1-arm64-APFS-fa254ee10963e8cee50a2f2dab5cf9758a2937596c3169fae9396d3f7b98a349"

// QualifiedDarwinTESTTuple is separate from the source-only Linux/Windows tuple.
// The host supplies this exact qualification; the runtime must also be arm64,
// and changed writes must pass the held APFS backend's own admission.
func QualifiedDarwinTESTTuple() domain.LocalQualifiedTuple {
	return domain.LocalQualifiedTuple{VSCodeVersion: "1.140.0", CopilotVersion: "0.68.0", TargetOS: "darwin", TargetShell: string(vscodelocalhooks.MacOSSH), QualificationID: DarwinTESTQualification}
}

// LocalConfig supplies explicit, checked host inputs. Hook executable identity
// is verified by the host BEFORE construction; this adapter neither selects nor
// downloads a runtime. Args are fixed literals with optional PLUGIN_ROOT/DATA
// tokens. MCP and skill names are independent selections, absent by default.
type LocalConfig struct {
	ProfileSettingsPath string
	QualifiedTuple      domain.LocalQualifiedTuple
	TargetShell         vscodelocalhooks.Target
	NativeStop          bool
	HookSpecs           []vscodelocalhooks.Spec
	DeclaredHookDigest  string
	MCPServers, Skills  []string
}

// LocalAdapter implements explicit selected-profile Local delivery. It shares
// the historical client ID, but embeds none of New's CLI lifecycle capabilities.
type LocalAdapter struct{ config LocalConfig }

func NewLocal(config LocalConfig) (*LocalAdapter, error) {
	config.HookSpecs = cloneSpecs(config.HookSpecs)
	config.MCPServers, config.Skills = slices.Clone(config.MCPServers), slices.Clone(config.Skills)
	if err := validateLocalConfig(config); err != nil {
		return nil, err
	}
	physical, err := resolveSettingsPath(config.ProfileSettingsPath)
	if err != nil {
		return nil, err
	}
	config.ProfileSettingsPath = physical
	return &LocalAdapter{config: config}, nil
}

func (*LocalAdapter) ID() domain.ClientID { return domain.ClientVSCode }

func cloneSpecs(specs []vscodelocalhooks.Spec) []vscodelocalhooks.Spec {
	out := slices.Clone(specs)
	for i := range out {
		out[i].Args = slices.Clone(out[i].Args)
	}
	return out
}

func validateLocalConfig(c LocalConfig) error {
	if !localTupleMatches(c.QualifiedTuple, c.NativeStop) ||
		c.QualifiedTuple.TargetOS != runtime.GOOS || c.QualifiedTuple.TargetShell != string(c.TargetShell.Shell) {
		return fmt.Errorf("local capability_unverified: exact qualified TEST tuple and executor required")
	}
	if !selectedNames(c.MCPServers) || !selectedNames(c.Skills) {
		return fmt.Errorf("local selections require unique sorted component names")
	}
	if c.NativeStop && (len(c.HookSpecs) != 1 || c.HookSpecs[0].Event != vscodelocalhooks.Stop) {
		return fmt.Errorf("local native selection requires exactly the fixed Stop spec")
	}
	if !c.NativeStop && len(c.HookSpecs) != 0 {
		return fmt.Errorf("local unselected native route must have no specs")
	}
	if c.NativeStop && !validLocalDigest(c.DeclaredHookDigest) {
		return fmt.Errorf("local declared hook digest required")
	}
	if c.DeclaredHookDigest != "" && !validLocalDigest(c.DeclaredHookDigest) {
		return fmt.Errorf("invalid Local declared hook digest")
	}
	if c.NativeStop {
		_, err := renderSpecs(c, "/TEST-plugin", "/TEST-data")
		return err
	}
	return nil
}

func localTupleMatches(tuple domain.LocalQualifiedTuple, native bool) bool {
	var expected domain.LocalQualifiedTuple
	switch tuple.TargetOS {
	case "linux", "windows":
		expected = SourceQualifiedTESTTuple(tuple.TargetOS)
	case "darwin":
		if runtime.GOARCH != "arm64" {
			return false
		}
		expected = QualifiedDarwinTESTTuple()
	default:
		return false
	}
	if !native && tuple.TargetShell == "" {
		expected.TargetShell = ""
	}
	return tuple == expected
}

func selectedNames(names []string) bool {
	if !slices.IsSorted(names) {
		return false
	}
	previous := ""
	for _, name := range names {
		if name == "" || filepath.Base(name) != name || name == "." || name == ".." || strings.ContainsAny(name, "\\/\x00\r\n") || previous == name {
			return false
		}
		previous = name
	}
	return true
}

func validLocalDigest(digest string) bool {
	if len(digest) != 71 || !strings.HasPrefix(digest, "sha256:") {
		return false
	}
	_, err := hex.DecodeString(digest[7:])
	return err == nil && strings.ToLower(digest) == digest
}

var (
	_ clients.Adapter                    = (*LocalAdapter)(nil)
	_ clients.HostDetector               = (*LocalAdapter)(nil)
	_ clients.ProfileResolver            = (*LocalAdapter)(nil)
	_ clients.ProfileBindingValidator    = (*LocalAdapter)(nil)
	_ clients.NativeRegistryLayout       = (*LocalAdapter)(nil)
	_ clients.TargetLayout               = (*LocalAdapter)(nil)
	_ clients.PlanPrecondition           = (*LocalAdapter)(nil)
	_ clients.LocalPreparationAuthorizer = (*LocalAdapter)(nil)
	_ clients.PlanQualifier              = (*LocalAdapter)(nil)
	_ clients.PlanRefiner                = (*LocalAdapter)(nil)
	_ clients.Projector                  = (*LocalAdapter)(nil)
	_ clients.ActiveNativeProjector      = (*LocalAdapter)(nil)
	_ clients.Lifecycle                  = (*LocalAdapter)(nil)
	_ clients.AutomaticActivator         = (*LocalAdapter)(nil)
	_ clients.ActivationPreflighter      = (*LocalAdapter)(nil)
	_ clients.ReadOnlyVerifier           = (*LocalAdapter)(nil)
	_ clients.RegistryInspector          = (*LocalAdapter)(nil)
)
