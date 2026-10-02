package opencode

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

func crossOpenCodeDialect(previous, desired []domain.NativeObjectOwnership) bool {
	old, next := map[nativeconfig.Codec]bool{}, map[nativeconfig.Codec]bool{}
	for _, side := range []struct {
		objects []domain.NativeObjectOwnership
		codecs  map[nativeconfig.Codec]bool
	}{{previous, old}, {desired, next}} {
		for _, object := range side.objects {
			codec, mcp, _ := nativeconfig.OpenCodeCodecForKind(object.Kind)
			if mcp {
				side.codecs[codec] = true
			}
		}
	}
	for codec := range old {
		for target := range next {
			if codec != target {
				return true
			}
		}
	}
	return false
}

func openCodeTransitionRequest(p *openCodeNativeApply) (nativeconfig.TransitionRequest, error) {
	req := nativeconfig.TransitionRequest{Paths: openCodeMCPPaths(p.projection, p.previous)}
	desired := shared.ObjectMap(p.desired)
	for _, object := range p.previous {
		source, mcp, err := nativeconfig.OpenCodeCodecForKind(object.Kind)
		if err != nil {
			return req, err
		}
		if !mcp {
			continue
		}
		next, ok := desired[object.ObjectID]
		if !ok {
			return req, nativeconfig.ErrNativeMigrationRequired
		}
		target, mcp, err := nativeconfig.OpenCodeCodecForKind(next.Kind)
		if err != nil || !mcp || source == target || object.LogicalName != next.LogicalName || object.Path != next.Path {
			return req, nativeconfig.ErrNativeMigrationRequired
		}
		if req.SourceCodec != "" && (req.SourceCodec != source || req.TargetCodec != target) {
			return req, nativeconfig.ErrNativeMigrationRequired
		}
		req.SourceCodec, req.TargetCodec = source, target
		old, err := receiptFromOpenCodeObject(object)
		if err != nil {
			return req, err
		}
		receipt, err := receiptFromOpenCodeObject(next)
		if err != nil {
			return req, err
		}
		server, ok := p.projection.MCPServers[next.LogicalName]
		if !ok {
			return req, fmt.Errorf("transition lacks desired server")
		}
		req.Entries = append(req.Entries, nativeconfig.TransitionEntry{LogicalID: object.ObjectID, Name: object.LogicalName, SourceOwned: old, TargetServer: server, TargetPlaceholders: nativeconfig.Placeholders{PackageRoot: p.projection.PackageRoot, DataRoot: p.projection.DataRoot}, DesiredReceipt: receipt})
	}
	count := 0
	for _, object := range p.desired {
		_, mcp, err := nativeconfig.OpenCodeCodecForKind(object.Kind)
		if err != nil {
			return req, err
		}
		if mcp {
			count++
		}
	}
	if count == 0 || count != len(req.Entries) {
		return req, nativeconfig.ErrNativeMigrationRequired
	}
	codec, err := projectionCodec(p.projection)
	if err != nil || codec != req.TargetCodec {
		return req, nativeconfig.ErrNativeMigrationRequired
	}
	sort.Slice(req.Entries, func(i, j int) bool { return req.Entries[i].Name < req.Entries[j].Name })
	return req, nil
}

func transitionDTO(p nativeconfig.PreparedTransition) domain.OpenCodeTransitionPrepared {
	receipt := func(r nativeconfig.Receipt) domain.OpenCodeTransitionReceipt {
		return domain.OpenCodeTransitionReceipt{Version: r.Version, Path: r.Path, Codec: string(r.Codec), Name: r.Name, Digest: r.Digest}
	}
	d := domain.OpenCodeTransitionPrepared{Path: p.Path, OriginalBytes: p.Original.Body, OriginalMode: uint32(p.Original.Mode), OriginalExists: p.Original.Exists, TargetBytes: p.TargetBytes, TargetHash: p.TargetHash, SourceCodec: string(p.SourceCodec), TargetCodec: string(p.TargetCodec)}
	for _, e := range p.Entries {
		v := domain.OpenCodeTransitionEntry{LogicalID: e.LogicalID, Name: e.Name, SourceReceipt: receipt(e.SourceReceipt), TargetReceipt: receipt(e.TargetReceipt)}
		if e.PreviouslyOwnedTarget != nil {
			r := receipt(*e.PreviouslyOwnedTarget)
			v.PreviouslyOwnedTarget = &r
		}
		d.Entries = append(d.Entries, v)
	}
	return d
}

