//go:build linux

package pluginkitairepo_test

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var geminiCompleteNonce = regexp.MustCompile(`^[0-9a-f]{32}$`)

type geminiCompleteProof struct {
	Views map[string]map[string]*geminiCycleRecipe
	Raw   map[string][]byte
}

func geminiCompleteView(recipes map[string]*geminiCycleRecipe) map[string]*geminiCycleRecipe {
	result := map[string]*geminiCycleRecipe{}
	for scope, recipe := range recipes {
		copyRecipe := *recipe
		result[scope] = &copyRecipe
	}
	return result
}

func geminiCompleteRecipe(value map[string]any, paths map[string]string) (*geminiCycleRecipe, error) {
	row, err := geminiCompleteObject(value, "scope source name revision nonce digest files skill manifest mcp", "")
	scope, _ := row["scope"].(string)
	nonce, _ := row["nonce"].(string)
	revision, _ := row["revision"].(string)
	name := map[string]string{"user": "uapgeminiu", "a": "uapgeminia", "b": "uapgeminib"}[scope]
	digest, _ := row["digest"].(string)
	if err != nil || name == "" || !geminiCompleteNonce.MatchString(nonce) || !geminiCycleDigest(digest) || row["name"] != name || row["source"] != paths["root"]+"/cycle-source-"+scope || !map[string]bool{"user/U-one": true, "a/A-one": true, "a/A-two": true, "b/B-one": true}[scope+"/"+revision] {
		return nil, geminiCompleteRefusal()
	}
	r := &geminiCycleRecipe{Scope: scope, Name: name, Source: paths["root"] + "/cycle-source-" + scope, Project: geminiCompleteScopePath(paths, scope), Revision: revision, Nonce: nonce, Digest: digest, Authored: map[string][]byte{}}
	r.StateRoot, r.Profile = filepath.Join(r.Project, ".uap"), filepath.Join(r.Project, ".gemini")
	if scope == "user" {
		r.StateRoot, r.Profile = paths["root"]+"/installer-state", paths["home"]+"/.gemini"
	}
	r.Config, r.SkillPath = r.Profile+"/settings.json", r.Profile+"/skills/"+name+"/SKILL.md"
	for label, path := range map[string]string{"manifest": "plugin.json", "mcp": "mcp.json", "skill": "skills/" + name + "/SKILL.md"} {
		body, err := geminiCompleteStream(row[label])
		if err != nil {
			return nil, err
		}
		r.Authored[path] = body
	}
	if err := geminiCompleteRecipeBytes(r, row["files"], paths); err != nil {
		return nil, err
	}
	return r, nil
}

func geminiCompleteRecipeBytes(r *geminiCycleRecipe, files any, paths map[string]string) error {
	manifest := map[string]any{"$schema": domain.PluginSchemaV1, "name": geminiCycleName, "version": "1.0.0", "description": "Disposable Gemini lifecycle proof"}
	env := map[string]any{"UAP_TEST_ROOT": paths["root"], "UAP_TEST_EVENTS": paths["root"] + "/events.jsonl", "UAP_TEST_NONCE": r.Nonce, "UAP_TEST_REVISION": r.Revision, "UAP_TEST_LITERAL": "literal ${UNKNOWN} $HOME", "UAP_TEST_ONCE": "${PLUGIN_ROOT}/${UNKNOWN}"}
	mcp := map[string]any{"$schema": domain.MCPSchemaV1, "mcpServers": map[string]any{r.Name: map[string]any{"type": "stdio", "command": "./bin/probe", "args": []any{"argument with spaces", "${PLUGIN_ROOT}/argument", "${UNKNOWN}"}, "cwd": "${PLUGIN_ROOT}", "env": env}}}
	for path, want := range map[string]map[string]any{"plugin.json": manifest, "mcp.json": mcp} {
		got, err := geminiCompleteJSON(r.Authored[path])
		if err != nil || !reflect.DeepEqual(got, want) {
			return geminiCompleteRefusal()
		}
	}
	wantSkill := fmt.Sprintf("---\nname: %s\ndescription: %s\n---\nUAP_GEMINI_%s_%s\n", r.Name, r.Revision, r.Revision, r.Nonce)
	if string(r.Authored["skills/"+r.Name+"/SKILL.md"]) != wantSkill {
		return geminiCompleteRefusal()
	}
	inventory, ok := files.(map[string]any)
	if !ok || len(inventory) != 4 {
		return geminiCompleteRefusal()
	}
	for path, value := range inventory {
		row, err := geminiCompleteObject(value, "bytes sha256 executable", "")
		body, authored := r.Authored[path]
		hash, _ := row["sha256"].(string)
		if err != nil || !authored && path != "bin/probe" || !geminiCycleDigest("sha256:"+hash) || row["executable"] != (path == "bin/probe") || !geminiCompleteInteger(row["bytes"], 0, 32<<20) || authored && (!geminiCompleteIntegerIs(row["bytes"], int64(len(body))) || hash != geminiCycleSHA(body)) {
			return geminiCompleteRefusal()
		}
	}
	return nil
}

