package pi

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"syscall"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

var errPiSkillObservationRead = errors.New("pi Skill observation could not read current authority")

type piObservedSkill struct {
	name, description, path string
}

type piSkillProof struct {
	objects        []domain.NativeObjectOwnership
	components     []domain.ComponentDecision
	skills         map[string]piObservedSkill
	trust          []shared.NativeFileSnapshot
	authentication domain.AuthenticationState
}

// The confirmed install already owns its files. Project discovery additionally
// requires its existing confirmed trust hook; this observation never grants it.
func completePiSkills(ctx context.Context, env clients.Env, request domain.ActivationRequest, out domain.ActivationOutcome) (domain.ActivationOutcome, error) {
	if err := ctx.Err(); err != nil {
		return out, err
	}
	runner, ok := env.Runner.(ports.CompletionCommandRunner)
	if !ok || env.ParseSkill == nil || env.Snapshots == nil || !piSkillsAuthority(request, out) || !piSkillCLI(request) {
		return out, nil
	}
	bounded, cancel := context.WithTimeout(ctx, piSkillObservationTimeout)
	defer cancel()
	proof, err := capturePiSkills(bounded, env, request)
	if err != nil {
		return out, bounded.Err()
	}
	observed, queryErr := runPiSkillCatalog(bounded, runner, request)
	verification, finish := context.WithTimeout(ctx, 5*time.Second)
	defer finish()
	return finishPiSkills(verification, env, request, out, proof, observed, queryErr)
}

func finishPiSkills(ctx context.Context, env clients.Env, request domain.ActivationRequest, out domain.ActivationOutcome, proof *piSkillProof, observed []any, queryErr error) (domain.ActivationOutcome, error) {
	queryErr = shared.CompletionProbeFailure(queryErr)
	if domain.IsUnclosedClientProbe(queryErr) {
		out.NativeEffect = domain.NativeEffectUncertain
		return shared.FailedActivation(out, "confirm prior Pi catalog process cleanup before retrying", queryErr)
	}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	fresh, err := capturePiSkills(ctx, env, request)
	if stopped := ctx.Err(); stopped != nil {
		return out, stopped
	}
	if errors.Is(err, errPiSkillObservationRead) {
		// Read-only uncertainty is not evidence of a new native mutation. Retain
		// the verified binding and its already recorded effect, without promotion.
		return out, nil
	}
	if err != nil || !reflect.DeepEqual(proof, fresh) {
		out.NativeEffect = domain.NativeEffectUncertain
		return shared.FailedActivation(out, "repair the exact owned Pi Skills and review the selected profile before retrying",
			errors.Join(queryErr, err, errors.New("pi Skill source, native objects or trust authority changed during observation")))
	}
	if queryErr == nil && piSkillCatalogMatches(observed, proof.skills, request.Plan.Scope, request.Plan.NativeRegistryRoot) &&
		piSkillsAuthority(request, out) {
		out.Activation = domain.ActivationActive
		if out.Authentication == domain.AuthenticationNotChecked {
			out.Authentication = proof.authentication
		}
		// Preserve the completed trust scope disclosure without presenting its
		// now-completed restart/verification step as a pending manual action.
		actions := make([]string, 0, len(out.UserActions))
		for _, action := range out.UserActions {
			if action != piCompletedSkillAction(request.Plan.Scope) {
				actions = append(actions, action)
			}
		}
		if request.Plan.Scope == domain.ScopeProject {
			actions = shared.AppendUnique(actions, piCompletedTrustNotice)
			out.CompletedUserActions = shared.AppendUnique(out.CompletedUserActions, piCompletedTrustNotice)
		}
		out.UserActions = actions
	}
	return out, nil
}

