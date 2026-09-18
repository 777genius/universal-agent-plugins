package cline

import (
	"context"
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
	return applyClineNative(env, clients.Ops{}, request.Client.ConfigRoot, request.Delivery.ActivePath, request.PreviousNativeObjects, request.Delivery.NativeObjects, nil)
}

func DeactivateNative(ctx context.Context, env clients.Env, request domain.DeactivationRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return applyClineNative(env, clients.Ops{}, request.Client.ConfigRoot, "", request.NativeObjects, nil, nil)
}

type clineRenameFunc func(string, string) error

func renameClineDirectoryNoReplace(oldPath, newPath string, rename clineRenameFunc) error {
	return rename(oldPath, newPath)
}

func applyClineNativeMutation(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership) error {
	return applyClineNative(clients.Env{NativeConfig: nativeconfig.New()}, clients.Ops{}, configRoot, activePath, previous, desired, nil)
}

func applyClineNative(env clients.Env, ops clients.Ops, configRoot, activePath string, previous, desired []domain.NativeObjectOwnership, capacity shared.CombinedCapacityFunc) (resultErr error) {
	rename := ops.Rename
	if rename == nil {
		rename = shared.RenameDirectoryExclusive
	}
	if capacity == nil {
		capacity = shared.CheckedCombinedCapacity
	}
	kernel := env.NativeConfig
	if rename == nil {
		return fmt.Errorf("Cline rename operation is unavailable")
	}
	if capacity == nil {
		return fmt.Errorf("Cline capacity checker is unavailable")
	}
	configRoot = strings.TrimSpace(configRoot)
	if configRoot == "" || !filepath.IsAbs(configRoot) {
		return fmt.Errorf("Cline config root is unavailable")
	}
	previous, desired = NativeObjects(previous), NativeObjects(desired)
	if err := VerifyNativeObjects(configRoot, previous, true); err != nil {
		return err
	}
	previousByID, desiredByID := shared.ObjectMap(previous), shared.ObjectMap(desired)
	idsCapacity, capacityErr := capacity(len(previousByID), len(desiredByID))
	if capacityErr != nil {
		return fmt.Errorf("prepare managed Cline MCP server set: %w", capacityErr)
	}
	for id, object := range desiredByID {
		if prior, replacing := previousByID[id]; replacing {
			if prior.Kind != object.Kind || prior.LogicalName != object.LogicalName || !shared.SameCleanPath(prior.Path, object.Path) {
				return fmt.Errorf("Cline native object identity changed unexpectedly for %s", id)
			}
			continue
		}
		if object.Kind == clineSkillObjectKind {
			if _, err := os.Lstat(object.Path); err == nil {
				return fmt.Errorf("Cline skill %q already exists without agentplugins ownership", object.LogicalName)
			} else if !os.IsNotExist(err) {
				return err
			}
		}
	}

	skillsRoot := filepath.Join(configRoot, "skills")
	if err := pathpolicy.RequireContainedChild(configRoot, skillsRoot); err != nil {
		return err
	}
	if err := os.MkdirAll(skillsRoot, 0o700); err != nil {
		return fmt.Errorf("create Cline skills root: %w", err)
	}
	transactionRoot, err := os.MkdirTemp(skillsRoot, ".agentplugins-native-")
	if err != nil {
		return err
	}
	cleanupTransaction := true

	staged := map[string]string{}
	for id, object := range desiredByID {
		if object.Kind != clineSkillObjectKind {
			continue
		}
		if activePath == "" {
			return fmt.Errorf("active package path is required for Cline skill installation")
		}
		source := filepath.Join(activePath, filepath.FromSlash(object.SourceRelative))
		if err := pathpolicy.RequireContainedChild(activePath, source); err != nil {
			return err
		}
		target := filepath.Join(transactionRoot, "new-"+object.LogicalName)
		if err := filetree.CopyDir(source, target); err != nil {
			return err
		}
		digest, err := shared.DigestSkillDirectory(target)
		if err != nil || digest != object.ManagedDigest {
			return fmt.Errorf("staged Cline skill %q does not match its ownership digest", object.LogicalName)
		}
		staged[id] = target
	}

	backups := map[string]string{}
	installed := map[string]domain.NativeObjectOwnership{}
	rollbackSkills := func() error {
		var first error
		attempted := map[string]bool{}
		for id, object := range installed {
			attempted[id] = true
			if digest, err := shared.DigestSkillDirectory(object.Path); err == nil && digest == object.ManagedDigest {
				if err := os.RemoveAll(object.Path); err != nil && first == nil {
					first = err
				}
			}
			if backup := backups[id]; backup != "" {
				if err := renameClineDirectoryNoReplace(backup, object.Path, rename); err != nil {
					if first == nil {
						first = err
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
			if err := renameClineDirectoryNoReplace(backup, previousByID[id].Path, rename); err != nil {
				if first == nil {
					first = err
				}
			} else {
				delete(backups, id)
			}
		}
		return first
	}
	defer func() {
		if cleanupTransaction {
			_ = os.RemoveAll(transactionRoot)
		}
	}()
	defer func() {
		if resultErr != nil && !nativeconfig.IsCommittedCleanup(resultErr) {
			if rollbackErr := rollbackSkills(); rollbackErr != nil {
				cleanupTransaction = false
				resultErr = fmt.Errorf("%v; Cline skill rollback failed: %w; recovery retained at %q", resultErr, rollbackErr, transactionRoot)
			}
		}
	}()

	for id, object := range previousByID {
		if object.Kind != clineSkillObjectKind {
			continue
		}
		if _, err := os.Lstat(object.Path); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		}
		backup := filepath.Join(transactionRoot, "old-"+object.LogicalName)
		if err := rename(object.Path, backup); err != nil {
			return err
		}
		backups[id] = backup
		digest, digestErr := shared.DigestSkillDirectory(backup)
		if digestErr != nil || digest != object.ManagedDigest {
			if digestErr != nil {
				return fmt.Errorf("verify isolated Cline skill backup %q: %w", object.LogicalName, digestErr)
			}
			return fmt.Errorf("isolated Cline skill backup %q changed outside agentplugins", object.LogicalName)
		}
	}
	for id, source := range staged {
		object := desiredByID[id]
		if err := renameClineDirectoryNoReplace(source, object.Path, rename); err != nil {
			return err
		}
		installed[id] = object
	}

	// Prove the filesystem half before committing the single ownership-aware MCP
	// batch. ApplyBatch is the final manager operation, so a reported config
	// failure can still roll the skills back safely.
	if err := VerifyNativeObjects(configRoot, clineSkillObjects(desired), false); err != nil {
		return err
	}
	if err := mutateClineMCPWithKernelAndCapacity(configRoot, activePath, previousByID, desiredByID, kernel, idsCapacity); err != nil {
		return err
	}
	return nil
}

func mutateClineMCP(configRoot, activePath string, previous, desired map[string]domain.NativeObjectOwnership) error {
	return mutateClineMCPWithKernel(configRoot, activePath, previous, desired, nativeconfig.New())
}

func mutateClineMCPWithKernel(configRoot, activePath string, previous, desired map[string]domain.NativeObjectOwnership, kernel nativeconfig.Kernel) error {
	idsCapacity, capacityErr := shared.CheckedCombinedCapacity(len(previous), len(desired))
	if capacityErr != nil {
		return fmt.Errorf("prepare managed Cline MCP server set: %w", capacityErr)
	}
	return mutateClineMCPWithKernelAndCapacity(configRoot, activePath, previous, desired, kernel, idsCapacity)
}

func mutateClineMCPWithKernelAndCapacity(configRoot, activePath string, previous, desired map[string]domain.NativeObjectOwnership, kernel nativeconfig.Kernel, idsCapacity int) error {
	if !hasClineMCP(previous) && !hasClineMCP(desired) {
		return nil
	}
	path := clineMCPSettingsPath(configRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	projection := clineProjection{Servers: map[string]nativeconfig.Server{}}
	if activePath != "" {
		var err error
		projection, err = readClineProjection(activePath)
		if err != nil {
			return err
		}
	}
	ids := make([]string, 0, idsCapacity)
	seen := map[string]bool{}
	for id, object := range previous {
		if object.Kind == clineMCPObjectKind {
			ids, seen[id] = append(ids, id), true
		}
	}
	for id, object := range desired {
		if object.Kind == clineMCPObjectKind && !seen[id] {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	requests := make([]nativeconfig.Request, 0, len(ids))
	for _, id := range ids {
		prior, hadPrior := previous[id]
		next, hasNext := desired[id]
		req := nativeconfig.Request{Paths: nativeconfig.Paths{JSON: path}, Codec: nativeconfig.CodecCline}
		var present, exactlyOwned bool
		if hadPrior {
			receipt := clineReceipt(prior)
			var err error
			present, exactlyOwned, err = kernel.Inspect(req.Paths, req.Codec, prior.LogicalName, &receipt)
			if err != nil {
				return fmt.Errorf("inspect managed Cline MCP server %q: %w", prior.LogicalName, err)
			}
			if present && !exactlyOwned {
				return fmt.Errorf("managed Cline MCP server %q changed outside agentplugins: %w", prior.LogicalName, nativeconfig.ErrNotOwned)
			}
		} else if hasNext {
			var err error
			present, _, err = kernel.Inspect(req.Paths, req.Codec, next.LogicalName, nil)
			if err != nil {
				return fmt.Errorf("inspect Cline MCP server %q before add: %w", next.LogicalName, err)
			}
			if present {
				return fmt.Errorf("Cline MCP server %q already exists: %w", next.LogicalName, nativeconfig.ErrCollision)
			}
		}
		switch {
		case hadPrior && hasNext:
			if !present {
				if !sameClineMCPObject(prior, next) {
					return fmt.Errorf("managed Cline MCP server %q is absent during update: %w", prior.LogicalName, nativeconfig.ErrNotOwned)
				}
				req.Action, req.Name = nativeconfig.ActionAdd, next.LogicalName
			} else {
				receipt := clineReceipt(prior)
				req.Action, req.Name, req.Owned = nativeconfig.ActionUpdate, next.LogicalName, &receipt
			}
			req.Server = projection.Servers[next.LogicalName]
		case hadPrior:
			if !present {
				continue
			}
			receipt := clineReceipt(prior)
			req.Action, req.Name, req.Owned = nativeconfig.ActionRemove, prior.LogicalName, &receipt
		case hasNext:
			req.Action, req.Name, req.Server = nativeconfig.ActionAdd, next.LogicalName, projection.Servers[next.LogicalName]
		}
		if hasNext {
			receipt, err := nativeconfig.DesiredReceipt(path, nativeconfig.CodecCline, next.LogicalName, req.Server, req.Placeholders)
			if err != nil {
				return err
			}
			if receipt.Digest != next.ManagedDigest {
				return fmt.Errorf("Cline MCP desired receipt mismatch for %q", next.LogicalName)
			}
			req.Desired = &receipt
		}
		requests = append(requests, req)
	}
	_, err := kernel.ApplyBatch(requests)
	if err != nil {
		return err
	}
	return nil
}