func geminiCompleteProofs(body []byte, paths map[string]string) (geminiCompleteProof, error) {
	result := geminiCompleteProof{Views: map[string]map[string]*geminiCycleRecipe{}, Raw: map[string][]byte{}}
	rows, err := geminiCompleteRows(body, geminiCompleteProofOrder, false)
	if err != nil || len(rows) != len(geminiCompleteProofOrder) {
		return result, geminiCompleteRefusal()
	}
	recipes := map[string]*geminiCycleRecipe{}
	inventories := map[string]any{}
	for index, row := range rows {
		if strings.HasPrefix(row.Stage, "source-") {
			r, err := geminiCompleteRecipe(row.Value, paths)
			if err != nil || row.Stage != "source-"+r.Scope+"-"+r.Revision {
				return result, geminiCompleteRefusal()
			}
			for scope, prior := range recipes {
				if scope != r.Scope && prior.Nonce == r.Nonce {
					return result, geminiCompleteRefusal()
				}
			}
			if prior := recipes[r.Scope]; prior != nil {
				if r.Scope != "a" || prior.Revision != "A-one" || r.Revision != "A-two" || prior.Nonce != r.Nonce || prior.Digest == r.Digest || !reflect.DeepEqual(inventories[r.Scope], row.Value["files"].(map[string]any)["bin/probe"]) {
					return result, geminiCompleteRefusal()
				}
				r.Installation, r.Binding, r.Active, r.Data = prior.Installation, prior.Binding, prior.Active, prior.Data
			}
			recipes[r.Scope] = r
			for _, prior := range inventories {
				if !reflect.DeepEqual(prior, row.Value["files"].(map[string]any)["bin/probe"]) {
					return result, geminiCompleteRefusal()
				}
			}
			inventories[r.Scope] = row.Value["files"].(map[string]any)["bin/probe"]
		} else if err := geminiCompleteStateRow(row, recipes, paths, result.Raw); err != nil {
			return result, err
		}
		if index == 2 {
			result.Views[""] = geminiCompleteView(recipes)
		}
		result.Views[row.Stage] = geminiCompleteView(recipes)
	}
	return result, nil
}

