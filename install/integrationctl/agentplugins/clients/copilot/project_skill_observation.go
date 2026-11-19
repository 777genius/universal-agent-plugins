package copilot

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

type copilotProjectSkillProof struct {
	objects        []domain.NativeObjectOwnership
	components     []domain.ComponentDecision
	authentication domain.AuthenticationState
	skills         map[string]copilotObservedSkill
	trust          shared.NativeFileSnapshot
	activeDigest   string
}

type copilotObservedSkill struct {
	name, path, description string
	source                  string
}

// This ordinary catalog reads inert Skills from the selected working directory.
// It does not start a task, MCP server, model turn, or IDE session.
func observeCopilotProjectSkills(ctx context.Context, env clients.Env, request domain.ActivationRequest, out domain.ActivationOutcome) (domain.ActivationOutcome, error) {
	if err := ctx.Err(); err != nil {
		return out, err
	}
	if env.Runner == nil || env.ParseSkill == nil || env.Snapshots == nil || !copilotProjectSkillAuthority(request, out) {
		return out, nil
	}
	executable := request.Client.ExecutablePath
	if !copilotProjectSkillCLI(request.Client) || filepath.Clean(executable) != executable || strings.ContainsAny(executable, "\x00\r\n") ||
		(request.BackendExecutable != "" && request.BackendExecutable != executable) {
		return out, nil
	}
	proof := captureCopilotProjectSkills(ctx, env, request)
	if proof == nil || ctx.Err() != nil {
		return out, ctx.Err()
	}
	result, err := runCopilotProjectSkills(ctx, env, executable, request.Plan.TargetContext)
	return finishCopilotProjectSkills(ctx, env, request, out, proof, result, err)
}

func finishCopilotProjectSkills(ctx context.Context, env clients.Env, request domain.ActivationRequest, out domain.ActivationOutcome, proof *copilotProjectSkillProof, result ports.CommandResult, err error) (domain.ActivationOutcome, error) {
	if !result.CleanupComplete && !result.NoChildStarted && err == nil {
		err = &domain.UnclosedClientProbe{Operation: "Copilot Skill catalog", Err: errors.New("runner did not confirm process cleanup")}
	}
	if domain.IsUnclosedClientProbe(err) {
		out.NativeEffect = domain.NativeEffectUncertain
		return shared.FailedActivation(out, "confirm prior Copilot catalog process cleanup before retrying", err)
	}
	if ctx.Err() != nil {
		return out, ctx.Err()
	}
	fresh := captureCopilotProjectSkills(ctx, env, request)
	if ctx.Err() != nil {
		return out, ctx.Err()
	}
	if copilotProjectSkillCustodyChanged(proof, fresh) {
		out.NativeEffect = domain.NativeEffectUncertain
		return shared.FailedActivation(out, "repair exact owned Copilot project Skills before retrying",
			errors.Join(err, errors.New("owned Copilot project Skill source, receipts or native bytes changed during catalog observation")))
	}
	if copilotProjectSkillCatalogFailed(result, err, proof.skills) {
		return out, nil
	}
	if !copilotProjectSkillAuthority(request, out) || !reflect.DeepEqual(proof, fresh) {
		return out, nil
	}
	out.Activation = domain.ActivationActive
	if out.Authentication == domain.AuthenticationNotChecked {
		out.Authentication = proof.authentication
	}
	kept := make([]string, 0, len(out.UserActions))
	for _, action := range out.UserActions {
		if action != copilotProjectInstalledAction {
			kept = append(kept, action)
		}
	}
	out.UserActions = kept
	return out, nil
}

func copilotProjectSkillCustodyChanged(proof, fresh *copilotProjectSkillProof) bool {
	return fresh == nil || !slices.Equal(proof.objects, fresh.objects) || !reflect.DeepEqual(proof.skills, fresh.skills) || !reflect.DeepEqual(proof.trust, fresh.trust) || proof.activeDigest != fresh.activeDigest
}

