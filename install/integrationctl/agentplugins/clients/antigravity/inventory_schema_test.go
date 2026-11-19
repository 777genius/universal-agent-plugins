package antigravity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func inventorySchemaRow(plan domain.DeliveryPlan, enabled bool) map[string]any {
	return map[string]any{"name": filepath.Base(plan.ActivePath), "path": plan.ActivePath,
		"version": plan.DeclaredVersion, "enabled": enabled}
}

func inventorySchemaForeign(plan domain.DeliveryPlan) map[string]any {
	return map[string]any{"name": "foreign-readback-control", "version": "1.0.0", "enabled": false,
		"path": filepath.Join(filepath.Dir(plan.ActivePath), "foreign-readback-control")}
}

func inventorySchemaBody(t *testing.T, rows ...map[string]any) []byte {
	t.Helper()
	body, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func inventorySchemaOriginals(t *testing.T, req domain.ActivationRequest) []*inventoryFile {
	t.Helper()
	foreign := filepath.Join(req.Plan.TargetRoot, "foreign-readback-control")
	if err := os.Mkdir(foreign, 0700); err != nil {
		t.Fatal(err)
	}
	manifest := map[string]any{"$schema": "https://antigravity.google/schemas/v1/plugin.json", "name": "foreign-readback-control"}
	if req.Plan.DeclaredVersion != "" {
		manifest["version"] = "1.0.0"
	}
	if err := shared.WriteJSON(filepath.Join(foreign, "plugin.json"), manifest); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(foreign, "sentinel"), []byte("foreign must survive\n"), 0600); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(req.Client.ConfigRoot, "config.json")
	if err := shared.WriteJSON(config, map[string]any{"plugins": map[string]any{
		"foreign-readback-control":         map[string]any{"enabled": false},
		filepath.Base(req.Plan.ActivePath): map[string]any{"enabled": false},
	}}); err != nil {
		t.Fatal(err)
	}
	originals := make([]*inventoryFile, 0, 4)
	for _, path := range []string{config, filepath.Join(req.Plan.ActivePath, "plugin.json"), filepath.Join(foreign, "plugin.json"), filepath.Join(foreign, "sentinel")} {
		record, err := openInventoryFile(t.Context(), path, 1<<20, false)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := record.close(); err != nil {
				t.Error(err)
			}
		})
		originals = append(originals, record)
	}
	return originals
}

func TestInventoryConfiguredRepeatAcceptsOmittedForeignComponents(t *testing.T) {
	req, env, runner := inventoryFixture(t)
	req.PackageUnchanged = true
	originals := inventorySchemaOriginals(t, req)
	selected := inventorySchemaRow(req.Plan, false)
	selected["skills"], selected["mcpServers"] = []string{"skill"}, []string{"server"}
	body := inventorySchemaBody(t, inventorySchemaForeign(req.Plan), selected)
	runner.reply = func(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
		stdout := []byte("1.5.3\n")
		if len(command.Argv) == 4 {
			stdout = body
		}
		return legacyports.CommandResult{CleanupComplete: true, Stdout: stdout}, nil
	}
	outcome, err := New().Activate(t.Context(), env, req)
	if err != nil || outcome.Activation != domain.ActivationPrepared || outcome.Verification != domain.VerificationInstalled ||
		!outcome.AuthoritativeObservation || outcome.ActivationAttested || outcome.AuthenticationAttested ||
		len(outcome.NativeObjects) != 0 || outcome.NativeEffect != domain.NativeEffectUnchanged {
		t.Fatal("pinned four-field foreign inventory prevented prepared observation", outcome, err)
	}
	if len(outcome.UserActions) != 1 || outcome.UserActions[0] != "The selected User plugin remains disabled; decide whether to enable it in Antigravity" {
		t.Fatal("disabled observation lost exact guidance", outcome.UserActions)
	}
	if len(runner.calls) != 2 || strings.Join(runner.calls[0].Argv[1:], " ") != "--version" ||
		strings.Join(runner.calls[1].Argv[1:], " ") != "plugin list --json" {
		t.Fatal("readback changed the native command pair", runner.calls)
	}
	for _, original := range originals {
		if err := original.verify(t.Context()); err != nil {
			t.Fatal("readback changed original config, selected or foreign evidence", err)
		}
	}
}