func geminiCompleteStateRow(row geminiCompleteRow, recipes map[string]*geminiCycleRecipe, paths map[string]string, raw map[string][]byte) error {
	value, err := geminiCompleteObject(row.Value, "scope revision removed source_digest state binding", "")
	scope, _ := value["scope"].(string)
	r := recipes[scope]
	removed, ok := value["removed"].(bool)
	if err != nil || r == nil || !ok || removed != (strings.HasPrefix(row.Stage, "remove-") || row.Stage == "repeat-remove-a") || value["revision"] != r.Revision || value["source_digest"] != r.Digest || scope != geminiCycleScope(row.Stage) {
		return geminiCompleteRefusal()
	}
	body, err := geminiCompleteStream(value["state"])
	if err != nil {
		return err
	}
	state, err := geminiCycleState(body)
	stateValue, shapeErr := geminiCompleteJSON(body)
	if err != nil || shapeErr != nil || !geminiCompleteEqual(stateValue, state) || len(state.Installations) != 1 || len(state.EnvironmentReceipts) != 0 {
		return geminiCompleteRefusal()
	}
	i := state.Installations[0]
	if err := geminiCycleSource(i, r); err != nil {
		return err
	}
	var wrapper domain.ClientBinding
	if err := geminiCompleteTyped(value["binding"], &wrapper); err != nil || !geminiCompleteEqual(value["binding"], wrapper) {
		return geminiCompleteRefusal()
	}
	previous := r.Binding
	if r.Installation.InstallationID != "" && (r.Installation.InstallationID != i.InstallationID || !reflect.DeepEqual(r.Installation.DataReceipts, i.DataReceipts)) {
		return geminiCompleteRefusal()
	}
	if r.Installation.InstallationID != "" && r.Installation.CreatedAt != i.CreatedAt {
		return geminiCompleteRefusal()
	}
	if prior, err := geminiCompleteTime(r.Installation.UpdatedAt); err == nil {
		current, err := geminiCompleteTime(i.UpdatedAt)
		if err != nil || current.Before(prior) {
			return geminiCompleteRefusal()
		}
	}
	if removed {
		if err := geminiCompleteRemoved(state, previous, wrapper); err != nil {
			return err
		}
	} else {
		if len(state.TransactionReceipts) != 0 {
			return geminiCompleteRefusal()
		}
		if err := geminiCompleteInstalled(row.Stage, r, i, previous, wrapper, paths); err != nil {
			return err
		}
	}
	if err := geminiCompleteData(r, i); err != nil {
		return err
	}
	for other, recipe := range recipes {
		if other != scope && recipe.Installation.InstallationID != "" && (recipe.Installation.InstallationID == i.InstallationID || recipe.Binding.ClientBindingID == r.Binding.ClientBindingID || recipe.Binding.PhysicalArtifact == r.Binding.PhysicalArtifact || recipe.Binding.DataReceiptID == r.Binding.DataReceiptID) {
			return geminiCompleteRefusal()
		}
	}
	if prior := map[string]string{"repeat-add-a": "add-a", "unchanged-update-a": "repeat-add-a", "repeat-remove-a": "remove-a", "before-source-revision-a": "intact-repair-a", "before-missing-a": "update-a"}[row.Stage]; prior != "" {
		if !reflect.DeepEqual(raw[prior], body) {
			return geminiCompleteRefusal()
		}
	}
	r.Installation, r.Removed = i, removed
	raw[row.Stage] = body
	return nil
}

func geminiCompleteInstalled(stage string, r *geminiCycleRecipe, i domain.Installation, previous, wrapper domain.ClientBinding, paths map[string]string) error {
	b, err := geminiCycleBindingIdentity(r, i)
	if err != nil || !reflect.DeepEqual(b, wrapper) {
		return geminiCompleteRefusal()
	}
	if previous.ClientBindingID != "" && (previous.ClientBindingID != b.ClientBindingID || previous.PhysicalArtifact != b.PhysicalArtifact || previous.DataReceiptID != b.DataReceiptID) {
		return geminiCompleteRefusal()
	}
	r.Active, r.Data = b.TargetLocator, filepath.Join(r.StateRoot, "plugin-data", b.PhysicalArtifact)
	want := len(previous.Receipts)
	if strings.HasPrefix(stage, "add-") || stage == "intact-repair-a" || stage == "update-a" || stage == "missing-repair-a" {
		want++
	}
	if len(b.Receipts) != want || want < 1 || !geminiCycleDigest(b.Receipts[want-1].AfterDigest) || i.OperationGroupID != b.Receipts[want-1].OperationGroupID {
		return geminiCompleteRefusal()
	}
	payload := b.Receipts[want-1].AfterDigest
	if err := geminiCycleReceipts(b, previous, payload); err != nil {
		return err
	}
	for _, receipt := range b.Receipts {
		if err := geminiCompleteMutationShape(receipt, false); err != nil {
			return err
		}
	}
	if err := geminiCycleObjects(r, b, payload, geminiCompleteEntry(r, paths)); err != nil {
		return err
	}
	r.Binding = b
	return nil
}

