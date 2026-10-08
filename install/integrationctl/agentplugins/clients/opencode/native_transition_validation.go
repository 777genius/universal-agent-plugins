package opencode

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Validation phases retain their order: envelope and native preparation precede
// bound state, then receipt ownership, skill intent, and projection authority.
func validateTransitionRecord(record transitionRecord, root string) error {
	if err := validateTransitionEnvelope(record, root); err != nil {
		return err
	}
	if err := nativeconfig.ValidatePreparedTransition(transitionPaths(record.Identity.NativeRoot), nativePrepared(record.Native)); err != nil {
		return err
	}
	old, target, err := validateTransitionBindings(record)
	if err != nil {
		return err
	}
	if err := validateTransitionStateChange(record, target); err != nil {
		return err
	}
	if err := validateTransitionReceipts(record, old, target); err != nil {
		return err
	}
	if err := validateTransitionSkills(record, old, target); err != nil {
		return err
	}
	if !validTransitionHash(record.ProjectionHash) {
		return fmt.Errorf("invalid projection hash")
	}
	return nil
}

func validateTransitionEnvelope(record transitionRecord, root string) error {
	if record.Version != 1 || record.Type != "opencode_native_transition" || record.Root != root || len(record.Skills) > maxTransitionSkills {
		return fmt.Errorf("invalid transition record schema")
	}
	switch record.Phase {
	case "prepared", "native_committed", "state_committed", "cleanup_pending":
	default:
		return fmt.Errorf("invalid transition phase")
	}
	for _, id := range []string{record.Identity.OperationID, record.Identity.InstallationID, record.Identity.BindingID} {
		if err := pathpolicy.ValidateLeafID(id); err != nil {
			return err
		}
	}
	return validateTransitionRoot(record.Identity.NativeRoot, root)
}

func validateTransitionRoot(nativeRoot, root string) error {
	if !filepath.IsAbs(nativeRoot) || filepath.Clean(nativeRoot) != nativeRoot || filepath.Dir(root) != filepath.Join(nativeRoot, "skills") || !strings.HasPrefix(filepath.Base(root), ".agentplugins-native-") {
		return fmt.Errorf("invalid transition root")
	}
	if err := pathpolicy.RequireContainedChild(nativeRoot, root); err != nil {
		return err
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("transition root must be private")
	}
	return validateTransitionPrivacy(root, info, true)
}

func validateTransitionBindings(record transitionRecord) (domain.ClientBinding, domain.ClientBinding, error) {
	_, old, err := transitionBinding(record.OldState, record.Identity)
	if err != nil {
		return old, domain.ClientBinding{}, err
	}
	_, target, err := transitionBinding(record.TargetState, record.Identity)
	if err != nil {
		return old, target, err
	}
	if old.NativeActivationAttempt != record.Identity.OperationID || target.NativeActivationAttempt != "" || old.PackageRevision == nil || old.TargetLocator == "" || !filepath.IsAbs(old.TargetLocator) {
		return old, target, fmt.Errorf("invalid bound attempt/package")
	}
	return old, target, validateTransitionPackage(old, target)
}

func transitionPackageObjects(objects []domain.NativeObjectOwnership) ([]domain.NativeObjectOwnership, error) {
	packages := []domain.NativeObjectOwnership{}
	for _, object := range objects {
		if object.Kind == "managed_package_directory" {
			packages = append(packages, object)
			continue
		}
		_, mcp, err := nativeconfig.OpenCodeCodecForKind(object.Kind)
		if err != nil || !mcp && object.Kind != openCodeSkillKind {
			return nil, fmt.Errorf("transition contains unbound native objects")
		}
	}
	return packages, nil
}

func validateTransitionPackage(old, target domain.ClientBinding) error {
	oldPackage, err := transitionPackageObjects(old.NativeObjects)
	if err != nil {
		return err
	}
	targetPackage, err := transitionPackageObjects(target.NativeObjects)
	if err != nil {
		return err
	}
	if len(oldPackage) != 1 || oldPackage[0].Path != old.TargetLocator || !validTransitionHash(oldPackage[0].ManagedDigest) {
		return fmt.Errorf("transition has no exact directory-committed package")
	}
	if !reflect.DeepEqual(oldPackage, targetPackage) {
		return fmt.Errorf("transition changes directory-committed package ownership")
	}
	return nil
}

func validateTransitionStateChange(record transitionRecord, target domain.ClientBinding) error {
	expected := transitionTargetState(record.OldState, record.Identity, target.NativeObjects)
	if !bytes.Equal(comparableTransitionState(expected), comparableTransitionState(record.TargetState)) {
		return fmt.Errorf("transition changes unbound state")
	}
	return nil
}

