package agentpluginscli

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path"
	"path/filepath"
	"sort"
	"sync"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
)

// Authoring is a separate file-edit permission. A trusted project Skills root
// does not enable project-scope native installation in a user-scope engine.
func normalizeManagementAuthoringRoots(input *management.Startup) error {
	if len(input.AuthoringRoots) > 64 {
		return errors.New("at most 64 explicit authoring roots allowed")
	}
	ids, paths := map[string]bool{}, map[string]bool{}
	input.AuthoringRoots = append([]management.LocalSkillRoot(nil), input.AuthoringRoots...)
	for i := range input.AuthoringRoots {
		root := &input.AuthoringRoots[i]
		if root.Id == "" || len(root.Id) > 128 || ids[root.Id] || root.Scope != management.User && root.Scope != management.Project {
			return errors.New("unique authoring root IDs and user/project authoring scope required")
		}
		ids[root.Id] = true
		physical, err := managementAbsolute(root.Path)
		if err != nil {
			return err
		}
		root.Path = physical
		if paths[physical] || localSkillPathOverlap(physical, input.StateRoot) {
			return errors.New("authoring roots must be distinct from engine state")
		}
		paths[physical] = true
		if root.TargetId != "" {
			found := false
			for _, target := range input.Targets {
				if target.Id == root.TargetId {
					found = true
					break
				}
			}
			if !found {
				return errors.New("authoring target does not belong to captured context")
			}
		}
	}
	sort.Slice(input.AuthoringRoots, func(i, j int) bool { return input.AuthoringRoots[i].Id < input.AuthoringRoots[j].Id })
	return nil
}

type localSkillChange struct {
	relative string
	expected *string
	content  []byte
	remove   bool
}
type managementLocalSkill struct {
	mu           sync.Mutex
	backend      *ManagementBackend
	root         management.LocalSkillRoot
	input        management.LocalSkillRequest
	operationID  string
	before       localSkillTree
	rootAnchor   string
	rootIdentity os.FileInfo
	changes      []localSkillChange
	restored     *localSkillReceipt
	review       management.Plan
	closed       bool
}

func (b *ManagementBackend) PrepareLocalSkill(ctx context.Context, input management.LocalSkillRequest, operationID string) (management.Prepared, error) {
	if !localSkillName(input.SkillName) {
		return nil, localSkillRefusal("INVALID_REQUEST", "Choose a single portable Skill directory name")
	}
	if len(operationID) != 64 {
		return nil, errors.New("internal local operation identity required")
	}
	var root management.LocalSkillRoot
	for _, candidate := range b.config.AuthoringRoots {
		if candidate.Id == input.RootId {
			root = candidate
			break
		}
	}
	if root.Id == "" {
		return nil, localSkillRefusal("AUTHORING_ROOT_UNAVAILABLE", "This Skill root was not captured by the host")
	}
	p := &managementLocalSkill{backend: b, root: root, input: input, operationID: operationID}
	var err error
	p.changes, err = parseLocalSkillChanges(input)
	if err != nil {
		return nil, err
	}
	unlock, err := b.lockLocalSkill(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = unlock() }()
	if err = b.localSkillUnowned(ctx, filepath.Join(root.Path, input.SkillName)); err != nil {
		return nil, err
	}
	p.rootAnchor, p.rootIdentity, err = localSkillRootAnchor(root.Path)
	if err != nil {
		return nil, err
	}
	p.before, err = localSkillObserve(root.Path, input.SkillName, false)
	if err != nil {
		return nil, err
	}
	if string(input.Action) == "restore" {
		p.restored, err = b.readLocalSkillReceipt(input.RestoreOperationId, root, input.SkillName)
		if err != nil {
			return nil, err
		}
		if err = p.validateRestoreSnapshot(); err != nil {
			return nil, err
		}
	} else if string(input.Action) == "delete" {
		if !p.before.Exists {
			return nil, localSkillRefusal("SKILL_NOT_FOUND", "This local Skill is already absent")
		}
	} else if err = p.verifyChanges(p.before); err != nil {
		return nil, err
	}
	p.review = management.Plan{Action: management.ChangeAction("local_skill"), ContextId: b.ContextID(), Confirmable: true, Actions: []management.PlanAction{}, Warnings: []management.Diagnostic{}, Conflicts: []management.Diagnostic{}}
	p.review.Actions = append(p.review.Actions, management.PlanAction{Action: string(input.Action), Alias: input.SkillName, TargetId: root.TargetId, DestinationPath: filepath.Join(root.Path, input.SkillName), Description: "Apply reviewed local Skill " + string(input.Action)})
	if p.before.Exists {
		p.review.Warnings = append(p.review.Warnings, managementDiagnostic("LOCAL_BACKUP_RETAINED", "The previous local Skill is retained in an engine-owned reversible backup"))
	}
	return p, nil
}

