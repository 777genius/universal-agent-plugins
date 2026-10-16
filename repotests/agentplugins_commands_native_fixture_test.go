package pluginkitairepo_test

// Shared fixture and lifecycle for the native portable-command suites. The
// fixture carries one ordinary command (echo) and one explicit-only command
// (wizard). No client declares explicit-invocation enforcement yet, so every
// client must receive echo as a skill and must not receive wizard at all.
// The suites prove delivery and lifecycle, never model behavior.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/loader"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/all"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/commandext"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
)

const (
	commandsPluginName   = "uap-commands-fixture"
	commandsGuideSkill   = "fixture-guide"
	commandsDirectory    = "io.github.777genius.agentplugins/commands"
	commandsWithheldCode = "explicit_invocation_not_enforceable"
	commandsEvidenceFile = "commands-evidence.json"
)

// commandsSkillBytes are the exact SKILL.md bytes the installer renders for
// each command while it is an ordinary command. The admission test ties them
// to the renderer; the native suites compare installed files against them.
var commandsSkillBytes = map[string]string{
	"echo":   "---\nname: \"uap-commands-fixture-echo\"\ndescription: \"Reply with a fixed token and repeat the text after the command.\"\n---\n\nReply with the exact token UAP-ECHO-5C1D, then repeat the text the user wrote after the command.\n",
	"wizard": "---\nname: \"uap-commands-fixture-wizard\"\ndescription: \"Run the fixed setup check only when the user asks for it.\"\n---\n\nReply with the exact token UAP-WIZARD-8E27. Do not run tools and do not change files.\n",
}

// commandsSkill is the skill a fixture command becomes: plugin name, a dash,
// command name.
func commandsSkill(command string) string {
	return commandsPluginName + "-" + command
}

func commandsFixtureSource(t *testing.T) string {
	t.Helper()
	return filepath.Join(RepoRoot(t), "repotests", "testdata", "agentplugins_commands_fixture")
}

// commandsSwapInvocation makes echo explicit-only and wizard ordinary in a
// package copy, so the next update has to withdraw one command skill and
// deliver the other.
func commandsSwapInvocation(t *testing.T, pkg string) {
	t.Helper()
	for command, flags := range map[string][2]string{"echo": {"false", "true"}, "wizard": {"true", "false"}} {
		path := filepath.Join(pkg, filepath.FromSlash(commandsDirectory), command+".md")
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		old := []byte("disable-model-invocation: " + flags[0] + "\n")
		if bytes.Count(body, old) != 1 {
			t.Fatalf("%s does not declare %q exactly once", path, old)
		}
		nativeWrite(t, path, bytes.Replace(body, old, []byte("disable-model-invocation: "+flags[1]+"\n"), 1), 0o644)
	}
}

// commandsTargetOutcome returns the result object of the single target in
// an installer JSON result.
func commandsTargetOutcome(result map[string]any) (map[string]any, error) {
	data, _ := result["data"].(map[string]any)
	targets, _ := data["targets"].([]any)
	if len(targets) != 1 {
		return nil, fmt.Errorf("expected exactly one target in installer result: %v", result)
	}
	target, _ := targets[0].(map[string]any)
	output, _ := target["output"].(map[string]any)
	outcome, ok := output["result"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("installer result has no target result: %v", target)
	}
	return outcome, nil
}