func geminiCompleteRemoved(state domain.StateFileV2, previous, wrapper domain.ClientBinding) error {
	i := state.Installations[0]
	if previous.ClientBindingID == "" || !reflect.DeepEqual(wrapper, previous) || len(i.Clients) != 0 || !i.DataRetained || len(state.TransactionReceipts) != 1 {
		return geminiCompleteRefusal()
	}
	r := state.TransactionReceipts[0]
	if err := geminiCompleteMutationShape(r, true); err != nil {
		return err
	}
	if r.ClientBindingID != previous.ClientBindingID || r.ActivePath != previous.TargetLocator || r.MutationType != "directory_remove" || r.Phase != "committed" || r.AfterDigest != "" || r.OperationID != r.OperationGroupID+"-001" || r.OperationGroupID == "" || i.OperationGroupID != r.OperationGroupID || r.Sequence != len(previous.Receipts)+1 || r.EnvironmentAuthority != nil || r.BeforeDigest != previous.Receipts[len(previous.Receipts)-1].AfterDigest || r.StagingPath != "" || r.BackupPath != filepath.Join(filepath.Dir(previous.TargetLocator), ".agentplugins-backup-"+geminiCycleSHA([]byte(r.OperationID))[:16]) {
		return geminiCompleteRefusal()
	}
	return nil
}

func geminiCompleteMutationShape(receipt domain.MutationReceipt, removed bool) error {
	body, err := json.Marshal(receipt)
	row, decodeErr := geminiCompleteJSON(body)
	required := "operation_id operation_group_id sequence mutation_type client_binding_id active_path staging_path backup_path after_digest phase"
	optional := "before_digest"
	if removed {
		required = "operation_id operation_group_id sequence mutation_type client_binding_id active_path backup_path before_digest phase"
		optional = ""
	}
	if err != nil || decodeErr != nil || projectNativeRuntimeKeys(row, required, optional) != nil {
		return geminiCompleteRefusal()
	}
	return nil
}

func geminiCompleteData(r *geminiCycleRecipe, i domain.Installation) error {
	b := r.Binding
	id := "data_" + geminiCycleSHA([]byte(strings.Join([]string{i.InstallationID, b.PhysicalArtifact, r.scopeName()}, "\x00")))[:24]
	d, ok := i.DataReceipts[id]
	if !ok || len(i.DataReceipts) != 1 || b.DataReceiptID != id || d.DataReceiptID != id || d.PhysicalBackend != b.PhysicalArtifact || d.Scope != r.scopeName() || d.Locator != r.Data || d.State != domain.DataReceiptOwned || d.OwnershipDigest != "sha256:"+geminiCycleSHA([]byte("agentplugins-plugin-data-v1\x00"+id+"\x00"+r.Data)) {
		return geminiCompleteRefusal()
	}
	return nil
}

func geminiCompleteEntry(r *geminiCycleRecipe, paths map[string]string) map[string]any {
	env := map[string]any{"PLUGIN_ROOT": r.Active, "PLUGIN_DATA": r.Data, "UAP_TEST_ROOT": paths["root"], "UAP_TEST_EVENTS": paths["root"] + "/events.jsonl", "UAP_TEST_NONCE": r.Nonce, "UAP_TEST_REVISION": r.Revision, "UAP_TEST_LITERAL": "literal ${UNKNOWN} $HOME", "UAP_TEST_ONCE": r.Active + "/${UNKNOWN}"}
	return map[string]any{"command": r.Active + "/bin/probe", "cwd": r.Active, "args": []any{"argument with spaces", r.Active + "/argument", "${UNKNOWN}"}, "env": env}
}