func parseLocalSkillChanges(input management.LocalSkillRequest) ([]localSkillChange, error) {
	if string(input.Action) == "delete" || string(input.Action) == "restore" {
		if len(input.Files) != 0 || (string(input.Action) == "restore") != (input.RestoreOperationId != "") {
			return nil, localSkillRefusal("INVALID_REQUEST", "Delete and restore do not accept file changes; restore requires an operation identity")
		}
		return nil, nil
	}
	if string(input.Action) != "upsert" || input.RestoreOperationId != "" || len(input.Files) == 0 || len(input.Files) > localSkillMaxFiles {
		return nil, localSkillRefusal("INVALID_REQUEST", "Upsert requires one to 200 exact file changes")
	}
	out := make([]localSkillChange, 0, len(input.Files))
	seen, total := map[string]bool{}, 0
	for _, file := range input.Files {
		if !localSkillRelative(file.RelativePath) || seen[file.RelativePath] {
			return nil, localSkillRefusal("INVALID_REQUEST", "File paths must be distinct portable relative paths")
		}
		seen[file.RelativePath] = true
		if file.ExpectedSHA256 != nil && !localSkillSHA(*file.ExpectedSHA256) {
			return nil, localSkillRefusal("INVALID_REQUEST", "Expected file digest must be lowercase SHA-256 or null")
		}
		change := localSkillChange{relative: file.RelativePath, expected: file.ExpectedSHA256, remove: file.ContentBase64 == nil}
		if change.remove && change.expected == nil {
			return nil, localSkillRefusal("INVALID_REQUEST", "Removing a file requires its reviewed digest")
		}
		if !change.remove {
			body, err := base64.StdEncoding.Strict().DecodeString(*file.ContentBase64)
			if err != nil {
				return nil, localSkillRefusal("INVALID_REQUEST", "Replacement bytes must be standard base64")
			}
			total += len(body)
			if total > localSkillMaxBytes {
				return nil, localSkillRefusal("REQUEST_TOO_LARGE", "Local Skill replacements exceed 10 MiB")
			}
			change.content = body
		}
		out = append(out, change)
	}
	return out, nil
}
func localSkillSHA(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, c := range value {
		if c < '0' || c > '9' {
			if c < 'a' || c > 'f' {
				return false
			}
		}
	}
	return true
}
func localSkillRefusal(code, message string) error {
	return &management.Refusal{Code: code, Message: message, Status: 409}
}
func (p *managementLocalSkill) Review() management.Plan { return p.review }
func (p *managementLocalSkill) OperationIDs() []string  { return []string{p.operationID} }
func (p *managementLocalSkill) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	p.changes = nil
}
func (p *managementLocalSkill) Check(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	unlock, err := p.backend.lockLocalSkill(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = unlock() }()
	return p.check(ctx)
}
func (p *managementLocalSkill) check(ctx context.Context) error {
	if p.closed {
		return management.ErrPlanStale
	}
	if err := p.backend.localSkillUnowned(ctx, filepath.Join(p.root.Path, p.input.SkillName)); err != nil {
		return err
	}
	physical, err := managementAbsolute(p.root.Path)
	if err != nil || physical != p.root.Path {
		return management.ErrPlanStale
	}
	info, err := os.Stat(p.rootAnchor)
	if err != nil || !os.SameFile(info, p.rootIdentity) {
		return management.ErrPlanStale
	}
	live, err := localSkillObserve(p.root.Path, p.input.SkillName, false)
	if err != nil || live.digest() != p.before.digest() {
		return management.ErrPlanStale
	}
	if string(p.input.Action) == "restore" {
		current, readErr := p.backend.readLocalSkillReceipt(p.input.RestoreOperationId, p.root, p.input.SkillName)
		if readErr != nil || (current == nil) != (p.restored == nil) || current != nil && *current != *p.restored {
			return management.ErrPlanStale
		}
	}
	return nil
}
func (p *managementLocalSkill) verifyChanges(tree localSkillTree) error {
	files := map[string]localSkillFile{}
	for _, file := range tree.Files {
		files[file.Path] = file
	}
	for _, change := range p.changes {
		current, exists := files[change.relative]
		if change.expected == nil && exists || change.expected != nil && (!exists || current.SHA256 != *change.expected) {
			return management.ErrPlanStale
		}
	}
	return nil
}
func localSkillRootAnchor(root string) (string, os.FileInfo, error) {
	for {
		info, err := os.Stat(root)
		if err == nil {
			if !info.IsDir() {
				return "", nil, errors.New("authoring root parent is not a directory")
			}
			return root, info, nil
		}
		if !os.IsNotExist(err) {
			return "", nil, err
		}
		parent := filepath.Dir(root)
		if parent == root {
			return "", nil, err
		}
		root = parent
	}
}
func localSkillObserve(rootPath, name string, keep bool) (localSkillTree, error) {
	root, err := os.OpenRoot(rootPath)
	if os.IsNotExist(err) {
		return localSkillTree{Directories: []localSkillDirectory{}, Files: []localSkillFile{}}, nil
	}
	if err != nil {
		return localSkillTree{}, err
	}
	defer root.Close()
	return readLocalSkillTree(root, name, keep)
}
func (b *ManagementBackend) localSkillUnowned(ctx context.Context, target string) error {
	if b.app.Installer == nil || b.app.StateStore == nil {
		return localSkillRefusal("OWNERSHIP_UNAVAILABLE", "Authoritative installation ownership is unavailable")
	}
	view, err := b.app.Installer.Inspect(ctx)
	if err != nil {
		return err
	}
	if view.Recovery.Required {
		return localSkillRefusal("RECOVERY_REQUIRED", "Recover pending installer work before local authoring")
	}
	state, err := b.app.StateStore.Load()
	if err != nil {
		return err
	}
	for _, installation := range state.Installations {
		for _, binding := range installation.Clients {
			if localSkillPathOverlap(target, binding.TargetLocator) {
				return localSkillRefusal("MANAGED_SKILL", "This path is managed by UAP; make an editable copy in a separate local directory")
			}
			for _, object := range binding.NativeObjects {
				if object.ProtectionClass == "managed" && localSkillPathOverlap(target, object.Path) {
					return localSkillRefusal("MANAGED_SKILL", "This path is managed by UAP; make an editable copy in a separate local directory")
				}
			}
		}
	}
	return nil
}
func (p *managementLocalSkill) desiredLocalTree(current localSkillTree) (localSkillTree, error) {
	current.Exists = true
	if current.Mode == 0 {
		current.Mode = 0755
	}
	files := map[string]localSkillFile{}
	dirs := map[string]uint32{}
	for _, f := range current.Files {
		files[f.Path] = f
	}
	for _, d := range current.Directories {
		dirs[d.Path] = d.Mode
	}
	for _, change := range p.changes {
		if change.remove {
			delete(files, change.relative)
			continue
		}
		mode := uint32(0644)
		if previous, exists := files[change.relative]; exists {
			mode = previous.Mode
		}
		files[change.relative] = localSkillFile{Path: change.relative, Mode: mode, SHA256: localSkillDigest(change.content), Bytes: change.content}
		for parent := path.Dir(change.relative); parent != "."; parent = path.Dir(parent) {
			if _, exists := dirs[parent]; !exists {
				dirs[parent] = 0755
			}
		}
	}
	current.Files = []localSkillFile{}
	current.Directories = []localSkillDirectory{}
	total := 0
	for _, file := range files {
		total += len(file.Bytes)
		current.Files = append(current.Files, file)
	}
	for directory, mode := range dirs {
		current.Directories = append(current.Directories, localSkillDirectory{Path: directory, Mode: mode})
	}
	if len(current.Files) > localSkillMaxFiles || total > localSkillMaxBytes {
		return current, localSkillRefusal("REQUEST_TOO_LARGE", "The resulting local Skill exceeds 200 files or 10 MiB")
	}
	for _, file := range current.Files {
		if _, exists := dirs[file.Path]; exists {
			return current, localSkillRefusal("INVALID_REQUEST", "A file cannot also be a directory")
		}
	}
	sortLocalSkillTree(&current)
	return current, nil
}

func (b *ManagementBackend) lockLocalSkill(ctx context.Context) (func() error, error) {
	unlockEnvironment, err := (processlock.Lock{Path: b.app.EnvironmentLockPath}).Acquire(ctx)
	if err != nil {
		return nil, err
	}
	nativePath := filepath.Join(b.config.StateRoot, "mutation.lock")
	if filepath.Clean(b.app.EnvironmentLockPath) == nativePath {
		return unlockEnvironment, nil
	}
	unlockNative, err := (processlock.Lock{Path: nativePath}).Acquire(ctx)
	if err != nil {
		_ = unlockEnvironment()
		return nil, err
	}
	return func() error { return errors.Join(unlockNative(), unlockEnvironment()) }, nil
}