// commandsPlanSelection checks the installer's own plan: the delivered
// command is the one skill rendered from a command, the withheld command is
// no component at all, and exactly one command diagnostic, a warning, names
// it.
func commandsPlanSelection(result map[string]any, delivered, withheld string) error {
	outcome, err := commandsTargetOutcome(result)
	if err != nil {
		return err
	}
	plan, _ := outcome["plan"].(map[string]any)
	components, _ := plan["components"].([]any)
	var rendered []string
	for _, raw := range components {
		component, _ := raw.(map[string]any)
		if component["name"] == commandsSkill(withheld) || component["source_name"] == withheld {
			return fmt.Errorf("withheld command %q is a planned component: %v", withheld, component)
		}
		if component["source"] == nil {
			continue
		}
		if component["source"] != domain.ComponentSourceCommand || component["kind"] != string(domain.ComponentSkill) || component["support"] == string(domain.SupportUnsupported) {
			return fmt.Errorf("rendered component is not a supported command skill: %v", component)
		}
		rendered = append(rendered, fmt.Sprint(component["source_name"], "=", component["name"]))
	}
	if want := []string{delivered + "=" + commandsSkill(delivered)}; !slices.Equal(rendered, want) {
		return fmt.Errorf("command components = %v, want %v", rendered, want)
	}
	diagnostics, _ := plan["diagnostics"].([]any)
	var commandDiagnostics []string
	for _, raw := range diagnostics {
		diagnostic, _ := raw.(map[string]any)
		if diagnostic["boundary"] == string(domain.BoundaryCommand) {
			commandDiagnostics = append(commandDiagnostics, fmt.Sprint(diagnostic["severity"], ":", diagnostic["code"], ":", diagnostic["item"]))
		}
	}
	if want := []string{"warning:" + commandsWithheldCode + ":" + withheld}; !slices.Equal(commandDiagnostics, want) {
		return fmt.Errorf("command diagnostics = %v, want %v", commandDiagnostics, want)
	}
	warnings, _ := plan["warnings"].([]any)
	if !slices.Contains(warnings, any(commandsWithheldCode)) {
		return fmt.Errorf("plan warnings %v omit %s", warnings, commandsWithheldCode)
	}
	return nil
}

// commandsSortedKeys keeps the module's Go 1.22 language level, which has no
// map iterators.
func commandsSortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// commandsCheckSkills compares what a client itself discovered, skill name
// to the SKILL.md it reads, with the fixture's guide skill plus exactly one
// command skill, and requires that skill's exact rendered bytes.
func commandsCheckSkills(discovered map[string]string, command string) error {
	want := []string{commandsGuideSkill, commandsSkill(command)}
	if got := commandsSortedKeys(discovered); !slices.Equal(got, want) {
		return fmt.Errorf("client discovered skills %v, want %v", got, want)
	}
	body, err := os.ReadFile(discovered[commandsSkill(command)])
	if err != nil {
		return err
	}
	if string(body) != commandsSkillBytes[command] {
		return fmt.Errorf("installed %s differs from the rendered command:\n%q\nwant\n%q", commandsSkill(command), body, commandsSkillBytes[command])
	}
	return nil
}

// commandsRunInstaller runs one installer command in a suite's isolated
// environment, keeps stdout and stderr as evidence and decodes the JSON
// result. The command error is returned for the caller to judge.
func commandsRunInstaller(t *testing.T, root, dir string, env []string, installer, label string, args ...string) (map[string]any, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, installer, append(args, "--format", "json")...)
	cmd.Env, cmd.Dir = env, dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.Output()
	nativeWrite(t, filepath.Join(root, label+"-installer.json"), stdout, 0o600)
	nativeWrite(t, filepath.Join(root, label+"-installer-stderr.log"), stderr.Bytes(), 0o600)
	if ctx.Err() != nil {
		t.Fatalf("%s timed out:\n%s", label, stderr.Bytes())
	}
	var result map[string]any
	if len(bytes.TrimSpace(stdout)) > 0 {
		if decodeErr := json.Unmarshal(stdout, &result); decodeErr != nil {
			t.Fatalf("%s: decode installer JSON: %v\n%s", label, decodeErr, stdout)
		}
	}
	return result, err
}

// commandsNativeClient is what one client suite supplies to the shared
// command lifecycle.
type commandsNativeClient struct {
	target    string
	pkg       string
	statePath string
	// run executes one installer command; see commandsRunInstaller.
	run func(t *testing.T, label string, args ...string) (map[string]any, error)
	// skills reports the plugin's skills as the client itself discovers
	// them: skill name to the SKILL.md path the client reads. It is empty
	// once the plugin is gone.
	skills func(t *testing.T, label string) map[string]string
	// dropManaged deletes the managed artifact recorded for the installation
	// and returns its path, so repair has to rebuild it.
	dropManaged func(t *testing.T, latest map[string]any) string
	removeArgs  []string
}