func inventorySchemaSelection(t *testing.T, shape string, enabled, explicitEmpty bool) (domain.DeliveryPlan, map[string]any) {
	t.Helper()
	req, _, _ := inventoryFixture(t)
	plan := req.Plan
	plan.Components = nil
	row := inventorySchemaRow(plan, enabled)
	if shape == "both" || shape == "skills" {
		plan.Components = append(plan.Components, req.Plan.Components[0])
		row["skills"] = []string{"skill"}
	}
	if shape == "both" || shape == "mcp" {
		plan.Components = append(plan.Components, req.Plan.Components[1])
		row["mcpServers"] = []string{"server"}
	}
	if explicitEmpty {
		for _, field := range []string{"skills", "mcpServers"} {
			if _, exists := row[field]; !exists {
				row[field] = []string{}
			}
		}
	}
	return plan, row
}

func TestInventoryOptionalComponentFieldsMatchSelectedPlan(t *testing.T) {
	for _, shape := range []string{"both", "skills", "mcp", "empty"} {
		for _, explicitEmpty := range []bool{false, true} {
			for _, enabled := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/explicit-empty=%v/enabled=%v", shape, explicitEmpty, enabled), func(t *testing.T) {
					plan, row := inventorySchemaSelection(t, shape, enabled, explicitEmpty)
					body := inventorySchemaBody(t, inventorySchemaForeign(plan), row)
					got, present, err := inventoryReadback(body, plan, &enabled)
					if err != nil || !present || got != enabled {
						t.Fatal("pinned optional component shape did not match its exact plan", got, present, err)
					}
				})
			}
		}
	}
}

func inventorySchemaOversizedNames() []string {
	names := make([]string, 129)
	for index := range names {
		names[index] = fmt.Sprint(index)
	}
	return names
}

func inventorySchemaFaults() map[string]func(map[string]any) {
	return map[string]func(map[string]any){
		"skills-null":          func(row map[string]any) { row["skills"] = nil },
		"servers-null":         func(row map[string]any) { row["mcpServers"] = nil },
		"skills-scalar":        func(row map[string]any) { row["skills"] = "skill" },
		"servers-scalar":       func(row map[string]any) { row["mcpServers"] = 1 },
		"skills-wrong-element": func(row map[string]any) { row["skills"] = []any{1} },
		"servers-wrong-element": func(row map[string]any) {
			row["mcpServers"] = []any{true}
		},
		"skills-empty-name":  func(row map[string]any) { row["skills"] = []string{""} },
		"servers-empty-name": func(row map[string]any) { row["mcpServers"] = []string{""} },
		"skills-duplicate":   func(row map[string]any) { row["skills"] = []string{"skill", "skill"} },
		"servers-duplicate":  func(row map[string]any) { row["mcpServers"] = []string{"server", "server"} },
		"skills-oversized":   func(row map[string]any) { row["skills"] = inventorySchemaOversizedNames() },
		"servers-oversized":  func(row map[string]any) { row["mcpServers"] = inventorySchemaOversizedNames() },
		"unknown":            func(row map[string]any) { row["unexpected"] = false },
		"missing-name":       func(row map[string]any) { delete(row, "name") },
		"missing-path":       func(row map[string]any) { delete(row, "path") },
		"missing-version":    func(row map[string]any) { delete(row, "version") },
		"missing-enabled":    func(row map[string]any) { delete(row, "enabled") },
		"name-type":          func(row map[string]any) { row["name"] = 1 },
		"path-type":          func(row map[string]any) { row["path"] = 1 },
		"version-type":       func(row map[string]any) { row["version"] = 1 },
		"enabled-type":       func(row map[string]any) { row["enabled"] = "false" },
		"name-empty":         func(row map[string]any) { row["name"] = "" },
		"version-empty":      func(row map[string]any) { row["version"] = "" },
	}
}

func TestInventoryOptionalFieldsKeepEveryRowStrict(t *testing.T) {
	plan, selected := inventorySchemaSelection(t, "both", false, false)
	for index, kind := range []string{"foreign", "selected"} {
		for name, mutate := range inventorySchemaFaults() {
			if index == 0 && (name == "missing-version" || name == "version-empty") {
				continue // Foreign raw Skills can also have no declared version.
			}
			t.Run(kind+"/"+name, func(t *testing.T) {
				rows := []map[string]any{inventorySchemaForeign(plan), maps.Clone(selected)}
				mutate(rows[index])
				if _, _, err := selectedInventory(inventorySchemaBody(t, rows...), plan); !errors.Is(err, errInventoryUnavailable) {
					t.Fatal("invalid optional inventory schema was accepted", err)
				}
			})
		}
	}
}