func copilotProjectSkillCatalogFailed(result ports.CommandResult, err error, skills map[string]copilotObservedSkill) bool {
	return err != nil || result.ExitCode != 0 || result.NoChildStarted || !result.CleanupComplete ||
		len(result.Stdout) > 1<<20 || len(result.Stderr) != 0 || !copilotProjectSkillsListed(result.Stdout, skills)
}

func copilotProjectSkillAuthority(request domain.ActivationRequest, out domain.ActivationOutcome) bool {
	if request.ActivationComplete || out.ActivationAttested || out.AuthenticationAttested ||
		request.Plan.InstallIntent != domain.InstallIntentAutomatic || request.Plan.Status != domain.PlanReady ||
		request.Delivery.ClientID != domain.ClientCopilot || request.Delivery.ActivePath != request.Plan.ActivePath ||
		request.Delivery.OwnedBase != request.Plan.TargetRoot || request.DeclaredName != request.Plan.DeclaredName ||
		request.Plan.Policy != domain.PolicyAllowed || out.Policy != domain.PolicyAllowed ||
		out.Verification != domain.VerificationInstalled || out.Activation != domain.ActivationPrepared ||
		(out.NativeEffect != domain.NativeEffectCommitted && out.NativeEffect != domain.NativeEffectUnchanged) ||
		!slices.Equal(out.NativeObjects, request.Delivery.NativeObjects) || len(request.Plan.Components) == 0 {
		return false
	}
	if _, err := copilotPlanRoot(request.Client, request.Plan); err != nil {
		return false
	}
	return copilotProjectSkillComponents(request.Plan.Components)
}