func piSkillsAuthority(request domain.ActivationRequest, out domain.ActivationOutcome) bool {
	if request.Plan.PackageMode != domain.PackageNative || request.Plan.Status != domain.PlanReady || request.Plan.InstallIntent != domain.InstallIntentAutomatic ||
		request.ActivationComplete || out.ActivationAttested || out.AuthenticationAttested ||
		request.Plan.Policy != domain.PolicyAllowed || out.Policy != domain.PolicyAllowed ||
		validatePlan(request.Plan) != nil || piSelectedAuthority(request.Client, request.Plan) != nil {
		return false
	}
	target := request.Plan.TargetContext
	if target.Scope != request.Plan.Scope || target.UserConfigRoot != request.Client.ConfigRoot {
		return false
	}
	if request.Plan.Scope == domain.ScopeUser && target.ProjectRoot != "" {
		return false
	}
	if request.Plan.Scope == domain.ScopeProject && !slices.Contains(out.UserActions, piProjectTrustReadyAction) {
		return false
	}
	return piSkillsReceiptAuthority(request, out) && piPureSkills(request.Plan.Components)
}

func piSkillsReceiptAuthority(request domain.ActivationRequest, out domain.ActivationOutcome) bool {
	return request.DeclaredName == request.Plan.DeclaredName &&
		request.Delivery.ClientID == domain.ClientPi && request.Delivery.ActivePath == request.Plan.ActivePath &&
		request.Delivery.OwnedBase == request.Plan.TargetRoot && request.Delivery.ArtifactDigest != "" &&
		out.Activation == domain.ActivationPrepared && out.Verification == domain.VerificationInstalled &&
		(out.NativeEffect == domain.NativeEffectCommitted || out.NativeEffect == domain.NativeEffectUnchanged) &&
		slices.Equal(out.NativeObjects, request.Delivery.NativeObjects)
}

func piCompletedSkillAction(scope domain.InstallScope) string {
	if scope == domain.ScopeProject {
		return piProjectTrustReadyAction
	}
	return piUserComponentAction
}

func piPureSkills(components []domain.ComponentDecision) bool {
	if len(components) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, component := range components {
		key := shared.FoldJSONKey(component.Name)
		if component.Kind != domain.ComponentSkill || component.Name == "" || seen[key] ||
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
		seen[key] = true
	}
	return true
}

func capturePiSkills(ctx context.Context, env clients.Env, request domain.ActivationRequest) (*piSkillProof, error) {
	if err := env.NativeConfig.RequireBoundedRead(); err != nil {
		return nil, err
	}
	if err := piPackageSkillReceipt(ctx, env.Snapshots, request); err != nil {
		return nil, err
	}
	layout, err := piComponents(request.Plan.NativeRegistryRoot, request.Plan.Scope)
	if err != nil {
		return nil, err
	}
	objects := piScopedNative(request.Delivery.NativeObjects, request.Plan.Scope)
	if len(objects) != len(request.Plan.Components) {
		return nil, errors.New("pi Skill receipt count differs")
	}
	if err := layout.Verify(objects, false, env.NativeConfig); err != nil {
		return nil, piSkillReadFailure(err)
	}
	proof := &piSkillProof{objects: slices.Clone(request.Delivery.NativeObjects), components: slices.Clone(request.Plan.Components),
		skills:         make(map[string]piObservedSkill, len(objects)),
		authentication: shared.ObservedSkillInstallationAuthentication(request.Plan)}
	for _, object := range objects {
		skill, err := piOwnedSkillMetadata(ctx, env, layout, request.Delivery.ActivePath, object)
		if err != nil {
			return nil, piSkillReadFailure(err)
		}
		proof.skills[skill.name] = skill
	}
	if len(proof.skills) != len(request.Plan.Components) {
		return nil, errors.New("pi Skill names are ambiguous")
	}
	for _, component := range request.Plan.Components {
		if _, ok := proof.skills[component.Name]; !ok {
			return nil, errors.New("pi native Skill differs from selected component")
		}
	}
	proof.trust, err = piSkillTrust(env, request)
	return proof, err
}