func TestInventoryOmittedListsKeepIdentityAndRequiredComponents(t *testing.T) {
	plan, selected := inventorySchemaSelection(t, "both", false, false)
	for _, fault := range []string{"missing-skill", "missing-server", "duplicate-foreign", "alias-foreign", "selected-alias", "duplicate-key"} {
		t.Run(fault, func(t *testing.T) {
			rows := []map[string]any{inventorySchemaForeign(plan), maps.Clone(selected)}
			switch fault {
			case "missing-skill":
				delete(rows[1], "skills")
			case "missing-server":
				delete(rows[1], "mcpServers")
			case "duplicate-foreign":
				rows = append(rows, maps.Clone(rows[0]))
			case "alias-foreign":
				alias := maps.Clone(rows[0])
				alias["name"] = "FOREIGN-READBACK-CONTROL"
				alias["path"] = filepath.Join(plan.TargetRoot, "different", "FOREIGN-READBACK-CONTROL")
				rows = append(rows, alias)
			case "selected-alias":
				rows[1]["name"] = strings.ToUpper(filepath.Base(plan.ActivePath))
				rows[1]["path"] = strings.ToUpper(plan.ActivePath)
			}
			body := inventorySchemaBody(t, rows...)
			if fault == "duplicate-key" {
				body = []byte(strings.Replace(string(body), `"enabled":false`, `"enabled":false,"enabled":false`, 1))
			}
			if _, _, err := selectedInventory(body, plan); !errors.Is(err, errInventoryUnavailable) {
				t.Fatal("omitted lists hid an identity conflict or required component", err)
			}
		})
	}
}

func TestInventoryVersionlessSelectedUserRetainsPrepared(t *testing.T) {
	for _, phase := range []string{"initial", "repeat", "verify-only"} {
		for _, omitted := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/omitted=%v", phase, omitted), func(t *testing.T) {
				req, env, runner := inventoryFixture(t)
				req.Plan.DeclaredVersion = ""
				req.Plan.Components = req.Plan.Components[:1]
				req.PackageUnchanged, req.VerifyOnly = phase == "repeat", phase == "verify-only"
				originals := inventorySchemaOriginals(t, req)
				selected := inventorySchemaRow(req.Plan, false)
				selected["skills"] = []string{"skill"}
				if omitted {
					delete(selected, "version")
				}
				foreign := inventorySchemaForeign(req.Plan)
				foreign["version"] = ""
				if omitted {
					delete(foreign, "version")
				}
				body := inventorySchemaBody(t, foreign, selected)
				runner.reply = func(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
					stdout := []byte("1.5.3\n")
					if len(command.Argv) == 4 {
						stdout = body
					}
					return legacyports.CommandResult{CleanupComplete: true, Stdout: stdout}, nil
				}
				got, err := New().Activate(t.Context(), env, req)
				if err != nil || len(runner.calls) != 2 || got.Activation != domain.ActivationPrepared ||
					got.Verification != domain.VerificationInstalled || got.NativeEffect != domain.NativeEffectUnchanged ||
					got.ActivationAttested || got.AuthenticationAttested || !got.AuthoritativeObservation {
					t.Fatal("versionless readback failed or promoted activation", got, err, runner.calls)
				}
				for _, original := range originals {
					if err := original.verify(t.Context()); err != nil {
						t.Fatal("versionless observation changed protected files", err)
					}
				}
			})
		}
	}
}

func TestInventoryVersionlessSelectionKeepsVersionAndIdentityRefusals(t *testing.T) {
	for _, fault := range []string{"selected-version", "selected-null", "selected-number", "foreign-null", "foreign-number", "versioned-omitted", "versioned-mismatch", "selected-path", "selected-name", "missing-skill"} {
		t.Run(fault, func(t *testing.T) {
			plan, selected := inventorySchemaSelection(t, "skills", false, false)
			plan.DeclaredVersion = ""
			delete(selected, "version")
			foreign := inventorySchemaForeign(plan)
			switch fault {
			case "selected-version":
				selected["version"] = "1.0.0"
			case "selected-null":
				selected["version"] = nil
			case "selected-number":
				selected["version"] = 0
			case "foreign-null":
				foreign["version"] = nil
			case "foreign-number":
				foreign["version"] = 0
			case "versioned-omitted":
				plan.DeclaredVersion = "1.0.0"
			case "versioned-mismatch":
				plan.DeclaredVersion = "1.0.0"
				selected["version"] = "2.0.0"
			case "selected-path":
				selected["path"] = plan.ActivePath + "-other"
			case "selected-name":
				selected["name"] = "other"
			case "missing-skill":
				delete(selected, "skills")
			}
			if _, _, err := selectedInventory(inventorySchemaBody(t, foreign, selected), plan); !errors.Is(err, errInventoryUnavailable) {
				t.Fatal("versionless selection bypassed exact authority", err)
			}
		})
	}
}