type transitionReceiptSide struct {
	objects []domain.NativeObjectOwnership
	codec   string
	source  bool
}

func validateTransitionReceipts(record transitionRecord, old, target domain.ClientBinding) error {
	for _, side := range []transitionReceiptSide{{old.NativeObjects, record.Native.SourceCodec, true}, {target.NativeObjects, record.Native.TargetCodec, false}} {
		if err := validateTransitionReceiptSide(record, side); err != nil {
			return err
		}
	}
	return nil
}

func validateTransitionReceiptSide(record transitionRecord, side transitionReceiptSide) error {
	objects := shared.ObjectMap(OpenCodeObjects(side.objects))
	mcpCount := 0
	for _, object := range objects {
		codec, mcp, err := nativeconfig.OpenCodeCodecForKind(object.Kind)
		if err != nil {
			return err
		}
		if err := validateOpenCodeObject(record.Identity.NativeRoot, OpenCodeProjection{}, object); err != nil {
			return err
		}
		if mcp {
			if string(codec) != side.codec {
				return fmt.Errorf("mixed stored codecs")
			}
			mcpCount++
		}
	}
	if mcpCount != len(record.Native.Entries) {
		return fmt.Errorf("incomplete transition ownership")
	}
	return validateTransitionReceiptEntries(record.Native.Entries, objects, side.source)
}

func validateTransitionReceiptEntries(entries []domain.OpenCodeTransitionEntry, objects map[string]domain.NativeObjectOwnership, source bool) error {
	for _, entry := range entries {
		object, ok := objects[entry.LogicalID]
		receipt := entry.TargetReceipt
		if source {
			receipt = entry.SourceReceipt
		}
		if !ok || object.LogicalName != entry.Name || object.Path != receipt.Path || object.ManagedDigest != receipt.Digest {
			return fmt.Errorf("transition receipt/state mismatch")
		}
	}
	return nil
}

func validateTransitionSkills(record transitionRecord, old, target domain.ClientBinding) error {
	oldObjects := shared.ObjectMap(OpenCodeObjects(old.NativeObjects))
	nextObjects := shared.ObjectMap(OpenCodeObjects(target.NativeObjects))
	if transitionSkillIntentCount(oldObjects, nextObjects) != len(record.Skills) {
		return fmt.Errorf("incomplete skill intent")
	}
	for i, skill := range record.Skills {
		if err := validateTransitionSkillIntent(record, i, skill); err != nil {
			return err
		}
		if err := validateTransitionSkillOwnership(skill, oldObjects, nextObjects); err != nil {
			return err
		}
		if err := validateTransitionSkillPaths(record, skill); err != nil {
			return err
		}
	}
	return nil
}

func transitionSkillIntentCount(old, next map[string]domain.NativeObjectOwnership) int {
	count := 0
	for _, object := range old {
		if object.Kind == openCodeSkillKind {
			count++
		}
	}
	for id, object := range next {
		if object.Kind == openCodeSkillKind {
			if _, ok := old[id]; !ok {
				count++
			}
		}
	}
	return count
}

func validateTransitionSkillIntent(record transitionRecord, index int, skill transitionSkill) error {
	if err := pathpolicy.ValidateLeafID(skill.Name); err != nil {
		return err
	}
	if skill.ID != "opencode-skill:"+skill.Name || index > 0 && record.Skills[index-1].ID >= skill.ID || skill.Target != filepath.Join(record.Identity.NativeRoot, "skills", skill.Name) || skill.Backup != filepath.Join(record.Root, "old-"+skill.Name) || skill.Old == nil && skill.New == nil || skill.Old == nil && skill.OldExists {
		return fmt.Errorf("invalid skill intent")
	}
	return nil
}

func validateTransitionSkillOwnership(skill transitionSkill, oldObjects, nextObjects map[string]domain.NativeObjectOwnership) error {
	old, oldOK := oldObjects[skill.ID]
	next, nextOK := nextObjects[skill.ID]
	if oldOK != (skill.Old != nil) || nextOK != (skill.New != nil) || oldOK && !reflect.DeepEqual(old, *skill.Old) || nextOK && !reflect.DeepEqual(next, *skill.New) {
		return fmt.Errorf("skill intent/state mismatch")
	}
	return nil
}

func validateTransitionSkillPaths(record transitionRecord, skill transitionSkill) error {
	if skill.New != nil && skill.Staged != filepath.Join(record.Root, "new-"+skill.Name) || skill.New == nil && skill.Staged != "" {
		return fmt.Errorf("invalid staged skill path")
	}
	for _, path := range []string{skill.Target, skill.Backup, skill.Staged} {
		if path != "" {
			if err := pathpolicy.RequireContainedChild(record.Identity.NativeRoot, path); err != nil {
				return err
			}
		}
	}
	return nil
}
