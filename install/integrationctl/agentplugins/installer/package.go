package installer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// snapshotLocalPackage uses packagedigest executable overrides so Windows
// host FileMode (no 0111 on regular files) does not drop logical bin/ helpers
// from TreeDigest. AcquireLocal hashes POSIX bits from the checkout.
// LocalPackageTreeDigest is the canonical TreeDigest of a local package root.
// It snapshots into TempRoot, does not write installer state, and does not
// report Progress. Optional Assess still binds that digest.
func (e *Engine) LocalPackageTreeDigest(ctx context.Context, packageRoot string) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("%w: context is required", ErrInvalidRequest)
	}
	if packageRoot == "" || !validRoot(packageRoot) {
		return "", fmt.Errorf("%w: PackageRoot must be an explicit absolute clean path", ErrInvalidRequest)
	}
	if overlappingRoots(e.cfg.TempRoot, packageRoot) {
		return "", fmt.Errorf("%w: TempRoot must not overlap PackageRoot", ErrInvalidRequest)
	}
	if err := os.MkdirAll(e.cfg.TempRoot, 0700); err != nil {
		return "", err
	}
	snapshot, err := snapshotLocalPackage(ctx, e.cfg.TempRoot, packageRoot)
	if err != nil {
		return "", err
	}
	defer func() { _ = packagedigest.Remove(snapshot) }()
	if err := e.assessSnapshot(ctx, snapshot, nil); err != nil {
		return "", err
	}
	return snapshot.TreeDigest, nil
}

func snapshotLocalPackage(ctx context.Context, tempRoot, packageRoot string) (domain.PackageSnapshot, error) {
	absolute, err := filepath.Abs(packageRoot)
	if err != nil {
		return domain.PackageSnapshot{}, fmt.Errorf("acquire local package: resolve source failed")
	}
	executables, err := declaredPackageExecutables(absolute)
	if err != nil {
		return domain.PackageSnapshot{}, err
	}
	source := domain.SourceIdentity{RequestedSource: packageRoot, CanonicalSource: filepath.Clean(absolute), SourceBindingHint: "direct-local"}
	snapshot, err := (packagedigest.Builder{TempRoot: tempRoot}).SnapshotWithExecutables(ctx, absolute, source, executables)
	if err != nil {
		return domain.PackageSnapshot{}, fmt.Errorf("acquire local package: snapshot package content failed")
	}
	return snapshot, nil
}

