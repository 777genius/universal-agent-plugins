package opencode

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type PendingNativeTransition struct{ OperationID, InstallationID, BindingID, TargetPath, Phase, Digest, Root string }

func (t NativeTransitions) Pending() ([]PendingNativeTransition, error) {
	state, err := t.State.Load()
	if err != nil {
		return nil, err
	}
	roots := map[string]bool{}
	attempts := map[string]PendingNativeTransition{}
	for _, installation := range state.Installations {
		for _, b := range installation.Clients {
			if b.ClientID != "opencode" {
				continue
			}
			if root := storedTransitionRoot(b); root != "" {
				roots[root] = true
			}
			if b.NativeActivationAttempt != "" {
				attempts[b.NativeActivationAttempt] = PendingNativeTransition{OperationID: b.NativeActivationAttempt, InstallationID: installation.InstallationID, BindingID: b.ClientBindingID, TargetPath: b.TargetLocator, Phase: "native_attempt"}
			}
		}
	}
	var pending []PendingNativeTransition
	for root := range roots {
		skills := filepath.Join(root, "skills")
		if err := pathpolicy.RequireContainedChild(root, skills); err != nil {
			return nil, err
		}
		entries, err := os.ReadDir(skills)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if !strings.HasPrefix(entry.Name(), ".agentplugins-native-") {
				continue
			}
			dir := filepath.Join(skills, entry.Name())
			if err := pathpolicy.RequireContainedChild(root, dir); err != nil {
				return nil, err
			}
			if _, err := os.Lstat(filepath.Join(dir, transitionRecordFile)); os.IsNotExist(err) {
				continue
			} else if err != nil {
				return nil, err
			}
			r, err := readTransitionRecord(dir)
			if err != nil {
				return nil, err
			}
			if r.Identity.NativeRoot != root {
				return nil, fmt.Errorf("transition root identity mismatch")
			}
			_, b, err := transitionBinding(r.OldState, r.Identity)
			if err != nil {
				return nil, err
			}
			pending = append(pending, PendingNativeTransition{OperationID: r.Identity.OperationID, InstallationID: r.Identity.InstallationID, BindingID: r.Identity.BindingID, TargetPath: b.TargetLocator, Phase: r.Phase, Digest: r.Hash, Root: dir})
			delete(attempts, r.Identity.OperationID)
			if len(pending) > nativeconfig.MaxTransitionEntries {
				return nil, fmt.Errorf("too many native transition records")
			}
		}
	}
	for _, attempt := range attempts {
		pending = append(pending, attempt)
	}
	sort.Slice(pending, func(i, j int) bool { return pending[i].OperationID < pending[j].OperationID })
	for i := 1; i < len(pending); i++ {
		if pending[i].OperationID == pending[i-1].OperationID {
			return nil, fmt.Errorf("duplicate native transition identity")
		}
	}
	return pending, nil
}

func (t NativeTransitions) Recover(ctx context.Context) error {
	pending, err := t.Pending()
	if err != nil {
		return err
	}
	for _, p := range pending {
		if err := ctx.Err(); err != nil {
			return err
		}
		if p.Root == "" {
			return fmt.Errorf("native attempt has no durable transition record; recovery required")
		}
		if _, err := t.Reconcile(ctx, p.Root); err != nil {
			return err
		}
	}
	return nil
}

