package cline

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type clineProjectSkillProof struct {
	objects        []domain.NativeObjectOwnership
	components     []domain.ComponentDecision
	authentication domain.AuthenticationState
	skills         map[string]clineObservedSkill
}

type clineObservedSkill struct {
	name, path, description, instructions string
	frontmatter                           []byte
}

// This ordinary catalog reads inert Skills from the selected working directory.
// It does not start a task, MCP server, model turn, or IDE session.
func observeClineProjectSkills(ctx context.Context, env clients.Env, request domain.ActivationRequest, out domain.ActivationOutcome) (domain.ActivationOutcome, error) {
	if err := ctx.Err(); err != nil {
		return out, err
	}
	if env.Runner == nil || env.ParseSkill == nil || env.Snapshots == nil || env.DecodeSkillFrontmatter == nil || !clineProjectSkillAuthority(request, out) {
		return out, nil
	}
	executable, observable := clineProjectSkillExecutable(env, request)
	if !observable {
		return out, nil
	}
	proof := captureClineProjectSkills(ctx, env, request)
	if proof == nil || ctx.Err() != nil {
		return out, ctx.Err()
	}
	result, err := runClineProjectSkills(ctx, env, executable, request.Plan.TargetContext.ProjectRoot)
	return finishClineProjectSkills(ctx, env, request, out, proof, result, err)
}

// Missing or ambiguous CLI authority leaves optional discovery unobserved.
func clineProjectSkillExecutable(env clients.Env, request domain.ActivationRequest) (string, bool) {
	executable, err := clineSessionExecutable(env, request.Client, len(request.Plan.Components))
	return executable, err == nil && filepath.Clean(executable) == executable && !strings.ContainsAny(executable, "\x00\r\n") &&
		(request.BackendExecutable == "" || request.BackendExecutable == executable)
}

func finishClineProjectSkills(ctx context.Context, env clients.Env, request domain.ActivationRequest, out domain.ActivationOutcome, proof *clineProjectSkillProof, result legacyports.CommandResult, err error) (domain.ActivationOutcome, error) {
	if !result.CleanupComplete && !result.NoChildStarted && err == nil {
		err = &domain.UnclosedClientProbe{Operation: "Cline Skill catalog", Err: errors.New("runner did not confirm process cleanup")}
	}
	if domain.IsUnclosedClientProbe(err) {
		out.NativeEffect = domain.NativeEffectUncertain
		return shared.FailedActivation(out, "confirm prior Cline catalog process cleanup before retrying", err)
	}
	if ctx.Err() != nil {
		return out, ctx.Err()
	}
	fresh := captureClineProjectSkills(ctx, env, request)
	if ctx.Err() != nil {
		return out, ctx.Err()
	}
	if fresh == nil || !slices.Equal(proof.objects, fresh.objects) || !reflect.DeepEqual(proof.skills, fresh.skills) {
		out.NativeEffect = domain.NativeEffectUncertain
		return shared.FailedActivation(out, "repair exact owned Cline project Skills before retrying",
			errors.Join(err, errors.New("owned Cline project Skill source, receipts or native bytes changed during catalog observation")))
	}
	if !clineProjectSkillQueryMatches(result, err, proof.skills) {
		return out, nil
	}
	if !clineProjectSkillAuthority(request, out) || !reflect.DeepEqual(proof, fresh) {
		return out, nil
	}
	out.Activation = domain.ActivationActive
	if out.Authentication == domain.AuthenticationNotChecked {
		out.Authentication = proof.authentication
	}
	kept := make([]string, 0, len(out.UserActions))
	for _, action := range out.UserActions {
		if action != clineProjectAction(request.Plan) {
			kept = append(kept, action)
		}
	}
	out.UserActions = kept
	return out, nil
}

func clineProjectSkillQueryMatches(result legacyports.CommandResult, err error, expected map[string]clineObservedSkill) bool {
	return err == nil && result.ExitCode == 0 && !result.NoChildStarted && result.CleanupComplete &&
		len(result.Stdout) <= 1<<20 && len(result.Stderr) == 0 && clineProjectSkillsListed(result.Stdout, expected)
}

func clineProjectSkillAuthority(request domain.ActivationRequest, out domain.ActivationOutcome) bool {
	if request.ActivationComplete || out.ActivationAttested || out.AuthenticationAttested ||
		request.Plan.InstallIntent != domain.InstallIntentAutomatic || request.Plan.Status != domain.PlanReady ||
		request.Delivery.ClientID != domain.ClientCline || request.Delivery.ActivePath != request.Plan.ActivePath ||
		request.Delivery.OwnedBase != request.Plan.TargetRoot || request.DeclaredName != request.Plan.DeclaredName ||
		request.Plan.Policy != domain.PolicyAllowed || out.Policy != domain.PolicyAllowed ||
		out.Verification != domain.VerificationInstalled || out.Activation != domain.ActivationPrepared ||
		(out.NativeEffect != domain.NativeEffectCommitted && out.NativeEffect != domain.NativeEffectUnchanged) ||
		!slices.Equal(out.NativeObjects, request.Delivery.NativeObjects) || len(request.Plan.Components) == 0 {
		return false
	}
	if _, err := clineProjectPlanRoot(request.Client, request.Plan); err != nil {
		return false
	}
	return clineProjectSkillComponents(request.Plan.Components)
}