func snapshotRequestPackage(ctx context.Context, tempRoot string, req Request) (domain.PackageSnapshot, error) {
	if req.ExecutableFiles == nil {
		return snapshotLocalPackage(ctx, tempRoot, req.PackageRoot)
	}
	for _, relative := range req.ExecutableFiles {
		if relative == "." || relative == ".." || path.IsAbs(relative) || path.Clean(relative) != relative || strings.HasPrefix(relative, "../") || strings.Contains(relative, `\`) || filepath.VolumeName(relative) != "" {
			return domain.PackageSnapshot{}, fmt.Errorf("%w: invalid snapshot executable path", ErrInvalidRequest)
		}
		info, err := os.Lstat(filepath.Join(req.PackageRoot, filepath.FromSlash(relative)))
		if err != nil || !info.Mode().IsRegular() {
			return domain.PackageSnapshot{}, fmt.Errorf("%w: snapshot executable is not a regular package file", ErrInvalidRequest)
		}
	}
	source := domain.SourceIdentity{RequestedSource: req.PackageRoot, CanonicalSource: req.PackageRoot, SourceBindingHint: "direct-local"}
	return (packagedigest.Builder{TempRoot: tempRoot}).SnapshotWithExecutables(ctx, req.PackageRoot, source, req.ExecutableFiles)
}

func declaredPackageExecutables(root string) ([]string, error) {
	seen := map[string]struct{}{}
	var out []string
	add := func(rel string) {
		rel = path.Clean(strings.TrimPrefix(filepath.ToSlash(rel), "./"))
		if rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
			return
		}
		info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil || !info.Mode().IsRegular() {
			return
		}
		if _, ok := seen[rel]; ok {
			return
		}
		seen[rel] = struct{}{}
		out = append(out, rel)
	}
	raw, err := os.ReadFile(filepath.Join(root, "mcp.json"))
	if err == nil {
		var mcp struct {
			Servers map[string]struct {
				Command string `json:"command"`
			} `json:"mcpServers"`
		}
		if json.Unmarshal(raw, &mcp) == nil {
			for _, server := range mcp.Servers {
				add(server.Command)
			}
		}
	}
	entries, err := os.ReadDir(filepath.Join(root, "bin"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		add(path.Join("bin", entry.Name()))
	}
	return out, nil
}

func (e *Engine) detectedClient(req Request) (domain.DetectedClient, error) {
	id := domain.ClientID(req.ClientID)
	if !e.SupportsClient(req.ClientID) {
		return domain.DetectedClient{}, fmt.Errorf("%w: client %q is not registered for this installer", ErrUnsupported, req.ClientID)
	}
	if req.ClientConfigRoot == "" || !validRoot(req.ClientConfigRoot) {
		return domain.DetectedClient{}, fmt.Errorf("%w: ClientConfigRoot must be an explicit absolute clean path", ErrInvalidRequest)
	}
	if req.Operation == OpInstall && (req.ClientExecutable == "" || !validRoot(req.ClientExecutable)) {
		return domain.DetectedClient{}, fmt.Errorf("%w: ClientExecutable must be an explicit absolute clean path", ErrInvalidRequest)
	}
	return domain.DetectedClient{ClientID: id, Status: domain.DetectionDetected, ConfigRoot: req.ClientConfigRoot, ExecutablePath: req.ClientExecutable}, nil
}

func missingRequired(envelope domain.PackageEnvelope, required []string) []string {
	var missing []string
	for _, item := range required {
		switch item {
		case "mcp":
			if len(envelope.MCP.Servers) == 0 {
				missing = append(missing, item)
			}
		case "skills":
			if len(envelope.Skills) == 0 {
				missing = append(missing, item)
			}
		}
	}
	return missing
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func findInstall(state domain.StateFileV2, id string) (domain.Installation, bool) {
	for _, item := range state.Installations {
		if item.InstallationID == id {
			return item, true
		}
	}
	return domain.Installation{}, false
}

func findBinding(installation domain.Installation, client domain.ClientID) (domain.ClientBinding, domain.DataReceipt, bool) {
	for _, binding := range installation.Clients {
		if binding.ClientID != string(client) {
			continue
		}
		return binding, installation.DataReceipts[binding.DataReceiptID], true
	}
	return domain.ClientBinding{}, domain.DataReceipt{}, false
}

func (e *Engine) assessSnapshot(ctx context.Context, snapshot domain.PackageSnapshot, supplied *Assessment) error {
	decisions := make([]Assessment, 0, 2)
	if e.cfg.Assess != nil {
		got, err := e.cfg.Assess(ctx, snapshot.Root, snapshot.TreeDigest)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrAssessmentRejected, err)
		}
		decisions = append(decisions, got)
	}
	if supplied != nil {
		decisions = append(decisions, *supplied)
	}
	if len(decisions) == 0 {
		if e.cfg.TrustedLocalPackages {
			return nil
		}
		return fmt.Errorf("%w: no evaluator, digest-bound host decision, or explicit trusted-local policy", ErrAssessmentRejected)
	}
	for _, got := range decisions {
		if got.TreeDigest != snapshot.TreeDigest {
			return fmt.Errorf("%w: assessment digest %s snapshot %s", ErrAssessmentRejected, got.TreeDigest, snapshot.TreeDigest)
		}
		if got.Outcome != AssessmentAllow {
			reason := got.Reason
			if reason == "" {
				reason = string(got.Outcome)
			}
			if reason == "" {
				reason = "unavailable"
			}
			return fmt.Errorf("%w: %s", ErrAssessmentRejected, reason)
		}
	}
	return nil
}

func (e *Engine) refuseRecordedDigestRewrite(installationID, desired string) error {
	if installationID == "" || desired == "" {
		return nil
	}
	state, err := e.store.Load()
	if err != nil {
		return fmt.Errorf("read installation state: %w", err)
	}
	installation, ok := findInstall(state, installationID)
	if !ok {
		return nil
	}
	recorded := installation.Source.TreeDigest
	if recorded != "" && recorded != desired {
		return fmt.Errorf("%w: recorded digest %s desired %s", ErrUpdateRequired, recorded, desired)
	}
	return nil
}

func (e *Engine) refuseRepairRevisionRewrite(installationID, clientID, desired string) error {
	if installationID == "" || clientID == "" || desired == "" {
		return nil
	}
	state, err := e.store.Load()
	if err != nil {
		return fmt.Errorf("read installation state: %w", err)
	}
	installation, ok := findInstall(state, installationID)
	if !ok {
		return nil
	}
	binding, _, ok := findBinding(installation, domain.ClientID(clientID))
	if !ok || binding.PackageRevision == nil || binding.PackageRevision.TreeDigest == "" {
		return nil
	}
	if binding.PackageRevision.TreeDigest != desired {
		return fmt.Errorf("%w: recorded digest %s desired %s", ErrUpdateRequired, binding.PackageRevision.TreeDigest, desired)
	}
	return nil
}

// reuseMatchingSourceIdentity keeps the recorded source binding when the new
// snapshot is the same logical source. Local CanonicalSource is a capture
// path, not revision identity: a same-bytes Add from a new directory must
// not become a source switch, and an authorized Update may change TreeDigest
// without rewriting SourceBindingID.
func (e *Engine) reuseMatchingSourceIdentity(installationID string, snapshot *domain.PackageSnapshot, allowDigestRewrite bool) {
	if installationID == "" || snapshot == nil || snapshot.TreeDigest == "" {
		return
	}
	state, err := e.store.Load()
	if err != nil {
		return
	}
	installation, ok := findInstall(state, installationID)
	if !ok || installation.Source.TreeDigest == "" {
		return
	}
	if !allowDigestRewrite && installation.Source.TreeDigest != snapshot.TreeDigest {
		return
	}
	if installation.Source.CanonicalSource != "" {
		snapshot.Source.CanonicalSource = installation.Source.CanonicalSource
	}
	if installation.Source.RequestedSource != "" {
		snapshot.Source.RequestedSource = installation.Source.RequestedSource
	}
	if installation.Source.Repository != "" {
		snapshot.Source.Repository = installation.Source.Repository
	}
	if installation.Source.PackageSubpath != "" {
		snapshot.Source.PackageSubpath = installation.Source.PackageSubpath
	}
	if installation.Source.ResolvedRevision != "" {
		snapshot.Source.ResolvedRevision = installation.Source.ResolvedRevision
	}
}

func wrapLifecycleError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if strings.Contains(msg, "run update separately") ||
		strings.Contains(msg, "at a different revision; use update") ||
		strings.Contains(msg, "differs from the installed revision; use update") ||
		strings.Contains(msg, "use switch to change source") {
		return fmt.Errorf("%w: %w", ErrUpdateRequired, err)
	}
	if strings.Contains(msg, "is not bound to an existing installation") ||
		strings.Contains(msg, "resolved source is not bound to an installation") ||
		strings.Contains(msg, "plugin is not materialized") {
		return fmt.Errorf("%w: %w", ErrNotInstalled, err)
	}
	return err
}