func piOwnedSkillMetadata(ctx context.Context, env clients.Env, layout shared.ProjectComponents, active string, object domain.NativeObjectOwnership) (piObservedSkill, error) {
	if object.Kind != layout.SkillKind || object.UserModified || object.ProtectionClass != "managed" {
		return piObservedSkill{}, errors.New("pi Skill object is not exactly owned")
	}
	source, err := layout.SkillObject(object.LogicalName, object.SourceRelative, active)
	if err != nil || source != object {
		return piObservedSkill{}, errors.New("pi Skill source differs from receipt")
	}
	body, err := shared.ReadOwnedSkillMetadata(ctx, env.NativeConfig, env.Snapshots, object.Path, object.ManagedDigest)
	if err != nil {
		return piObservedSkill{}, err
	}
	skill, err := env.ParseSkill(ctx, object.LogicalName, body)
	if err != nil {
		return piObservedSkill{}, err
	}
	return piObservedSkill{name: object.LogicalName, description: skill.Description,
		path: filepath.Join(object.Path, "SKILL.md")}, nil
}

func piPackageSkillReceipt(ctx context.Context, snapshots ports.DirectorySnapshots, request domain.ActivationRequest) error {
	packages := 0
	for _, object := range request.Delivery.NativeObjects {
		if object.Kind == piProjectSkillKind && request.Plan.Scope == domain.ScopeProject ||
			object.Kind == piUserSkillKind && request.Plan.Scope == domain.ScopeUser {
			continue
		}
		if object.Kind != "managed_package_directory" || object.UserModified || object.ProtectionClass != "managed" ||
			object.ObjectID != "package:pi:"+request.Plan.PhysicalArtifactID || object.Path != request.Plan.ActivePath ||
			object.LogicalName != request.DeclaredName || object.ManagedDigest != request.Delivery.ArtifactDigest {
			return errors.New("pi managed package receipt differs")
		}
		packages++
	}
	if packages != 1 {
		return errors.New("pi managed package receipt missing or duplicated")
	}
	digest, err := snapshots.Digest(ctx, request.Delivery.ActivePath)
	if err != nil {
		return piSkillReadFailure(err)
	}
	if digest != request.Delivery.ArtifactDigest {
		return errors.New("pi managed package bytes changed")
	}
	return nil
}

func piSkillTrust(env clients.Env, request domain.ActivationRequest) ([]shared.NativeFileSnapshot, error) {
	kernel := env.NativeConfig
	paths := []string{filepath.Join(request.Client.ConfigRoot, "settings.json"), filepath.Join(request.Client.ConfigRoot, "trust.json")}
	if request.Plan.Scope == domain.ScopeProject {
		recorded, err := observePiProjectTrust(kernel, request)
		if err != nil {
			return nil, piSkillReadFailure(err)
		}
		if !recorded {
			return nil, errors.New("pi project trust was not previously confirmed")
		}
		blocker, err := piTrustBlocker(kernel, request.Client.ConfigRoot, request.Plan.TargetContext.ProjectRoot)
		if err != nil {
			return nil, piSkillReadFailure(err)
		}
		if blocker != "" {
			return nil, fmt.Errorf("pi project trust is blocked: %s", blocker)
		}
		_, entry, projects := piTrustHookPaths(request.Client.ConfigRoot)
		paths = append(paths, entry, projects)
	}
	snapshots := make([]shared.NativeFileSnapshot, 0, len(paths))
	for _, path := range paths {
		snapshot, err := kernel.ReadExactFileBounded(path, 1<<20)
		if err != nil {
			return nil, piSkillReadFailure(err)
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, nil
}

const piCompletedTrustNotice = "Pi discovered the owned Skills in a fresh catalog. UAP's trust hook covers this exact project only while its .pi holds unchanged UAP-managed resources. Trust covers the whole folder; a running Pi session keeps it across /reload. Project node_modules imports, MCP working directories and .uap data are not checked. This catalog does not verify vendor authentication or model calls."

// Only a direct OS read failure is inconclusive. Do not traverse wrapped or
// joined errors: they may also contain a strict admission or custody refusal.
func piSkillReadFailure(err error) error {
	pathError, ok := err.(*os.PathError)
	if !ok {
		return err
	}
	cause, ok := pathError.Err.(syscall.Errno)
	if ok && (cause.Is(os.ErrPermission) || cause == syscall.EIO) {
		return errors.Join(errPiSkillObservationRead, err)
	}
	return err
}