func applyOpenCodeTransition(ctx context.Context, request domain.ActivationRequest, kernel nativeconfig.Kernel, recorder ports.OpenCodeTransitionRecorder) error {
	unchanged := func(err error) error {
		return &shared.NativeEffectError{Effect: domain.NativeEffectUnchanged, Err: err}
	}
	if recorder == nil || request.NativeAttempt.OperationID == "" {
		return unchanged(fmt.Errorf("durable native transition authority is required"))
	}
	p, err := prepareOpenCodeNativeApply(request.Client.ConfigRoot, request.Delivery.ActivePath, request.PreviousNativeObjects, request.Delivery.NativeObjects, kernel, shared.RenameDirectoryExclusive, os.RemoveAll)
	if err != nil {
		return unchanged(err)
	}
	req, err := openCodeTransitionRequest(p)
	if err != nil {
		return unchanged(err)
	}
	txn := &openCodeSkillTxn{durable: true, backups: map[string]openCodeBackup{}, installed: map[string]domain.NativeObjectOwnership{}, rename: p.rename, removeAll: p.removeAll}
	if err := txn.createRoot(request.Client.ConfigRoot); err != nil {
		return unchanged(err)
	}
	staged, err := txn.stageDesired(request.Delivery.ActivePath, shared.ObjectMap(p.desired))
	if err != nil {
		return unchanged(err)
	}
	// Private staging must be durable before its path can be journal authority.
	if err := syncTransitionTree(txn.root); err != nil {
		return unchanged(err)
	}
	persisted := false
	req.PersistPrepared = func(prepared nativeconfig.PreparedTransition) error {
		// Set the fence before persistence: a parent-fsync failure may leave the
		// complete record visible. Neither it nor staged/backup data may be guessed away.
		persisted = true
		if err := recorder.PersistPrepared(ctx, request.NativeAttempt, txn.root, transitionDTO(prepared), request.PreviousNativeObjects, request.Delivery.NativeObjects); err != nil {
			return err
		}
		if err := txn.backupPrevious(shared.ObjectMap(p.previous)); err != nil {
			return err
		}
		if err := atomicfile.SyncDirectory(filepath.Join(request.Client.ConfigRoot, "skills")); err != nil {
			return err
		}
		if err := atomicfile.SyncDirectory(txn.root); err != nil {
			return err
		}
		if err := txn.installStaged(staged, shared.ObjectMap(p.desired)); err != nil {
			return err
		}
		if err := atomicfile.SyncDirectory(filepath.Join(request.Client.ConfigRoot, "skills")); err != nil {
			return err
		}
		return atomicfile.SyncDirectory(txn.root)
	}
	receipts, applyErr := kernel.ApplyDialectTransition(req)
	if !persisted {
		_ = os.RemoveAll(txn.root)
		return unchanged(applyErr)
	}
	if applyErr == nil || nativeconfig.IsCommittedCleanup(applyErr) {
		if len(receipts) != len(req.Entries) {
			return fmt.Errorf("native transition committed without complete receipts; recovery required")
		}
		for i, r := range receipts {
			if r != req.Entries[i].DesiredReceipt {
				return fmt.Errorf("native transition receipt differs from staged ownership; recovery required")
			}
		}
	}
	// Reconcile uncertain writes using persisted facts, never inverse Apply or
	// in-memory rollback. Reopening obtains the same candidate lease and CAS.
	effect, recoveryErr := recorder.Reconcile(ctx, txn.root)
	if effect == domain.NativeEffectCommitted {
		if len(receipts) > 0 && len(receipts) != len(req.Entries) {
			return fmt.Errorf("transition returned incomplete receipts")
		}
		if applyErr != nil && !nativeconfig.IsCommittedCleanup(applyErr) {
			return &nativeconfig.CommittedCleanupError{Err: errors.Join(applyErr, recoveryErr)}
		}
		return errors.Join(applyErr, recoveryErr)
	}
	// Do not expose a committed-cleanup tag when state visibility is unknown.
	if applyErr != nil {
		applyErr = fmt.Errorf("native transition requires reconciliation: %v", applyErr)
	}
	err = errors.Join(applyErr, recoveryErr)
	if err == nil {
		err = fmt.Errorf("native transition restored source")
	}
	return &shared.NativeEffectError{Effect: effect, Err: err}
}

func syncTransitionTree(root string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("transition staging contains symlink")
		}
		if entry.IsDir() {
			return atomicfile.SyncDirectory(path)
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		err = file.Sync()
		return errors.Join(err, file.Close())
	})
}