func copilotProjectSkillComponents(components []domain.ComponentDecision) bool {
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
func captureCopilotProjectSkills(ctx context.Context, env clients.Env, request domain.ActivationRequest) *copilotProjectSkillProof {
	if verifyCopilotTrust(env, request, true) != nil {
		return nil
	}
	layout, err := copilotComponents(request.Plan.TargetContext.ProjectRoot)
	if err != nil {
		return nil
	}
	objects := copilotNativeObjects(request.Delivery.NativeObjects)
	if len(objects) != len(request.Plan.Components) {
		return nil
	}
	if layout.Verify(objects, false, env.NativeConfig) != nil {
		return nil
	}
	proof := &copilotProjectSkillProof{objects: slices.Clone(request.Delivery.NativeObjects),
		components: slices.Clone(request.Plan.Components), authentication: shared.ObservedSkillInstallationAuthentication(request.Plan),
		skills: make(map[string]copilotObservedSkill, len(objects))}
	trustPath, err := copilotTrustPath(request.Client, request.Plan)
	if err != nil {
		return nil
	}
	proof.trust, err = env.NativeConfig.ReadExactFileBounded(trustPath, 1<<20)
	if err != nil {
		return nil
	}
	for _, object := range objects {
		skill, ok := captureCopilotSkillMetadata(ctx, env, layout, request.Delivery.ActivePath, object)
		if !ok {
			return nil
		}
		if _, duplicate := proof.skills[object.LogicalName]; duplicate {
			return nil
		}
		proof.skills[object.LogicalName] = skill
	}
	for _, component := range request.Plan.Components {
		if _, ok := proof.skills[component.Name]; !ok {
			return nil
		}
	}
	proof.activeDigest, err = env.Snapshots.Digest(ctx, request.Delivery.ActivePath)
	if err != nil || request.Delivery.ArtifactDigest == "" || proof.activeDigest != request.Delivery.ArtifactDigest {
		return nil
	}
	return proof
}

func captureCopilotSkillMetadata(ctx context.Context, env clients.Env, layout shared.ProjectComponents, active string, object domain.NativeObjectOwnership) (copilotObservedSkill, bool) {
	if object.Kind != copilotProjectSkillKind || object.UserModified || object.ProtectionClass != "managed" {
		return copilotObservedSkill{}, false
	}
	source, err := layout.SkillObject(object.LogicalName, object.SourceRelative, active)
	if err != nil || !reflect.DeepEqual(source, object) {
		return copilotObservedSkill{}, false
	}
	body, err := shared.ReadOwnedSkillMetadata(ctx, env.NativeConfig, env.Snapshots, object.Path, object.ManagedDigest)
	if err != nil {
		return copilotObservedSkill{}, false
	}
	return copilotProjectSkillMetadata(ctx, env, object.LogicalName, object.Path, body)
}

func copilotProjectSkillMetadata(ctx context.Context, env clients.Env, name, path string, body []byte) (copilotObservedSkill, bool) {
	skill, err := env.ParseSkill(ctx, name, body)
	if err != nil {
		return copilotObservedSkill{}, false
	}
	return copilotObservedSkill{name: name, path: path, description: skill.Description}, true
}

func runCopilotProjectSkills(ctx context.Context, env clients.Env, executable string, target domain.TargetContext) (ports.CommandResult, error) {
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return shared.RunNativeRegistry(bounded, env.Runner, copilotProjectCatalogCommand(executable, target, "skill"))
}

func copilotProjectCatalogCommand(executable string, target domain.TargetContext, kind string) ports.Command {
	// The offline catalog needs no provider, model, credentials or permission
	// overrides. Bind the same selected profile used by ownership and trust.
	environment := []string{"COPILOT_OFFLINE=true", "COPILOT_AUTO_UPDATE=false", "COPILOT_HOME=" + target.UserConfigRoot}
	for _, key := range []string{"PATH", "HOME", "USERPROFILE", "SYSTEMROOT", "WINDIR", "TEMP", "TMP", "TMPDIR", "LANG", "LC_ALL",
		"XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME", "XDG_DATA_HOME"} {
		if value, ok := os.LookupEnv(key); ok {
			environment = append(environment, key+"="+value)
		}
	}
	return ports.Command{
		Argv: []string{executable, kind, "list", "--json"},
		Dir:  target.ProjectRoot, Env: environment, StdoutLimitBytes: 1 << 20,
	}
}

// Accept only the measured ordinary array schema. Foreign names may coexist;
// a namesake or second path cannot substitute for an exact owned Skill.
func copilotProjectSkillsListed(body []byte, expected map[string]copilotObservedSkill) bool {
	rows, ok := copilotCatalogArray(body)
	if !ok || len(expected) == 0 {
		return false
	}
	names, paths := map[string]bool{}, map[string]bool{}
	found := 0
	for _, value := range rows {
		row, ok := value.(map[string]any)
		if !ok || len(row) != 5 || !copilotSkillCatalogRow(row) {
			return false
		}
		name, path := row["name"].(string), row["path"].(string)
		key := shared.FoldJSONKey(name)
		if names[key] || paths[path] {
			return false
		}
		names[key], paths[path] = true, true
		if skill, selected := expected[name]; selected {
			if path != skill.path || row["description"] != skill.description || row["source"] != copilotObservedSkillSource(skill) || row["enabled"] != true {
				return false
			}
			found++
		}
	}
	return found == len(expected)
}

func copilotSkillCatalogRow(row map[string]any) bool {
	for _, key := range []string{"name", "path", "description", "source"} {
		if _, ok := row[key].(string); !ok {
			return false
		}
	}
	name, path := row["name"].(string), row["path"].(string)
	_, enabled := row["enabled"].(bool)
	return enabled && name != "" && row["source"] != "" &&
		filepath.IsAbs(path) && filepath.Clean(path) == path &&
		!strings.ContainsAny(path, "\x00\r\n")
}

func copilotProjectSkillCLI(client domain.DetectedClient) bool {
	if client.ClientID != domain.ClientCopilot || client.Status != domain.DetectionDetected || !filepath.IsAbs(client.ExecutablePath) {
		return false
	}
	for _, surface := range client.Surfaces {
		if surface.ID == "copilot_cli" && surface.Detected {
			return true
		}
	}
	return false
}

func copilotObservedSkillSource(skill copilotObservedSkill) string {
	if skill.source != "" {
		return skill.source
	}
	return "project"
}
