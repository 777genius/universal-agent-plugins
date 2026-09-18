package opencode

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func ActivateNative(ctx context.Context, env clients.Env, request domain.ActivationRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return applyOpenCodeNativeMutation(env, clients.Ops{}, request.Client.ConfigRoot, request.Delivery.ActivePath, request.PreviousNativeObjects, request.Delivery.NativeObjects)
}

func DeactivateNative(ctx context.Context, env clients.Env, request domain.DeactivationRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return applyOpenCodeNativeMutation(env, clients.Ops{}, request.Client.ConfigRoot, "", request.NativeObjects, nil)
}

func applyOpenCodeNative(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership) error {
	return applyOpenCodeNativeMutation(clients.Env{NativeConfig: nativeconfig.New()}, clients.Ops{}, configRoot, activePath, previous, desired)
}

func applyOpenCodeNativeMutation(env clients.Env, ops clients.Ops, configRoot, activePath string, previous, desired []domain.NativeObjectOwnership) (resultErr error) {
	prepared, err := prepareOpenCodeNativeApply(env, ops, configRoot, activePath, previous, desired)
	if err != nil {
		return err
	}
	requests, err := openCodeMCPRequests(prepared.projection, previous, desired)
	if err != nil {
		return err
	}
	skills, err := installOpenCodeSkillsWithOps(configRoot, activePath, previous, desired, prepared.rename, prepared.removeAll)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			if err := skills.rollback(); err != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("rollback OpenCode skills: %w", err))
			}
		}
	}()
	return commitOpenCodeNative(prepared.kernel, requests, skills, desired, &committed)
}

type openCodeNativeApply struct {
	rename     openCodeRenameFunc
	removeAll  func(string) error
	kernel     nativeconfig.Kernel
	projection openCodeProjection
}

func prepareOpenCodeNativeApply(env clients.Env, ops clients.Ops, configRoot, activePath string, previous, desired []domain.NativeObjectOwnership) (*openCodeNativeApply, error) {
	rename := ops.Rename
	if rename == nil {
		rename = shared.RenameDirectoryExclusive
	}
	removeAll := ops.RemoveAll
	if removeAll == nil {
		removeAll = os.RemoveAll
	}
	kernel := env.NativeConfig
	if rename == nil {
		return nil, fmt.Errorf("the OpenCode rename operation is unavailable")
	}
	if removeAll == nil {
		return nil, fmt.Errorf("the OpenCode cleanup operation is unavailable")
	}
	if strings.TrimSpace(configRoot) == "" || !filepath.IsAbs(configRoot) {
		return nil, fmt.Errorf("the OpenCode config root is unavailable")
	}
	previous, desired = NativeObjects(previous), NativeObjects(desired)
	projection := openCodeProjection{}
	var err error
	if len(desired) > 0 {
		projection, err = readOpenCodeProjection(activePath)
		if err != nil {
			return nil, err
		}
		if err := validateOpenCodeProjection(configRoot, activePath, projection); err != nil {
			return nil, err
		}
	}
	if err := preflightOpenCodeObjects(configRoot, activePath, projection, previous, desired); err != nil {
		return nil, err
	}
	if err := confirmOpenCodeConfigSelection(configRoot, projection, previous, desired); err != nil {
		return nil, err
	}
	return &openCodeNativeApply{rename: rename, removeAll: removeAll, kernel: kernel, projection: projection}, nil
}

func confirmOpenCodeConfigSelection(configRoot string, projection openCodeProjection, previous, desired []domain.NativeObjectOwnership) error {
	expectedConfig := projection.ConfigPath
	if expectedConfig == "" {
		for _, object := range previous {
			if object.Kind == openCodeMCPObjectKind {
				expectedConfig = object.Path
				break
			}
		}
	}
	if expectedConfig == "" {
		return nil
	}
	selected, err := selectOpenCodeConfig(filepath.Join(configRoot, "opencode.json"), filepath.Join(configRoot, "opencode.jsonc"))
	if err != nil {
		return err
	}
	jsonExists, jsoncExists, err := openCodeConfigPresence(configRoot)
	if err != nil {
		return err
	}
	if !shared.SameCleanPath(selected, expectedConfig) {
		if len(desired) > 0 || jsonExists || jsoncExists {
			return fmt.Errorf("the OpenCode config selection changed after staging; rerun the operation")
		}
	}
	return nil
}

func commitOpenCodeNative(kernel nativeconfig.Kernel, requests []nativeconfig.Request, skills *openCodeSkillTxn, desired []domain.NativeObjectOwnership, committed *bool) error {
	if len(requests) == 0 {
		*committed = true
		skills.commit()
		return nil
	}
	receipts, applyErr := kernel.ApplyBatch(requests)
	if applyErr != nil && !nativeconfig.IsCommittedCleanup(applyErr) {
		return applyErr
	}
	if len(receipts) != len(requests) {
		if applyErr != nil {
			return errors.Join(applyErr, fmt.Errorf("the OpenCode native config committed without complete receipts"))
		}
		return fmt.Errorf("the OpenCode native config returned incomplete receipts")
	}
	*committed = true
	desiredByID := shared.ObjectMap(desired)
	for index, request := range requests {
		if request.Action == nativeconfig.ActionRemove {
			continue
		}
		expected := desiredByID["opencode-mcp:"+request.Name]
		if receipts[index].Digest != expected.ManagedDigest || receipts[index].Path != expected.Path {
			return fmt.Errorf("the OpenCode native receipt differs from staged ownership")
		}
	}
	skills.commit()
	return applyErr
}