func (t NativeTransitions) Reconcile(ctx context.Context, root string) (domain.NativeEffectState, error) {
	r, err := readTransitionRecord(root)
	if err != nil {
		return domain.NativeEffectUncertain, err
	}
	effect := domain.NativeEffectUncertain
	observation, reconcileErr := t.Kernel.ReconcileDialectTransition(nativeconfig.TransitionRecoveryRequest{
		Paths: transitionPaths(r.Identity.NativeRoot), Prepared: nativePrepared(r.Native),
		Decide: func(observation nativeconfig.TransitionObservation) (nativeconfig.TransitionStateDecision, error) {
			if err := ctx.Err(); err != nil {
				return nativeconfig.TransitionStateUnknown, err
			}
			current, err := t.State.Load()
			if err != nil {
				return nativeconfig.TransitionStateUnknown, err
			}
			source := transitionStateMatches(current, r.OldState, r.Identity) || transitionStateMatches(current, sourceTransitionState(r), r.Identity)
			target := transitionStateMatches(current, r.TargetState, r.Identity)
			if !source && !target {
				return nativeconfig.TransitionStateUnknown, fmt.Errorf("transition state differs from bound source and target")
			}
			_, binding, _ := transitionBinding(r.TargetState, r.Identity)
			if t.PackageVerifier == nil {
				return nativeconfig.TransitionStateUnknown, fmt.Errorf("native package verifier missing")
			}
			if err := t.PackageVerifier.Verify(ctx, binding.TargetLocator, shared.ManagedPackageDigest(binding)); err != nil {
				return nativeconfig.TransitionStateUnknown, err
			}
			if observation == nativeconfig.TransitionSource {
				if !source {
					return nativeconfig.TransitionStateUnknown, fmt.Errorf("source bytes with target state")
				}
				return nativeconfig.TransitionStateOld, nil
			}
			// Prove projection and all desired skill effects before publishing receipts.

			projection, err := readTransitionFile(binding.TargetLocator, filepath.Join(binding.TargetLocator, OpenCodeProjectionFile), nativeconfig.MaxTransitionConfigBytes)
			if err != nil || transitionHash(projection) != r.ProjectionHash {
				return nativeconfig.TransitionStateUnknown, fmt.Errorf("transition package projection changed")
			}
			if err := verifyTransitionTargetSkills(r); err != nil {
				return nativeconfig.TransitionStateUnknown, err
			}
			r.Phase = "native_committed"
			if err := writeTransitionRecord(r); err != nil {
				return nativeconfig.TransitionStateUnknown, err
			}
			desired, err := transitionPublicationState(current, r.TargetState, r.Identity)
			if err != nil {
				return nativeconfig.TransitionStateUnknown, err
			}
			disposition, err := t.State.PersistStateDecisionWithDisposition(current, desired)
			switch disposition {
			case domain.StateDecisionDesired:
				effect = domain.NativeEffectCommitted
				return nativeconfig.TransitionStateDesired, err
			case domain.StateDecisionOld:
				// Old only means the exact caller preimage remains. If that preimage was
				// already target state, it grants no source restore authority.
				if target {
					return nativeconfig.TransitionStateUnknown, err
				}
				return nativeconfig.TransitionStateOld, err
			default:
				return nativeconfig.TransitionStateUnknown, err
			}
		},
		Complete: func(observation nativeconfig.TransitionObservation) error {
			if observation == nativeconfig.TransitionSource {
				if err := restoreTransitionSkills(r); err != nil {
					return err
				}
				current, err := t.State.Load()
				if err != nil {
					return err
				}
				if !transitionStateMatches(current, r.OldState, r.Identity) && !transitionStateMatches(current, sourceTransitionState(r), r.Identity) {
					return fmt.Errorf("source state changed before skill recovery")
				}
				desired, err := transitionPublicationState(current, sourceTransitionState(r), r.Identity)
				if err != nil {
					return err
				}
				disposition, err := t.State.PersistStateDecisionWithDisposition(current, desired)
				if err != nil || disposition != domain.StateDecisionDesired {
					return errors.Join(err, fmt.Errorf("source outcome durability unresolved"))
				}
				effect = domain.NativeEffectUnchanged
			} else {
				r.Phase = "state_committed"
				if err := writeTransitionRecord(r); err != nil {
					return err
				}
			}
			r.Phase = "cleanup_pending"
			if err := writeTransitionRecord(r); err != nil {
				return err
			}
			return cleanupTransition(r)
		},
	})
	if observation == nativeconfig.TransitionTarget && effect == domain.NativeEffectCommitted && reconcileErr != nil && !nativeconfig.IsCommittedCleanup(reconcileErr) {
		reconcileErr = &nativeconfig.CommittedCleanupError{Err: reconcileErr}
	}
	return effect, reconcileErr
}

