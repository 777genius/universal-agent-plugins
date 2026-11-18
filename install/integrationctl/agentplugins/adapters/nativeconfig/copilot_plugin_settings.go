package nativeconfig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path"
	"reflect"
	"strings"
)

// CopilotPluginRegistration is the exact pair of repository-local settings
// entries owned by one managed Copilot Agent Plugin. The marketplace path is
// repository-relative so that moving or cloning the checkout keeps it usable.
type CopilotPluginRegistration struct {
	Marketplace string
	Plugin      string
	Directory   string
}

func (r CopilotPluginRegistration) Validate() error {
	if r.Marketplace == "" || r.Plugin == "" || r.Directory == "" ||
		strings.TrimSpace(r.Marketplace) != r.Marketplace ||
		strings.TrimSpace(r.Plugin) != r.Plugin ||
		strings.ContainsAny(r.Marketplace+r.Plugin, "\x00\r\n") ||
		strings.Contains(r.Plugin, "@") ||
		strings.ContainsAny(r.Directory, "\\\x00\r\n") ||
		path.IsAbs(r.Directory) || path.Clean(r.Directory) != r.Directory ||
		!strings.HasPrefix(r.Directory, ".uap/managed/clients/copilot/") {
		return fmt.Errorf("invalid Copilot project plugin registration")
	}
	for _, segment := range strings.Split(r.Directory, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return fmt.Errorf("invalid Copilot project marketplace path")
		}
	}
	return nil
}

func (r CopilotPluginRegistration) pluginKey() string {
	return r.Plugin + "@" + r.Marketplace
}

func (r CopilotPluginRegistration) marketplaceValue() map[string]any {
	return map[string]any{"source": map[string]any{"source": "directory", "path": r.Directory}}
}

// InspectCopilotPluginSettings is read-only: a partially occupied namespace
// is still occupied, but cannot prove an installed, owned registration.
func InspectCopilotPluginSettings(body []byte, r CopilotPluginRegistration) (present, exact bool, err error) {
	if err := r.Validate(); err != nil {
		return false, false, err
	}
	doc, err := copilotPluginDocument(body)
	if err != nil {
		return false, false, err
	}
	return inspectCopilotPluginDocument(doc, r)
}

func copilotPluginDocument(body []byte) (*document, error) {
	if len(body) == 0 {
		body = []byte("{}\n")
	}
	return parseDocument(body, true)
}

func inspectCopilotPluginDocument(doc *document, r CopilotPluginRegistration) (bool, bool, error) {
	marketplaces, err := objectCollection(doc.root, "extraKnownMarketplaces", false)
	if err != nil {
		return false, false, err
	}
	enabled, err := objectCollection(doc.root, "enabledPlugins", false)
	if err != nil {
		return false, false, err
	}
	var marketPresent, enabledPresent, marketExact, enabledExact bool
	if marketplaces != nil {
		if member, _ := objectMember(marketplaces, r.Marketplace); member != nil {
			marketPresent = true
			raw, err := entryCanonical(member)
			if err != nil {
				return false, false, err
			}
			var existing map[string]any
			if err := json.Unmarshal(raw, &existing); err != nil {
				return false, false, err
			}
			marketExact = reflect.DeepEqual(existing, r.marketplaceValue())
		}
	}
	if enabled != nil {
		if member, _ := objectMember(enabled, r.pluginKey()); member != nil {
			enabledPresent = true
			raw, err := entryCanonical(member)
			if err != nil {
				return false, false, err
			}
			enabledExact = bytes.Equal(raw, []byte("true"))
		}
	}
	return marketPresent || enabledPresent, marketPresent && enabledPresent && marketExact && enabledExact, nil
}

// PatchCopilotPluginSettings edits only this package's two namespaced entries.
// owned must come from an already verified UAP binding, never the file itself.
// Foreign entries, including identical unowned ones, are refused rather than
// silently adopted. The caller owns exact-file locking, CAS and readback.
func PatchCopilotPluginSettings(body []byte, r CopilotPluginRegistration, owned, remove bool) ([]byte, bool, error) {
	if err := r.Validate(); err != nil {
		return nil, false, err
	}
	doc, err := copilotPluginDocument(body)
	if err != nil {
		return nil, false, err
	}
	present, exact, err := inspectCopilotPluginDocument(doc, r)
	if err != nil {
		return nil, false, err
	}
	if owned {
		// An entirely absent pair can be restored or removed using the
		// previously verified state receipt. A partial or changed pair cannot:
		// it may have been replaced by a concurrent native/foreign writer.
		if present && !exact {
			return nil, false, fmt.Errorf("%w: Copilot project registration changed or became partial", ErrNotOwned)
		}
	} else if present {
		return nil, false, fmt.Errorf("%w: Copilot project registration namespace already occupied", ErrCollision)
	}
	if remove {
		if !owned || !present {
			return bytes.Clone(body), false, nil
		}
		marketplaces, _ := objectCollection(doc.root, "extraKnownMarketplaces", false)
		enabled, _ := objectCollection(doc.root, "enabledPlugins", false)
		removeEntry(marketplaces, r.Marketplace)
		removeEntry(enabled, r.pluginKey())
	} else {
		if exact {
			return bytes.Clone(body), false, nil
		}
		if err := addCopilotPluginSettingsMembers(doc, r); err != nil {
			return nil, false, err
		}
	}
	next, err := doc.render()
	if err != nil {
		return nil, false, err
	}
	// Match the native file reader's exact 1 MiB bound before any CAS.
	// A valid near-limit preimage must never become an unreadable commit.
	if len(next) > 1<<20 {
		return nil, false, fmt.Errorf("copilot project settings would exceed the 1 MiB inspection limit")
	}
	return next, true, nil
}

func addCopilotPluginSettingsMembers(doc *document, r CopilotPluginRegistration) error {
	marketplaces, err := objectCollection(doc.root, "extraKnownMarketplaces", true)
	if err != nil {
		return err
	}
	enabled, err := objectCollection(doc.root, "enabledPlugins", true)
	if err != nil {
		return err
	}
	value, err := jsonValue(r.marketplaceValue())
	if err != nil {
		return err
	}
	setEntry(marketplaces, r.Marketplace, value)
	value, err = jsonValue(true)
	if err != nil {
		return err
	}
	setEntry(enabled, r.pluginKey(), value)
	return nil
}
