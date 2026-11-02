package agentpluginscli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

type dependencyTransferIdentity struct {
	Exists bool   `json:"exists"`
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256,omitempty"`
}

type dependencyTransferFileOutput struct {
	Name   string                     `json:"name"`
	Action string                     `json:"action"`
	Before dependencyTransferIdentity `json:"before"`
	After  dependencyTransferIdentity `json:"after"`
}

type dependencyTransferChange struct {
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Action string `json:"action"`
	Before any    `json:"before,omitempty"`
	After  any    `json:"after,omitempty"`
}

type dependencyTransferOutput struct {
	Diagnostics           []json.RawMessage              `json:"diagnostics,omitempty"`
	Error                 string                         `json:"error,omitempty"`
	Operation             string                         `json:"operation"`
	Scope                 string                         `json:"scope"`
	SourceDirectory       string                         `json:"source_directory"`
	DestinationDirectory  string                         `json:"destination_directory"`
	DryRun                bool                           `json:"dry_run"`
	Changed               bool                           `json:"changed"`
	Outcome               string                         `json:"outcome"`
	Files                 []dependencyTransferFileOutput `json:"files"`
	SemanticChanges       []dependencyTransferChange     `json:"semantic_changes"`
	DestinationDiagnostic string                         `json:"destination_diagnostic,omitempty"`
	InstallRequested      bool                           `json:"install_requested"`
	AgreementSaved        bool                           `json:"agreement_saved"`
	Actions               []dependencyActionDTO          `json:"actions,omitempty"`
	Results               []dependencyResultDTO          `json:"results,omitempty"`
}

func dependencyTransferFileIdentity(file dependencyTransferFile) dependencyTransferIdentity {
	identity := dependencyTransferIdentity{Exists: file.info != nil, Bytes: len(file.body)}
	if identity.Exists {
		digest := sha256.Sum256(file.body)
		identity.SHA256 = hex.EncodeToString(digest[:])
	}
	return identity
}

func newDependencyTransferOutput(transfer dependencyTransfer, operation string, dryRun, install bool) dependencyTransferOutput {
	out := dependencyTransferOutput{Operation: operation, Scope: "user", SourceDirectory: transfer.source.directory, DestinationDirectory: transfer.destination.directory, DryRun: dryRun, Outcome: "preview", InstallRequested: install, DestinationDiagnostic: transfer.destination.diagnostic, Files: []dependencyTransferFileOutput{}, SemanticChanges: []dependencyTransferChange{}}
	for i, file := range transfer.source.files {
		before, after := dependencyTransferFileIdentity(transfer.destination.files[i]), dependencyTransferFileIdentity(file)
		action := "unchanged"
		if !before.Exists || !bytes.Equal(transfer.destination.files[i].body, file.body) {
			action = "replace"
			if !before.Exists {
				action = "create"
			}
			out.Changed = true
		}
		out.Files = append(out.Files, dependencyTransferFileOutput{Name: []string{"uap.toml", "uap.lock"}[i], Action: action, Before: before, After: after})
	}
	if transfer.destination.diagnostic == "" {
		out.SemanticChanges = dependencyTransferChanges(transfer.destination, transfer.source)
	}
	return out
}

func dependencyTransferChanges(previous, next dependencyTransferPair) []dependencyTransferChange {
	before, after := dependencyTransferSemantics(previous), dependencyTransferSemantics(next)
	keys := make([]string, 0, len(before)+len(after))
	for key := range before {
		keys = append(keys, key)
	}
	for key := range after {
		if _, ok := before[key]; !ok {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	changes := []dependencyTransferChange{}
	for _, key := range keys {
		old, oldOK := before[key]
		value, exists := after[key]
		if oldOK && exists && reflect.DeepEqual(old.After, value.After) {
			continue
		}
		change := value
		change.Before, change.Action = old.After, "changed"
		if !oldOK {
			change.Action = "added"
		}
		if !exists {
			change = old
			change.Before, change.After, change.Action = old.After, nil, "removed"
		}
		changes = append(changes, change)
	}
	return changes
}

func dependencyTransferSemantics(pair dependencyTransferPair) map[string]dependencyTransferChange {
	out := map[string]dependencyTransferChange{}
	add := func(kind, name string, value any) {
		out[kind+"\x00"+name] = dependencyTransferChange{Kind: kind, Name: name, After: value}
	}
	for name, requirement := range pair.requirements.Dependencies {
		add("dependency", name, requirement)
	}
	for _, pkg := range pair.lock.Packages {
		add("locked_package", pkg.Alias, pkg)
	}
	for target, aliases := range pair.requirements.Targets {
		for _, alias := range aliases {
			add("target", target+"/"+alias, true)
		}
	}
	return out
}

func renderDependencyTransfer(cmd *cobra.Command, opts *options, out dependencyTransferOutput) error {
	if opts.format == "json" {
		return renderJSON(cmd.OutOrStdout(), out)
	}
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Dependency agreement %s (%s): %s -> %s\n", out.Operation, out.Scope, out.SourceDirectory, out.DestinationDirectory); err != nil {
		return err
	}
	if out.DestinationDiagnostic != "" {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Destination agreement is invalid/incomplete: %s\n", out.DestinationDiagnostic); err != nil {
			return err
		}
	}
	for _, file := range out.Files {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s %s: %d bytes sha256:%s -> %d bytes sha256:%s\n", file.Action, file.Name, file.Before.Bytes, file.Before.SHA256, file.After.Bytes, file.After.SHA256); err != nil {
			return err
		}
	}
	for _, change := range out.SemanticChanges {
		before, _ := json.Marshal(change.Before)
		after, _ := json.Marshal(change.After)
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s %s %s: %s -> %s\n", change.Kind, change.Name, change.Action, before, after); err != nil {
			return err
		}
	}
	if out.Outcome != "preview" {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), "Agreement transfer:", out.Outcome)
		return err
	}
	return nil
}

func renderDependencyTransferResults(cmd *cobra.Command, opts *options, transfer *dependencyTransfer, request dependencySyncRequest, results []installer.Result, applyErr error) error {
	transfer.output.Actions = dependencyActionOutput(request.prepared, opts.dependencyComponents)
	transfer.output.Results = dependencyResultOutput(results)
	transfer.output.Outcome = "completed"
	if applyErr != nil {
		transfer.output.Outcome = "partial"
	}
	if opts.format == "json" {
		return renderDependencyTransfer(cmd, opts, transfer.output)
	}
	if err := renderDependencyTransfer(cmd, opts, transfer.output); err != nil {
		return err
	}
	return renderDependencyHumanResults(cmd, opts, request.prepared, transfer.output.Results)
}

func renderDependencyTransferInstallPlan(cmd *cobra.Command, opts *options, transfer *dependencyTransfer) error {
	if err := renderDependencyTransfer(cmd, opts, transfer.output); err != nil {
		return err
	}
	if opts.format == "human" {
		return renderDependencySync(cmd, opts, transfer.prepared, true)
	}
	return nil
}
