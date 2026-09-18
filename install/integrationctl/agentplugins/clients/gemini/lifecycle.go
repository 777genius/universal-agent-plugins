package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/filetree"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func ActivateNative(ctx context.Context, env clients.Env, request domain.ActivationRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return applyGeminiNative(env, clients.Ops{}, request.Client.ConfigRoot, request.Delivery.ActivePath, request.PreviousNativeObjects, request.Delivery.NativeObjects, nil)
}

func DeactivateNative(ctx context.Context, env clients.Env, request domain.DeactivationRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return applyGeminiNative(env, clients.Ops{}, request.Client.ConfigRoot, "", request.NativeObjects, nil, nil)
}

type geminiRenameFunc func(string, string) error

func renameGeminiDirectoryNoReplace(oldPath, newPath string, rename geminiRenameFunc) error {
	if _, err := os.Lstat(newPath); err == nil {
		return fmt.Errorf("destination already exists: %s", newPath)
	} else if !os.IsNotExist(err) {
		return err
	}
	return rename(oldPath, newPath)
}

func applyGeminiNativeMutation(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership) error {
	return applyGeminiNative(clients.Env{NativeConfig: nativeconfig.New()}, clients.Ops{}, configRoot, activePath, previous, desired, nil)
}

func applyGeminiNative(env clients.Env, ops clients.Ops, configRoot, activePath string, previous, desired []domain.NativeObjectOwnership, capacity shared.CombinedCapacityFunc) (resultErr error) {
	rename := ops.Rename
	if rename == nil {
		rename = shared.RenameDirectoryExclusive
	}
	if capacity == nil {
		capacity = shared.CheckedCombinedCapacity
	}
	kernel := env.NativeConfig
	if rename == nil {
		return fmt.Errorf("Gemini rename operation is unavailable")
	}
	if capacity == nil {
		return fmt.Errorf("Gemini capacity checker is unavailable")
	}
	configRoot = strings.TrimSpace(configRoot)
	if configRoot == "" || !filepath.IsAbs(configRoot) {
		return fmt.Errorf("Gemini config root is unavailable")
	}
	previous, desired = NativeObjects(previous), NativeObjects(desired)
	if err := VerifyNativeObjects(configRoot, previous, true); err != nil {
		return err
	}
	previousByID, desiredByID := shared.ObjectMap(previous), shared.ObjectMap(desired)
	idsCapacity, capacityErr := capacity(len(previousByID), len(desiredByID))
	if capacityErr != nil {
		return fmt.Errorf("prepare managed Gemini object set: %w", capacityErr)
	}
	for id, object := range desiredByID {
		if prior, replacing := previousByID[id]; replacing {
			if prior.Kind != object.Kind || prior.LogicalName != object.LogicalName || !shared.SameCleanPath(prior.Path, object.Path) {
				return fmt.Errorf("Gemini native object identity changed unexpectedly for %s", id)
			}
			continue
		}
		if err := requireGeminiObjectAbsent(configRoot, object); err != nil {
			return err
		}
	}

	descriptor := geminiDescriptor{}
	if len(desired) > 0 {
		if strings.TrimSpace(activePath) == "" {
			return fmt.Errorf("active package path is required for Gemini native installation")
		}
		body, err := os.ReadFile(filepath.Join(activePath, geminiDescriptorName))
		if err != nil || json.Unmarshal(body, &descriptor) != nil || strings.TrimSpace(descriptor.DataRoot) == "" || !filepath.IsAbs(descriptor.DataRoot) {
			return fmt.Errorf("read Gemini projection descriptor: invalid or missing descriptor")
		}
	}

	transactionRoot := ""
	cleanupTransaction := true
	if hasGeminiSkillObjects(previous) || hasGeminiSkillObjects(desired) {
		skillsRoot := filepath.Join(configRoot, "skills")
		if err := pathpolicy.RequireContainedChild(configRoot, skillsRoot); err != nil {
			return err
		}
		if err := os.MkdirAll(skillsRoot, 0o700); err != nil {
			return fmt.Errorf("create Gemini skills root: %w", err)
		}
		var err error
		transactionRoot, err = os.MkdirTemp(skillsRoot, ".agentplugins-native-")
		if err != nil {
			return fmt.Errorf("create Gemini native transaction: %w", err)
		}
	}

	staged := map[string]string{}
	for id, object := range desiredByID {
		if object.Kind != geminiSkillObjectKind {
			continue
		}
		source := filepath.Join(activePath, filepath.FromSlash(object.SourceRelative))
		if err := pathpolicy.RequireContainedChild(activePath, source); err != nil {
			return fmt.Errorf("unsafe Gemini skill source for %q: %w", object.LogicalName, err)
		}
		target := filepath.Join(transactionRoot, "new-"+object.LogicalName)
		if err := filetree.CopyDir(source, target); err != nil {
			return fmt.Errorf("stage Gemini skill %q: %w", object.LogicalName, err)
		}
		if digest, err := shared.DigestSkillDirectory(target); err != nil || digest != object.ManagedDigest {
			return fmt.Errorf("staged Gemini skill %q does not match its ownership digest", object.LogicalName)
		}
		staged[id] = target
	}

	backups, installed := map[string]string{}, map[string]domain.NativeObjectOwnership{}
	rollbackSkills := func() error {
		var rollbackErr error
		attempted := map[string]bool{}
		for id, object := range installed {
			attempted[id] = true
			if digest, err := shared.DigestSkillDirectory(object.Path); err == nil && digest == object.ManagedDigest {
				if err := os.RemoveAll(object.Path); err != nil && rollbackErr == nil {
					rollbackErr = err
				}
			}
			if backup := backups[id]; backup != "" {
				if err := renameGeminiDirectoryNoReplace(backup, object.Path, rename); err != nil {
					if rollbackErr == nil {
						rollbackErr = err
					}
				} else {
					delete(backups, id)
				}
			}
		}
		for id, backup := range backups {
			if attempted[id] {
				continue
			}
			if err := renameGeminiDirectoryNoReplace(backup, previousByID[id].Path, rename); err != nil {
				if rollbackErr == nil {
					rollbackErr = err
				}
			} else {
				delete(backups, id)
			}
		}
		return rollbackErr
	}
	defer func() {
		if cleanupTransaction && transactionRoot != "" {
			_ = os.RemoveAll(transactionRoot)
		}
	}()
	// A failed restore leaves the only recoverable copy inside transactionRoot.
	// Keep that directory intact and report its exact location to the caller.
	defer func() {
		if resultErr == nil || nativeconfig.IsCommittedCleanup(resultErr) {
			return
		}
		if rollbackErr := rollbackSkills(); rollbackErr != nil {
			cleanupTransaction = false
			resultErr = fmt.Errorf("%v; Gemini skill rollback failed: %w; recovery retained at %q", resultErr, rollbackErr, transactionRoot)
		}
	}()
	for id, object := range previousByID {
		if object.Kind != geminiSkillObjectKind {
			continue
		}
		if _, err := os.Lstat(object.Path); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		}
		backup := filepath.Join(transactionRoot, "old-"+object.LogicalName)
		if err := rename(object.Path, backup); err != nil {
			return fmt.Errorf("backup Gemini skill %q: %w", object.LogicalName, err)
		}
		backups[id] = backup
		digest, digestErr := shared.DigestSkillDirectory(backup)
		if digestErr != nil || digest != object.ManagedDigest {
			if digestErr != nil {
				return fmt.Errorf("verify isolated Gemini skill backup %q: %w", object.LogicalName, digestErr)
			}
			return fmt.Errorf("isolated Gemini skill backup %q changed outside agentplugins", object.LogicalName)
		}
	}
	for id, source := range staged {
		object := desiredByID[id]
		if err := renameGeminiDirectoryNoReplace(source, object.Path, rename); err != nil {
			return fmt.Errorf("activate Gemini skill %q: %w", object.LogicalName, err)
		}
		installed[id] = object
	}

	requests := make([]nativeconfig.Request, 0)
	ids := make([]string, 0, idsCapacity)
	seen := map[string]bool{}
	for id := range previousByID {
		ids, seen[id] = append(ids, id), true
	}
	for id := range desiredByID {
		if !seen[id] {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		prior, hadPrior := previousByID[id]
		next, hasNext := desiredByID[id]
		if (hadPrior && prior.Kind != geminiMCPObjectKind) || (hasNext && next.Kind != geminiMCPObjectKind) {
			continue
		}
		if hasNext {
			server, err := geminiServerFromPackage(activePath, next.LogicalName)
			if err != nil {
				return err
			}
			server, err = materializeGeminiServer(server, activePath, descriptor.DataRoot)
			if err != nil {
				return err
			}
			present, owned := false, false
			if hadPrior {
				present, owned, err = nativeconfig.New().Inspect(geminiConfigPaths(configRoot), nativeconfig.CodecGemini, prior.LogicalName, geminiReceipt(prior))
				if err != nil {
					return err
				}
			}
			action := nativeconfig.ActionAdd
			var receipt *nativeconfig.Receipt
			if present {
				if !owned {
					return fmt.Errorf("Gemini MCP server %q is no longer owned", prior.LogicalName)
				}
				action, receipt = nativeconfig.ActionUpdate, geminiReceipt(prior)
			}
			requests = append(requests, nativeconfig.Request{Paths: geminiConfigPaths(configRoot), Codec: nativeconfig.CodecGemini, Action: action, Name: next.LogicalName, Server: server, Placeholders: nativeconfig.Placeholders{PackageRoot: activePath, DataRoot: descriptor.DataRoot}, Owned: receipt})
		} else if hadPrior {
			present, owned, err := nativeconfig.New().Inspect(geminiConfigPaths(configRoot), nativeconfig.CodecGemini, prior.LogicalName, geminiReceipt(prior))
			if err != nil {
				return err
			}
			if present {
				if !owned {
					return fmt.Errorf("Gemini MCP server %q is no longer owned", prior.LogicalName)
				}
				requests = append(requests, nativeconfig.Request{Paths: geminiConfigPaths(configRoot), Codec: nativeconfig.CodecGemini, Action: nativeconfig.ActionRemove, Name: prior.LogicalName, Owned: geminiReceipt(prior)})
			}
		}
	}
	for _, request := range requests {
		if request.Action == nativeconfig.ActionRemove {
			continue
		}
		expected := desiredByID["gemini-mcp:"+request.Name]
		preview, err := nativeconfig.DesiredReceipt(filepath.Join(configRoot, "settings.json"), nativeconfig.CodecGemini, request.Name, request.Server, nativeconfig.Placeholders{PackageRoot: activePath, DataRoot: descriptor.DataRoot})
		if err != nil || preview.Digest != expected.ManagedDigest {
			return fmt.Errorf("Gemini MCP server %q does not match staged ownership", request.Name)
		}
	}
	_, err := kernel.ApplyBatch(requests)
	if err != nil {
		return err
	}
	backups = map[string]string{}
	return nil
}
