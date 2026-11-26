package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// earlyForeignBindingError is presentation for an approved command boundary
// before a result document exists. Apply errors retain their result envelope.
type earlyForeignBindingError struct {
	cause error
	scope domain.InstallScope
	root  string
}

func (err *earlyForeignBindingError) Unwrap() error { return err.cause }

func (err *earlyForeignBindingError) Error() string {
	var refusal *environment.ForeignBindingError
	if !errors.As(err.cause, &refusal) {
		return err.cause.Error()
	}
	lines := []string{"Saved installation is blocked by existing package ownership."}
	for _, conflict := range foreignBindingConflicts(refusal) {
		label := conflict.SourceKey
		if conflict.Alias != "" {
			label = conflict.Alias
		}
		if conflict.RootAlias != "" {
			label = conflict.RootAlias + " -> " + label
		}
		reason := "has bindings outside this saved environment"
		if conflict.Reason == "foreign_environment" {
			reason = "belongs to another saved environment"
		}
		names := make([]string, len(conflict.ClientIDs))
		for index, id := range conflict.ClientIDs {
			names[index] = domain.ClientDisplayName(domain.ClientID(id))
		}
		lines = append(lines, fmt.Sprintf("%s %s: installation %s; clients %s.", label, reason, conflict.InstallationID, strings.Join(names, ", ")))
		lines = append(lines, err.inspectGuidance(conflict.InstallationID))
	}
	return strings.Join(append(lines, "Preserve the existing bindings and inspect their ownership before choosing a next step. This command cannot transfer ownership automatically."), "\n")
}

func (err *earlyForeignBindingError) inspectGuidance(id string) string {
	args := []string{"agentplugins", "info", id, "--scope", string(err.scope)}
	if err.scope == domain.ScopeProject {
		args = append(args, "--project-root", err.root)
	}
	for _, arg := range args {
		if !isBatchRetrySafeArg(arg) {
			return fmt.Sprintf("Inspect installation %s with info using scope %s and project root %q.", id, err.scope, err.root)
		}
	}
	return "Inspect: " + strings.Join(args, " ")
}

func earlyForeignBinding(app App, err error) error {
	var refusal *environment.ForeignBindingError
	if !errors.As(err, &refusal) {
		return err
	}
	return &earlyForeignBindingError{cause: err, scope: app.scope(), root: app.ProjectRoot}
}

// A known source key permits an advisory refusal only. The complete resolved
// graph and the mutation-lock recheck still decide the actual operation.
func checkSavedAddOwnership(ctx context.Context, app App, sourceKey, alias string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if sourceKey == "" || app.Installer == nil {
		return nil
	}
	view, err := app.Installer.Inspect(ctx)
	if err != nil {
		return err
	}
	observed, err := environment.ObserveForEnvironment(view, app.environmentID())
	if err != nil {
		return err
	}
	return earlyForeignBinding(app, environment.CheckSourceOwnership(environment.LockedPackage{Alias: alias, SourceKey: sourceKey}, observed))
}

type foreignBindingConflict struct {
	Reason             string   `json:"reason"`
	Alias              string   `json:"alias"`
	SourceKey          string   `json:"source_key"`
	InstallationID     string   `json:"installation_id"`
	ClientIDs          []string `json:"client_ids"`
	RootAlias          string   `json:"root_alias,omitempty"`
	OwnerEnvironmentID string   `json:"owner_environment_id,omitempty"`
}

func foreignBindingConflicts(refusal *environment.ForeignBindingError) []foreignBindingConflict {
	groups := map[string]*foreignBindingConflict{}
	for _, binding := range refusal.Bindings {
		key := binding.InstallationID + "\x00" + binding.Reason + "\x00" + binding.OwnerEnvironmentID
		group := groups[key]
		if group == nil {
			group = &foreignBindingConflict{Reason: binding.Reason, Alias: refusal.Alias, SourceKey: refusal.SourceKey, InstallationID: binding.InstallationID, RootAlias: refusal.RootAlias, OwnerEnvironmentID: binding.OwnerEnvironmentID, ClientIDs: []string{}}
			groups[key] = group
		}
		group.ClientIDs = append(group.ClientIDs, binding.ClientID)
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]foreignBindingConflict, 0, len(keys))
	for _, key := range keys {
		group := groups[key]
		sort.Strings(group.ClientIDs)
		group.ClientIDs = slices.Compact(group.ClientIDs)
		out = append(out, *group)
	}
	return out
}