func transitionSkillDigest(root, path string) (string, bool, error) {
	if err := pathpolicy.RequireContainedChild(root, path); err != nil {
		return "", false, err
	}
	digest, err := shared.DigestSkillDirectory(path)
	if os.IsNotExist(err) {
		return "", false, nil
	}
	return digest, err == nil, err
}
func verifyTransitionTargetSkills(r transitionRecord) error {
	for _, s := range r.Skills {
		if s.Staged != "" {
			_, exists, err := transitionSkillDigest(r.Root, s.Staged)
			if err != nil {
				return err
			}
			if exists {
				return fmt.Errorf("target skill has no completed exclusive staging move")
			}
		}
		digest, exists, err := transitionSkillDigest(r.Identity.NativeRoot, s.Target)
		if err != nil {
			return err
		}
		if s.New == nil {
			if exists {
				return fmt.Errorf("removed transition skill is occupied")
			}
		} else if !exists || digest != s.New.ManagedDigest {
			return fmt.Errorf("target transition skill differs from receipt")
		}
	}
	return nil
}

func restoreTransitionSkills(r transitionRecord) error {
	for _, s := range r.Skills {
		current, exists, err := transitionSkillDigest(r.Identity.NativeRoot, s.Target)
		if err != nil {
			return err
		}
		backup, backupExists, err := transitionSkillDigest(r.Root, s.Backup)
		if err != nil {
			return err
		}
		if backupExists && (s.Old == nil || !s.OldExists || backup != s.Old.ManagedDigest) {
			return fmt.Errorf("transition backup differs from source intent")
		}
		if s.OldExists && exists && current == s.Old.ManagedDigest && !backupExists {
			continue
		}
		if exists {
			if s.Staged != "" {
				_, stagedExists, err := transitionSkillDigest(r.Root, s.Staged)
				if err != nil {
					return err
				}
				if stagedExists {
					return fmt.Errorf("preserve equal foreign skill; staged move never completed")
				}
			}
			if s.New == nil || current != s.New.ManagedDigest || s.OldExists && !backupExists {
				return fmt.Errorf("preserve edited transition skill")
			}
			if err := os.RemoveAll(s.Target); err != nil {
				return err
			}
			if err := atomicfile.SyncDirectory(filepath.Dir(s.Target)); err != nil {
				return err
			}
		}
		if s.OldExists {
			if !backupExists {
				return fmt.Errorf("source skill backup missing")
			}
			if err := renameOpenCodeDirectoryNoReplace(s.Backup, s.Target, shared.RenameDirectoryExclusive); err != nil {
				return err
			}
			if err := atomicfile.SyncDirectory(filepath.Dir(s.Target)); err != nil {
				return err
			}
			if err := atomicfile.SyncDirectory(r.Root); err != nil {
				return err
			}
		} else if backupExists {
			return fmt.Errorf("unexpected source skill backup")
		}
	}
	return nil
}
func cleanupTransition(r transitionRecord) error {
	// All owned payloads are proved before destructive cleanup. Unknown residue
	// is retained; an edited backup is never deleted by a path-only RemoveAll.
	for _, s := range r.Skills {
		for _, item := range []struct {
			path   string
			object *domain.NativeObjectOwnership
		}{{s.Backup, s.Old}, {s.Staged, s.New}} {
			if item.path == "" {
				continue
			}
			digest, exists, err := transitionSkillDigest(r.Root, item.path)
			if err != nil {
				return err
			}
			if !exists {
				continue
			}
			if item.object == nil || digest != item.object.ManagedDigest {
				return fmt.Errorf("preserve changed transition residue")
			}
			if err := os.RemoveAll(item.path); err != nil {
				return err
			}
			if err := atomicfile.SyncDirectory(r.Root); err != nil {
				return err
			}
		}
	}
	entries, err := os.ReadDir(r.Root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() != transitionRecordFile {
			return fmt.Errorf("preserve unrecognized transition residue")
		}
	}
	if err := os.Remove(filepath.Join(r.Root, transitionRecordFile)); err != nil {
		return err
	}
	if err := atomicfile.SyncDirectory(r.Root); err != nil {
		return err
	}
	if err := os.Remove(r.Root); err != nil {
		return err
	}
	return atomicfile.SyncDirectory(filepath.Dir(r.Root))
}

func (t NativeTransitions) HoldsActivation(installationID, bindingID string) (bool, error) {
	pending, err := t.Pending()
	if err != nil {
		return false, err
	}
	for _, p := range pending {
		if p.Root != "" && p.InstallationID == installationID && p.BindingID == bindingID {
			return true, nil
		}
	}
	return false, nil
}
