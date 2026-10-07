package vscode

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func cleanLocalPath(path string) bool {
	return path != "" && len(path) <= 4096 && utf8.ValidString(path) && strings.TrimSpace(path) == path &&
		!strings.ContainsAny(path, "\x00\r\n") && filepath.IsAbs(path) && filepath.Clean(path) == path &&
		!strings.HasPrefix(path, "//") && !strings.HasPrefix(path, `\\`)
}

func resolveSettingsPath(path string) (string, error) {
	if !cleanLocalPath(path) || filepath.Base(path) != "settings.json" {
		return "", fmt.Errorf("local requires explicit profile settings.json")
	}
	root, err := clients.CanonicalDirectory(filepath.Dir(path), os.Lstat, filepath.EvalSymlinks)
	if err != nil {
		return "", err
	}
	physical := filepath.Join(root, "settings.json")
	if _, err := nativeconfig.New().ReadExactFile(physical); err != nil {
		return "", err
	}
	return physical, nil
}

func validatePhysicalSettings(path string) error {
	physical, err := resolveSettingsPath(path)
	if err != nil {
		return err
	}
	if physical != path {
		return fmt.Errorf("local physical profile changed or is aliased")
	}
	return nil
}

func (a *LocalAdapter) ResolveProfileRoot(root string) (string, error) {
	if !cleanLocalPath(root) {
		return "", fmt.Errorf("local profile root must be explicit and absolute")
	}
	physical, err := clients.CanonicalDirectory(root, os.Lstat, filepath.EvalSymlinks)
	if err != nil {
		return "", err
	}
	if physical != filepath.Dir(a.config.ProfileSettingsPath) {
		return "", fmt.Errorf("local selected profile differs from constructor")
	}
	return physical, validatePhysicalSettings(a.config.ProfileSettingsPath)
}

func (a *LocalAdapter) DetectSurfaces(host clients.Host) clients.Detection {
	root := filepath.Dir(a.config.ProfileSettingsPath)
	physical, err := host.CanonicalDirectory(root)
	if err != nil || physical != root {
		return clients.Detection{Err: fmt.Errorf("local selected physical profile unavailable")}
	}
	return clients.Detection{ConfigRoot: root, Surfaces: []domain.ClientSurface{host.DirectorySurface("vscode_local_selected_TEST_profile", root)}}
}

func (*LocalAdapter) NativeRegistry(in clients.PlanInput) (string, string) {
	return in.Client.ConfigRoot, ""
}

func (*LocalAdapter) TargetRoot(_ domain.DetectedClient, _ domain.PackageMode, managed string) (string, string, error) {
	if !cleanLocalPath(managed) {
		return "", "", fmt.Errorf("local managed root required")
	}
	return managed, filepath.Join(managed, "vscode-local"), nil
}

// ValidateBindingProfile consumes persisted authority, including the native
// selector preimage. It creates no profile lock and never reads ambient HOME.
func (*LocalAdapter) ValidateBindingProfile(root string, binding domain.ClientBinding) error {
	facts, err := recordedLocalFacts(binding.SelectedDelivery)
	if err != nil {
		return err
	}
	if root != facts.ProfileRoot || binding.NativeProfileRoot != root || binding.TargetLocator != facts.Registration.Selector {
		return fmt.Errorf("local recorded physical profile/target differs")
	}
	if !binding.SelectedDelivery.OwnsProfileEntry(binding.NativeObjects) {
		return fmt.Errorf("local exact owned selector receipt required")
	}
	if binding.LocalEntryObservation != nil && !localSameBasis(binding.SelectedDelivery, binding.LocalEntryObservation) {
		if binding.PendingNativeIntent == nil {
			return fmt.Errorf("local recorded observation is stale")
		}
		if err := binding.PendingNativeIntent.Validate(binding); err != nil {
			return err
		}
	}
	_, err = inspectObservedRegistration(nativeconfig.New(), binding.SelectedDelivery, binding.NativeObjects, binding.LocalEntryObservation)
	return err
}

func recordedLocalFacts(selected domain.SelectedDelivery) (domain.LocalDeliveryFacts, error) {
	if err := selected.Validate(); err != nil {
		return domain.LocalDeliveryFacts{}, err
	}
	facts, ok := selected.LocalFacts()
	if !ok {
		return facts, fmt.Errorf("local selected delivery required; historical CLI receipt is not Local authority")
	}
	if !sourceTupleMatches(facts.Tuple, facts.NativeStop) || facts.Tuple.TargetOS != runtime.GOOS {
		return facts, fmt.Errorf("local recorded tuple unqualified")
	}
	if facts.ProfileIdentity != facts.ProfileRoot || facts.SettingsIdentity != facts.SettingsPath || facts.Registration.ObjectID != localObjectID(facts.SettingsPath, facts.Registration.Selector) {
		return facts, fmt.Errorf("local recorded physical identities differ")
	}
	return facts, validatePhysicalSettings(facts.SettingsPath)
}

func localObjectID(settings, selector string) string {
	return "vscode-local:" + localHash([]byte(settings+"\x00"+selector))
}

func ownedLocalObjects(facts domain.LocalDeliveryFacts, objects []domain.NativeObjectOwnership) (bool, error) {
	expected := facts.Registration.Ownership(facts.SettingsPath)
	owned := false
	for _, object := range objects {
		if object.Kind == "managed_package_directory" {
			continue
		}
		if owned || !reflect.DeepEqual(object, expected) {
			return false, fmt.Errorf("local native receipt outside recorded selector")
		}
		owned = true
	}
	return owned, nil
}