func (c commandsNativeClient) succeed(t *testing.T, label string, args ...string) map[string]any {
	t.Helper()
	result, err := c.run(t, label, append(args, "--target", c.target)...)
	if err != nil || result["result"] != "success" {
		t.Fatalf("%s failed: %v; %v", label, err, result)
	}
	return result
}

func (c commandsNativeClient) complete(t *testing.T, label string, args ...string) map[string]any {
	t.Helper()
	result := c.succeed(t, label, args...)
	if data, _ := result["data"].(map[string]any); data["status"] != "completed" {
		t.Fatalf("%s did not complete: %v", label, result)
	}
	return result
}

func (c commandsNativeClient) requireSkills(t *testing.T, label, command string) {
	t.Helper()
	if err := commandsCheckSkills(c.skills(t, label), command); err != nil {
		t.Fatalf("%s: %v", label, err)
	}
}

func commandsRequireMutated(t *testing.T, label string, result map[string]any, want bool) {
	t.Helper()
	outcome, err := commandsTargetOutcome(result)
	if err != nil {
		t.Fatal(err)
	}
	if mutated, ok := outcome["mutated"].(bool); !ok || mutated != want {
		t.Fatalf("%s: mutated = %v, want %v", label, outcome["mutated"], want)
	}
}

func commandsStages() map[string]nativeStage {
	stages := map[string]nativeStage{}
	for _, name := range []string{"install", "command_skill_discovery", "explicit_command_withheld", "unchanged_update", "invocation_change_update", "intact_repair", "absent_repair", "remove"} {
		stages[name] = nativeStage{Status: "not_evaluated", Reason: "not reached"}
	}
	stages["explicit_invocation_enforced"] = nativeStage{Status: "not_evaluated", Reason: "no client declares explicit-invocation enforcement yet, so explicit-only commands are withheld"}
	stages["real_model_command_use"] = nativeStage{Status: "not_evaluated", Reason: "no model, login or OAuth is used"}
	return stages
}

// commandsNativeLifecycle drives one client through the command lifecycle.
// Each stage is marked failed before its assertions and passed after them,
// so an early stop leaves honest evidence.
func commandsNativeLifecycle(t *testing.T, c commandsNativeClient, stages map[string]nativeStage) {
	t.Helper()
	copyTree(t, commandsFixtureSource(t), c.pkg)

	nativeAttempt(stages, "install", "add-installer.json")
	added := c.complete(t, "add", "add", c.pkg)
	if err := commandsPlanSelection(added, "echo", "wizard"); err != nil {
		t.Fatal(err)
	}
	stages["install"] = nativeStage{Status: "passed", Artifacts: []string{"add-installer.json"}}
	nativeAttempt(stages, "command_skill_discovery", "add-installer.json")
	c.requireSkills(t, "skills-after-add", "echo")
	stages["command_skill_discovery"] = nativeStage{Status: "passed", Reason: "the client itself lists the ordinary command as a plugin skill with the exact rendered bytes"}
	stages["explicit_command_withheld"] = nativeStage{Status: "passed", Reason: "the explicit-only command is no planned component, carries an explicit_invocation_not_enforceable warning and is absent from the client's own skill listing"}

	nativeAttempt(stages, "unchanged_update", "update-unchanged-installer.json")
	stateBefore := nativeSHA(t, c.statePath)
	unchanged := c.complete(t, "update-unchanged", "update", commandsPluginName)
	commandsRequireMutated(t, "unchanged update", unchanged, false)
	if nativeSHA(t, c.statePath) != stateBefore {
		t.Fatal("unchanged update rewrote installer state")
	}
	c.requireSkills(t, "skills-after-unchanged-update", "echo")
	stages["unchanged_update"] = nativeStage{Status: "passed", Reason: "update of the unchanged package: mutated=false, byte-identical installer state, same client skills and bytes"}

	nativeAttempt(stages, "invocation_change_update", "update-invocation-change-installer.json")
	commandsSwapInvocation(t, c.pkg)
	swapped := c.complete(t, "update-invocation-change", "update", commandsPluginName)
	commandsRequireMutated(t, "invocation change update", swapped, true)
	if err := commandsPlanSelection(swapped, "wizard", "echo"); err != nil {
		t.Fatal(err)
	}
	c.requireSkills(t, "skills-after-invocation-change", "wizard")
	stages["invocation_change_update"] = nativeStage{Status: "passed", Reason: "after the commands swapped invocation flags, update delivered the newly ordinary command and the client no longer lists the newly explicit-only one"}

	nativeAttempt(stages, "intact_repair", "repair-intact-installer.json")
	c.complete(t, "repair-intact", "repair", commandsPluginName)
	c.requireSkills(t, "skills-after-intact-repair", "wizard")
	stages["intact_repair"] = nativeStage{Status: "passed", Reason: "repair of an intact package with current command rules succeeded and left the client skills unchanged"}

	nativeAttempt(stages, "absent_repair", "repair-absent-installer.json")
	managed := c.dropManaged(t, swapped)
	c.complete(t, "repair-absent", "repair", commandsPluginName)
	if _, err := os.Stat(managed); err != nil {
		t.Fatalf("repair did not rebuild the managed artifact: %v", err)
	}
	c.requireSkills(t, "skills-after-absent-repair", "wizard")
	stages["absent_repair"] = nativeStage{Status: "passed", Reason: "the deleted managed artifact was rebuilt with the rendered command skill"}

	nativeAttempt(stages, "remove", "remove-installer.json")
	c.succeed(t, "remove", append([]string{"remove", commandsPluginName}, c.removeArgs...)...)
	if remaining := c.skills(t, "skills-after-remove"); len(remaining) != 0 {
		t.Fatalf("client still lists plugin skills after remove: %v", remaining)
	}
	stages["remove"] = nativeStage{Status: "passed", Reason: "the client lists no plugin skill after remove"}
}

