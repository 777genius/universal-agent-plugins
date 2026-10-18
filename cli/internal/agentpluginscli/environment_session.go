package agentpluginscli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/manifestedit"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

const dependencyMismatch = "uap.toml and uap.lock disagree (including an interrupted write); run agentplugins deps lock (without changing agents) or agentplugins deps sync, then retry"

type environmentSnapshot struct {
	Requirements             environment.Requirements
	Lock                     environment.Lock
	ManifestPath, LockPath   string
	ManifestBytes, LockBytes []byte
}

func snapshotEnvironment(cmd *cobra.Command, app App) (environmentSnapshot, error) {
	s := environmentSnapshot{}
	if err := requireDependencyMutationReady(app); err != nil {
		return s, err
	}
	if app.Installer == nil {
		return s, errors.New("typed SDK installer is unavailable")
	}
	view, err := app.Installer.Inspect(cmd.Context())
	if err != nil {
		return s, err
	}
	if view.Recovery.Required {
		return s, installer.ErrRecoveryRequired
	}
	s.ManifestPath, s.LockPath = app.dependencyPaths()
	s.ManifestBytes, err = os.ReadFile(s.ManifestPath)
	if os.IsNotExist(err) {
		s.ManifestBytes = nil
		s.Requirements = environment.Requirements{SchemaVersion: 1, Dependencies: map[string]environment.Requirement{}, Targets: map[string][]string{}}
	} else if err != nil {
		return s, err
	} else {
		s.Requirements, err = readDependencyManifestBytes(s.ManifestPath, s.ManifestBytes)
		if err != nil {
			return s, err
		}
	}
	s.LockBytes, err = os.ReadFile(s.LockPath)
	if os.IsNotExist(err) {
		s.LockBytes = nil
		if len(s.Requirements.Dependencies) > 0 {
			return s, errors.New(dependencyMismatch)
		}
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if s.ManifestBytes == nil {
		_, err = decodeDependencyLockBytes(s.LockBytes, s.Requirements, true)
		return s, err
	}
	s.Lock, err = decodeDependencyLockBytes(s.LockBytes, s.Requirements, false)
	if err != nil {
		return s, fmt.Errorf("%s: %w", dependencyMismatch, err)
	}
	canonicalizeRequirementsFromLock(&s.Requirements, s.Lock)
	if err := verifyDependencyDeclarations(cmd.Context(), app, s.Lock); err != nil {
		return s, err
	}
	return s, nil
}

// runSavedEnvironment shares preflight, confirmation and publication with deps sync.
// The candidate is fully validated before either intent file is published.
func runSavedEnvironment(cmd *cobra.Command, app App, opts *options, snapshot environmentSnapshot, requirements environment.Requirements, locked environment.Lock, manifest []byte, aliases []string, attest *environment.Attestation) error {
	if err := validateDependencyTargets(app, requirements); err != nil {
		return err
	}
	if err := locked.Validate(requirements); err != nil {
		return err
	}
	if err := verifyDependencyDeclarations(cmd.Context(), app, locked); err != nil {
		return err
	}
	aliases = savedClosureAliases(snapshot.Lock, locked, aliases)
	if err := verifyUnselectedPins(snapshot.Lock, locked, aliases); err != nil {
		return err
	}
	view, err := app.Installer.Inspect(cmd.Context())
	if err != nil {
		return err
	}
	observed, err := environment.Observe(view)
	if err != nil {
		return err
	}
	prepared, err := (environment.Coordinator{}).PrepareSyncFor(requirements, locked, observed, aliases, attest)
	if err != nil {
		return err
	}
	detected, err := detectDependencyClients(cmd.Context(), app)
	if err != nil {
		return err
	}
	consent := map[string]bool{}
	actions, err := preflightDependencyActions(cmd, app, opts, prepared.Bindings, detected, consent)
	if err != nil {
		return err
	}
	if opts.dryRun {
		return renderDependencySync(cmd, opts, actions, true)
	}
	body, err := json.MarshalIndent(locked, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	// Match the established lock encoding so unchanged commands never rewrite it.
	if equalDependencyLock(snapshot.Lock, locked) || (snapshot.Lock.LockVersion == 0 && len(requirements.Dependencies) == 0) {
		body = snapshot.LockBytes
	}
	request := dependencySyncRequest{requirements: requirements, lock: locked, manifestPath: snapshot.ManifestPath, lockPath: snapshot.LockPath, manifestBytes: snapshot.ManifestBytes, lockBytes: snapshot.LockBytes, prepared: actions, detected: detected, consent: consent, candidateManifest: manifest, candidateLock: body, aliases: aliases, attestation: attest}
	return applyDependencySync(cmd, app, opts, request, mutationConfirmed(app, opts))
}

// Replaced only by disposable failure-injection tests.
var publishEnvironmentFile = func(path string, body []byte) error {
	mode := os.FileMode(0o600)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	return atomicfile.Write(path, body, mode)
}

func sameEnvironmentFile(path string, expected []byte) bool {
	body, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return expected == nil
	}
	return err == nil && bytes.Equal(body, expected)
}

func editEnvironmentIntent(snapshot environmentSnapshot, wanted environment.Requirements) ([]byte, error) {
	if reflect.DeepEqual(snapshot.Requirements, wanted) {
		return snapshot.ManifestBytes, nil
	}
	changes := manifestedit.Changes{Dependencies: dependencyIntentEdits(snapshot.Requirements, wanted), Targets: targetIntentEdits(snapshot.Requirements, wanted)}
	body := snapshot.ManifestBytes
	if body == nil {
		body = initialDependencyManifest()
	}
	edited, err := manifestedit.Apply(body, changes)
	if err != nil {
		return nil, err
	}
	actual, err := readDependencyManifestBytes(snapshot.ManifestPath, edited)
	if err != nil {
		return nil, err
	}
	canonicalizeRequirementsFromLock(&actual, snapshot.Lock)
	if !reflect.DeepEqual(actual, wanted) {
		return nil, errors.New("edited manifest does not match intended requirements")
	}
	return edited, nil
}

func verifyUnselectedPins(previous, locked environment.Lock, aliases []string) error {
	for _, old := range previous.Packages {
		selected := false
		for _, alias := range aliases {
			if old.Alias == alias {
				selected = true
			}
		}
		if selected {
			continue
		}
		found := false
		for _, pkg := range locked.Packages {
			if old.Alias == pkg.Alias {
				found = environment.SamePin(old, pkg)
			}
		}
		if !found {
			return errors.New("internal error: unselected dependency pin changed")
		}
	}

	return nil
}

func dependencyIntentEdits(previous, wanted environment.Requirements) []manifestedit.DependencyEdit {
	var edits []manifestedit.DependencyEdit
	aliases := map[string]bool{}
	for alias := range previous.Dependencies {
		aliases[alias] = true
	}
	for alias := range wanted.Dependencies {
		aliases[alias] = true
	}
	names := make([]string, 0, len(aliases))
	for alias := range aliases {
		names = append(names, alias)
	}
	sort.Strings(names)
	for _, alias := range names {
		old, oldOK := previous.Dependencies[alias]
		value, ok := wanted.Dependencies[alias]
		if ok && oldOK && reflect.DeepEqual(old, value) {
			continue
		}
		edit := manifestedit.DependencyEdit{Alias: alias}
		if ok {
			v := &manifestedit.Dependency{}
			if value.Git != nil {
				v.Git = "https://github.com/" + value.Git.Repository + ".git"
				v.Path = value.Git.Path
				switch value.Git.Selector.Kind {
				case environment.SelectorCommit:
					v.Commit = value.Git.Selector.Value
				case environment.SelectorRef:
					v.Ref = value.Git.Selector.Value
				}
			} else {
				v.Registry = "uap"
				v.Distribution = value.Registry.Distribution
				if value.Registry.Selector.Kind == environment.SelectorVersion || value.Registry.Selector.Kind == environment.SelectorRange {
					v.Version = value.Registry.Selector.Value
					v.Registry, v.Distribution = "", ""
				} else {
					v.Release = value.Registry.Selector.Value
				}
			}
			edit.Value = v
		}
		edits = append(edits, edit)
	}

	return edits
}

func targetIntentEdits(previous, wanted environment.Requirements) []manifestedit.TargetEdit {
	var edits []manifestedit.TargetEdit
	targets := map[string]bool{}
	for target := range previous.Targets {
		targets[target] = true
	}
	for target := range wanted.Targets {
		targets[target] = true
	}
	names := make([]string, 0, len(targets))
	for target := range targets {
		names = append(names, target)
	}
	sort.Strings(names)
	for _, target := range names {
		old, oldOK := previous.Targets[target]
		value, ok := wanted.Targets[target]
		if oldOK && ok && reflect.DeepEqual(old, value) {
			continue
		}
		edits = append(edits, manifestedit.TargetEdit{Name: target, Aliases: value, Remove: !ok})
	}

	return edits
}