func clineProjectSkillComponents(components []domain.ComponentDecision) bool {
	seen := map[string]bool{}
	for _, component := range components {
		if component.Kind != domain.ComponentSkill || component.Name == "" || seen[shared.FoldJSONKey(component.Name)] ||
			(component.Support != domain.SupportNative && component.Support != domain.SupportProjected) {
			return false
		}
		switch component.Source {
		case "":
			if component.SourceName != "" {
				return false
			}
		case domain.ComponentSourceCommand:
			if component.SourceName == "" {
				return false
			}
		default:
			return false
		}
		seen[shared.FoldJSONKey(component.Name)] = true
	}
	return true
}

// Source and installed component directories must match the same immutable
// receipt both before and after the native query, including resource bytes.
func captureClineProjectSkills(ctx context.Context, env clients.Env, request domain.ActivationRequest) *clineProjectSkillProof {
	if validateClineProjectSelection(request.Plan, request.Delivery.NativeObjects) != nil {
		return nil
	}
	layout := clineProjectComponents(request.Plan.TargetContext.ProjectRoot)
	objects := clineProjectOwned(request.Delivery.NativeObjects)
	if layout.Verify(objects, false, env.NativeConfig) != nil {
		return nil
	}
	proof := &clineProjectSkillProof{objects: slices.Clone(request.Delivery.NativeObjects),
		components: slices.Clone(request.Plan.Components), authentication: shared.ObservedSkillInstallationAuthentication(request.Plan),
		skills: make(map[string]clineObservedSkill, len(objects))}
	for _, object := range objects {
		if object.Kind != clineProjectSkillKind || object.UserModified || object.ProtectionClass != "managed" {
			return nil
		}
		source, err := layout.SkillObject(object.LogicalName, object.SourceRelative, request.Delivery.ActivePath)
		if err != nil || !reflect.DeepEqual(source, object) {
			return nil
		}
		path := filepath.Join(object.Path, "SKILL.md")
		body, err := shared.ReadOwnedSkillMetadata(ctx, env.NativeConfig, env.Snapshots, object.Path, object.ManagedDigest)
		if err != nil {
			return nil
		}
		skill, ok := clineProjectSkillMetadata(ctx, env, object.LogicalName, path, body)
		if !ok {
			return nil
		}
		proof.skills[object.LogicalName] = skill
	}
	return proof
}

func clineProjectSkillMetadata(ctx context.Context, env clients.Env, name, path string, body []byte) (clineObservedSkill, bool) {
	skill, err := env.ParseSkill(ctx, name, body)
	if err != nil {
		return clineObservedSkill{}, false
	}
	values, instructions, err := env.DecodeSkillFrontmatter(ctx, body)
	if err != nil {
		return clineObservedSkill{}, false
	}
	metadata, err := json.Marshal(values)
	return clineObservedSkill{name: name, path: path, description: skill.Description,
		instructions: strings.TrimSpace(string(instructions)), frontmatter: metadata}, err == nil
}

func runClineProjectSkills(ctx context.Context, env clients.Env, executable, project string) (legacyports.CommandResult, error) {
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	// No provider, credential, MCP source, or approval override is needed by
	// config skills. Keep only ordinary process/profile plumbing.
	environment := []string{"CLINE_NO_AUTO_UPDATE=1"}
	for _, key := range []string{"PATH", "HOME", "USERPROFILE", "SYSTEMROOT", "WINDIR", "TEMP", "TMP", "TMPDIR", "LANG", "LC_ALL",
		"XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME", "XDG_DATA_HOME"} {
		if value, ok := os.LookupEnv(key); ok {
			environment = append(environment, key+"="+value)
		}
	}
	return shared.RunNativeRegistry(bounded, env.Runner, legacyports.Command{
		Argv: []string{executable, "--auto-approve", "false", "--cwd", project, "config", "skills", "--json"},
		Dir:  project, Env: environment, StdoutLimitBytes: 1 << 20,
	})
}

// Accept only the measured ordinary array schema. Foreign names may coexist;
// a namesake or second path cannot substitute for an exact owned Skill.
func clineProjectSkillsListed(body []byte, expected map[string]clineObservedSkill) bool {
	if len(body) > 1<<20 || !utf8.Valid(body) {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	value, err := shared.DecodeUniqueJSONValue(decoder)
	if err != nil {
		return false
	}
	if _, err := decoder.Token(); err != io.EOF {
		return false
	}
	rows, ok := value.([]any)
	if !ok || len(expected) == 0 {
		return false
	}
	names, paths := map[string]bool{}, map[string]bool{}
	found := 0
	for _, value := range rows {
		row, ok := value.(map[string]any)
		if !ok || len(row) != 5 || !clineSkillCatalogRow(row) {
			return false
		}
		name, path := row["name"].(string), row["path"].(string)
		key := shared.FoldJSONKey(name)
		if names[key] || paths[path] {
			return false
		}
		names[key], paths[path] = true, true
		if skill, selected := expected[name]; selected {
			metadata, err := json.Marshal(row["frontmatter"])
			if err != nil || path != skill.path || row["description"] != skill.description ||
				row["instructions"] != skill.instructions || !bytes.Equal(metadata, skill.frontmatter) {
				return false
			}
			found++
		}
	}
	return found == len(expected)
}

func clineSkillCatalogRow(row map[string]any) bool {
	for _, key := range []string{"name", "path", "description", "instructions"} {
		if _, ok := row[key].(string); !ok {
			return false
		}
	}
	name, path := row["name"].(string), row["path"].(string)
	front, ok := row["frontmatter"].(map[string]any)
	return ok && name != "" && front["name"] == name && front["description"] == row["description"] &&
		filepath.IsAbs(path) && filepath.Clean(path) == path && filepath.Base(path) == "SKILL.md" &&
		!strings.ContainsAny(path, "\x00\r\n")
}