// commandsWriteEvidence records stages and transcript digests under a name
// the released-evidence verifier does not read as a lifecycle fixture, then
// fails the test for any stage that started and did not pass.
func commandsWriteEvidence(t *testing.T, root string, evidence map[string]any, stages map[string]nativeStage) {
	t.Helper()
	hashes := map[string]string{}
	entries, _ := os.ReadDir(root)
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() && name != commandsEvidenceFile && (strings.HasSuffix(name, ".log") || strings.HasSuffix(name, ".json") || strings.HasSuffix(name, ".jsonl")) {
			hashes[name] = nativeSHA(t, filepath.Join(root, name))
		}
	}
	evidence["finished_utc"] = time.Now().UTC().Format(time.RFC3339Nano)
	evidence["os"], evidence["arch"] = runtime.GOOS, runtime.GOARCH
	evidence["stages"] = stages
	evidence["artifact_sha256"] = hashes
	nativeJSON(t, filepath.Join(root, commandsEvidenceFile), evidence)
	if err := nativeStageFailures(stages); err != nil {
		t.Error(err)
	}
}

func commandsLoad(t *testing.T, root string) domain.PackageEnvelope {
	t.Helper()
	registry, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := (loader.Loader{Registry: registry}).Load(context.Background(), domain.LoadInput{SnapshotRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	return envelope
}

func commandsRequireGolden(t *testing.T, envelope domain.PackageEnvelope, command string) {
	t.Helper()
	files, err := commandext.Files(context.Background(), envelope.Commands[command], envelope.Manifest.Name, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || string(files[commandext.SkillFile]) != commandsSkillBytes[command] {
		t.Fatalf("rendered %s = %q, want only SKILL.md %q", command, files, commandsSkillBytes[command])
	}
}

// TestAgentpluginsCommandsFixtureAdmission binds the native command suites
// to the installer core without any client: the fixture loads with both
// commands and no command diagnostic, every client of the native matrix
// reports one delivered and one withheld command row, and the expected
// skill bytes are what the renderer writes before and after the flag swap.
func TestAgentpluginsCommandsFixtureAdmission(t *testing.T) {
	root := t.TempDir()
	copyTree(t, commandsFixtureSource(t), root)
	envelope := commandsLoad(t, root)
	if got := commandsSortedKeys(envelope.Commands); !slices.Equal(got, []string{"echo", "wizard"}) || len(envelope.InvalidCommands) != 0 || !envelope.Commands["wizard"].ExplicitOnly || envelope.Commands["echo"].ExplicitOnly {
		t.Fatalf("fixture commands = %v, invalid = %v", envelope.Commands, envelope.InvalidCommands)
	}
	for _, diagnostic := range envelope.Diagnostics {
		if diagnostic.Severity == domain.SeverityError || diagnostic.Boundary == domain.BoundaryCommand {
			t.Fatalf("fixture diagnostic: %+v", diagnostic)
		}
	}
	commandsRequireGolden(t, envelope, "echo")
	targets := []domain.ClientID{domain.ClientClaude, domain.ClientCodex, domain.ClientOpenCode}
	compatibility, err := planner.Compatibility(all.Default(), envelope, targets)
	if err != nil {
		t.Fatal(err)
	}
	for _, client := range compatibility {
		delivered, withheld := 0, 0
		for _, row := range client.Components {
			switch {
			case row.Source != domain.ComponentSourceCommand:
			case row.Support == domain.SupportUnsupported && slices.Equal(row.Limitations, []string{commandsWithheldCode}):
				withheld++
			case row.Support != domain.SupportUnsupported && len(row.Limitations) == 0:
				delivered++
			default:
				t.Fatalf("%s: unexpected command row %+v", client.ClientID, row)
			}
		}
		if delivered != 1 || withheld != 1 {
			t.Fatalf("%s: delivered %d and withheld %d command rows, want 1 and 1", client.ClientID, delivered, withheld)
		}
	}
	commandsSwapInvocation(t, root)
	swapped := commandsLoad(t, root)
	if !swapped.Commands["echo"].ExplicitOnly || swapped.Commands["wizard"].ExplicitOnly {
		t.Fatalf("swapped commands = %v", swapped.Commands)
	}
	commandsRequireGolden(t, swapped, "wizard")
}

func TestCommandsPlanSelection(t *testing.T) {
	const fixture = `{"data":{"targets":[{"output":{"result":{"plan":{"components":[{"kind":"skill","name":"fixture-guide","support":"projected"},{"kind":"skill","name":"uap-commands-fixture-echo","support":"projected","source":"command","source_name":"echo"}],"diagnostics":[{"severity":"warning","boundary":"command","code":"explicit_invocation_not_enforceable","item":"wizard"}],"warnings":["explicit_invocation_not_enforceable"]}}}}]}}`
	for _, tc := range []struct {
		name, input string
		wantError   bool
	}{
		{"exact", fixture, false},
		{"withheld planned", strings.Replace(fixture, `"name":"fixture-guide","support":"projected"`, `"name":"uap-commands-fixture-wizard","support":"projected"`, 1), true},
		{"command unsupported", strings.Replace(fixture, `"support":"projected","source"`, `"support":"unsupported","source"`, 1), true},
		{"wrong source name", strings.Replace(fixture, `"source_name":"echo"`, `"source_name":"wizard"`, 1), true},
		{"delivered missing", strings.Replace(fixture, `"source":"command","source_name":"echo"`, `"source_name":"echo"`, 1), true},
		{"warning missing", strings.Replace(fixture, `"warnings":["explicit_invocation_not_enforceable"]`, `"warnings":[]`, 1), true},
		{"diagnostic is an error", strings.Replace(fixture, `"severity":"warning"`, `"severity":"error"`, 1), true},
		{"wrong item", strings.Replace(fixture, `"item":"wizard"`, `"item":"echo"`, 1), true},
		{"two targets", strings.Replace(fixture, `"targets":[{`, `"targets":[{},{`, 1), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var result map[string]any
			if err := json.Unmarshal([]byte(tc.input), &result); err != nil {
				t.Fatal(err)
			}
			if err := commandsPlanSelection(result, "echo", "wizard"); (err != nil) != tc.wantError {
				t.Fatalf("selection error = %v, wantError = %v", err, tc.wantError)
			}
		})
	}
}
