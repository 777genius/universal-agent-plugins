package pi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Pi documents no trust-file API, and UAP never writes Pi's trust.json. A
// confirmed project installation instead records its exact canonical project,
// owned resource names and their content digests beside one UAP-owned user
// extension that answers Pi's documented project_trust event (see
// piTrustHookSource). Any later change to owned content, such as a git pull,
// returns the project to Pi's own trust flow until a confirmed UAP activation
// records the new content. Removing the last owned installation of a project
// deletes its record immediately.
const (
	piTrustHookName      = "agentplugins-project-trust"
	piTrustHookEntry     = "index.ts"
	piTrustProjectsName  = "projects.json"
	piTrustSchemaVersion = 1
	piTrustDocumentLimit = 1 << 20
)

// Project actions after activation. The pending action is also the only
// observation that admits completing an old installation's setup.
const (
	piProjectTrustReadyAction   = "Open or restart Pi in this project, or reload resources; UAP's Pi trust hook trusts this exact project while its .pi holds only unchanged UAP-managed resources. Installed files do not prove runtime loading, authentication or model calls."
	piProjectTrustPendingAction = "Pi project trust is not set up for this installation; repeat the confirmed installation to set it up, or review project trust in Pi, then reload or restart. Installed files do not prove runtime loading."
	piProjectTrustBlockedFormat = "UAP's Pi trust hook leaves this project to Pi's own trust decision because %s; review it and Pi's trust prompt, then reload or restart. Installed files do not prove runtime loading."
	piProjectTrustFailedFormat  = "Pi project trust could not be set up automatically: %v; review project trust in Pi, then reload or restart. Installed files do not prove runtime loading."
)

var (
	errPiTrustAbsent     = errors.New("pi project trust hook is absent")
	piTrustDigestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

// piTrustOwner maps one installation's owned names per Pi project surface to
// content digests (see project_trust_digest.go). Its key in the record is the
// installation's managed package directory. The hook trusts a name only while
// its live content still has the recorded digest.
type piTrustOwner struct {
	Extensions map[string]string `json:"extensions"`
	Skills     map[string]string `json:"skills"`
	MCPServers map[string]string `json:"mcp_servers"`
}

type piTrustDocument struct {
	SchemaVersion int                                `json:"schema_version"`
	Projects      map[string]map[string]piTrustOwner `json:"projects"`
}

func piTrustHookPaths(user string) (dir, entry, projects string) {
	dir = filepath.Join(user, "extensions", piTrustHookName)
	return dir, filepath.Join(dir, piTrustHookEntry), filepath.Join(dir, piTrustProjectsName)
}

// piTrustTarget binds trust to the selected canonical project and original
// user profile. It never covers a home directory or filesystem root.
func piTrustTarget(client domain.DetectedClient, plan domain.DeliveryPlan) (string, string, error) {
	root, err := projectPiRoot(client, plan.TargetContext, plan.NativeRegistryRoot)
	if err != nil {
		return "", "", err
	}
	project := filepath.Dir(root)
	canonical, err := clients.CanonicalDirectory(project, os.Lstat, filepath.EvalSymlinks)
	if err != nil || canonical != project {
		return "", "", fmt.Errorf("pi project trust requires the selected canonical project")
	}
	if filepath.Dir(project) == project {
		return "", "", fmt.Errorf("pi project trust cannot cover a filesystem root")
	}
	if home, err := os.UserHomeDir(); err == nil {
		resolved, resolveErr := filepath.EvalSymlinks(home)
		if project == home || resolveErr == nil && project == resolved {
			return "", "", fmt.Errorf("pi project trust cannot cover the entire home directory")
		}
	}
	return project, client.ConfigRoot, nil
}

// piTrustNames lists this installation's owned names without reading disk.
func piTrustNames(plan domain.DeliveryPlan, objects []domain.NativeObjectOwnership) (string, piTrustOwner, error) {
	artifact := filepath.Base(plan.ActivePath)
	if artifact != plan.PhysicalArtifactID || pathpolicy.ValidateLeafID(artifact) != nil {
		return "", piTrustOwner{}, fmt.Errorf("pi project trust requires the managed package directory")
	}
	owner := piTrustOwner{Extensions: map[string]string{artifact: ""}, Skills: map[string]string{}, MCPServers: map[string]string{}}
	for _, object := range piScopedNative(objects, domain.ScopeProject) {
		switch object.Kind {
		case piProjectSkillKind:
			owner.Skills[object.LogicalName] = ""
		case piProjectMCPKind:
			owner.MCPServers[object.LogicalName] = ""
		}
	}
	return artifact, owner, validPiTrustOwner(artifact, owner, false)
}

// piTrustInventory digests the owned content that a confirmed activation has
// just installed and verified, so the hook can refuse any later change.
func piTrustInventory(project string, plan domain.DeliveryPlan, objects []domain.NativeObjectOwnership) (string, piTrustOwner, error) {
	artifact, owner, err := piTrustNames(plan, objects)
	if err != nil {
		return "", owner, err
	}
	config := filepath.Join(project, ".pi")
	if plan.ActivePath != filepath.Join(config, "extensions", artifact) {
		return "", owner, fmt.Errorf("pi project trust package differs from the selected project")
	}
	for surface, names := range map[string]map[string]string{"extensions": owner.Extensions, "skills": owner.Skills} {
		for name := range names {
			if names[name], err = piTrustTreeDigest(filepath.Join(config, surface, name)); err != nil {
				return "", owner, err
			}
		}
	}
	if len(owner.MCPServers) > 0 {
		servers, err := piTrustMCPServers(filepath.Join(config, "mcp.json"))
		if err != nil {
			return "", owner, err
		}
		for name := range owner.MCPServers {
			entry, ok := servers[name]
			if !ok {
				return "", owner, fmt.Errorf("owned Pi MCP server %q is missing", name)
			}
			if owner.MCPServers[name], err = piTrustValueDigest(entry); err != nil {
				return "", owner, err
			}
		}
	}
	return artifact, owner, validPiTrustOwner(artifact, owner, true)
}

// Directory names must be portable path segments; MCP server names are JSON
// member names and only need to be printable. Recorded digests are exact.
func validPiTrustOwner(artifact string, owner piTrustOwner, digested bool) error {
	if _, ok := owner.Extensions[artifact]; !ok || len(owner.Extensions) != 1 {
		return fmt.Errorf("pi project trust owner must own exactly its package directory")
	}
	for surface, names := range map[string]map[string]string{"extensions": owner.Extensions, "skills": owner.Skills, "mcp_servers": owner.MCPServers} {
		for name, digest := range names {
			if surface == "mcp_servers" && (name == "" || !utf8.ValidString(name) || strings.IndexFunc(name, unicode.IsControl) >= 0) ||
				surface != "mcp_servers" && pathpolicy.ValidatePortablePathSegment(name) != nil {
				return fmt.Errorf("pi project trust has an invalid owned %s name", surface)
			}
			if digested && !piTrustDigestPattern.MatchString(digest) {
				return fmt.Errorf("pi project trust has an invalid %s digest", surface)
			}
		}
	}
	return nil
}

func decodePiTrust(snapshot nativeconfig.FileSnapshot) (piTrustDocument, error) {
	document := piTrustDocument{SchemaVersion: piTrustSchemaVersion, Projects: map[string]map[string]piTrustOwner{}}
	if !snapshot.Exists {
		return document, nil
	}
	if len(snapshot.Body) > piTrustDocumentLimit {
		return document, fmt.Errorf("pi project trust record exceeds limit")
	}
	if _, err := shared.DecodeStrictJSONObject(snapshot.Body); err != nil {
		return document, fmt.Errorf("pi project trust record is not strict JSON: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(snapshot.Body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return document, fmt.Errorf("pi project trust record is unreadable: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return document, fmt.Errorf("pi project trust record contains trailing data")
	}
	if document.SchemaVersion != piTrustSchemaVersion || document.Projects == nil {
		return document, fmt.Errorf("pi project trust record schema is unsupported")
	}
	for project, owners := range document.Projects {
		if cleanRoot(project) != nil || len(owners) == 0 {
			return document, fmt.Errorf("pi project trust record has an invalid project")
		}
		for artifact, owner := range owners {
			if err := validPiTrustOwner(artifact, owner, true); err != nil {
				return document, err
			}
		}
	}
	return document, nil
}

func encodePiTrust(document piTrustDocument) ([]byte, error) {
	body, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, err
	}
	body = append(body, '\n')
	if len(body) > piTrustDocumentLimit {
		return nil, fmt.Errorf("pi project trust record exceeds limit")
	}
	return body, nil
}

type piTrustDirUse int

const (
	piTrustDirGrant piTrustDirUse = iota
	piTrustDirObserve
	piTrustDirRevoke
)

// checkPiTrustHookDir admits only real directories. A grant creates them, and
// a grant or observation also requires that the hook is discoverable and that
// its directory holds only the hook's files and their native writer locks:
// foreign bytes there could redirect Pi's discovery, so they are refused
// rather than adopted. Revocation needs neither and must not be blocked by
// unrelated user extensions.
func checkPiTrustHookDir(user string, use piTrustDirUse) error {
	dir, entry, projects := piTrustHookPaths(user)
	allowed := map[string]bool{}
	for _, path := range []string{entry, projects} {
		allowed[filepath.Base(path)] = true
		locks, err := nativeconfig.WriterLockPaths(nativeconfig.Paths{JSON: path}, nativeconfig.CodecMCPServers)
		if err != nil {
			return err
		}
		for _, lock := range locks {
			allowed[filepath.Base(lock)] = true
		}
	}
	for _, path := range []string{filepath.Dir(dir), dir} {
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			if use != piTrustDirGrant {
				return errPiTrustAbsent
			}
			if err := os.Mkdir(path, 0o700); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("pi trust hook directory is not a real directory")
		}
	}
	if use == piTrustDirRevoke {
		return nil
	}
	if err := checkDiscoveryRoot(filepath.Dir(dir)); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, found := range entries {
		if !allowed[found.Name()] {
			return fmt.Errorf("pi trust hook directory contains foreign %q", found.Name())
		}
	}
	return nil
}

// updatePiTrust holds both hook files under native writer locks in one fixed
// order, then commits the mutated record through commitPiTrust.
func updatePiTrust(kernel nativeconfig.Kernel, user string, use piTrustDirUse, mutate func(*piTrustDocument)) (err error) {
	if err := checkPiTrustHookDir(user, use); err != nil {
		return err
	}
	_, entryPath, projectsPath := piTrustHookPaths(user)
	hook, err := kernel.BeginExactFile(entryPath)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, hook.Close()) }()
	record, err := kernel.BeginExactFile(projectsPath)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, record.Close()) }()
	return commitPiTrust(hook, record, use == piTrustDirGrant, mutate)
}

type piTrustStep struct {
	file   *nativeconfig.ExactFile
	body   []byte
	remove bool
}

// commitPiTrust records projects before the hook and restores every exact
// original on the first failure. Either partial state answers no project: a
// hook without a record, or a record without a hook. Only a grant installs
// the hook bytes; an empty record removes the record and an owned hook, and
// foreign bytes at the hook path are never adopted or removed.
func commitPiTrust(hook, record *nativeconfig.ExactFile, grant bool, mutate func(*piTrustDocument)) error {
	foreign := hook.Original().Exists && !bytes.HasPrefix(hook.Original().Body, []byte(piTrustHookMarker))
	if grant && foreign {
		return fmt.Errorf("pi trust hook path holds a foreign extension")
	}
	document, err := decodePiTrust(record.Original())
	if err != nil {
		return err
	}
	mutate(&document)
	steps, err := piTrustSteps(hook, record, document, grant, foreign)
	if err != nil {
		return err
	}
	for index, step := range steps {
		if step.remove {
			err = step.file.Remove()
		} else {
			err = step.file.Apply(step.body)
		}
		if err != nil {
			for applied := index; applied >= 0; applied-- {
				err = errors.Join(err, steps[applied].file.Rollback())
			}
			return err
		}
	}
	return nil
}

func piTrustSteps(hook, record *nativeconfig.ExactFile, document piTrustDocument, grant, foreign bool) ([]piTrustStep, error) {
	if len(document.Projects) == 0 {
		steps := []piTrustStep{{file: record, remove: true}}
		if !foreign {
			steps = append(steps, piTrustStep{file: hook, remove: true})
		}
		return steps, nil
	}
	body, err := encodePiTrust(document)
	if err != nil {
		return nil, err
	}
	var steps []piTrustStep
	if original := record.Original(); !original.Exists || !bytes.Equal(original.Body, body) {
		steps = append(steps, piTrustStep{file: record, body: body})
	}
	source := []byte(piTrustHookSource)
	if current := hook.Original(); grant && (!current.Exists || !bytes.Equal(current.Body, source)) {
		steps = append(steps, piTrustStep{file: hook, body: source})
	}
	return steps, nil
}

// grantPiProjectTrust records this installation for its exact project. It
// runs only for a confirmed, verified activation, never for verification.
func grantPiProjectTrust(env clients.Env, request domain.ActivationRequest) error {
	if request.VerifyOnly || request.Plan.InstallIntent != domain.InstallIntentAutomatic || request.Delivery.ArtifactDigest == "" {
		return fmt.Errorf("pi project trust requires a confirmed verified installation")
	}
	project, user, err := piTrustTarget(request.Client, request.Plan)
	if err != nil {
		return err
	}
	artifact, owner, err := piTrustInventory(project, request.Plan, request.Delivery.NativeObjects)
	if err != nil {
		return err
	}
	return updatePiTrust(env.NativeConfig, user, piTrustDirGrant, func(document *piTrustDocument) {
		owners := document.Projects[project]
		if owners == nil {
			owners = map[string]piTrustOwner{}
		}
		owners[artifact] = owner
		document.Projects[project] = owners
	})
}

// revokePiProjectTrust deletes this installation's record before confirmed
// removal changes any project file. It creates nothing when no hook exists.
func revokePiProjectTrust(env clients.Env, request domain.DeactivationRequest) error {
	root, err := projectPiRoot(request.Client, request.TargetContext, request.NativeRegistryRoot)
	if err != nil {
		return err
	}
	artifact := filepath.Base(request.ManagedArtifactPath)
	if filepath.Dir(request.ManagedArtifactPath) != filepath.Join(root, "extensions") || request.PhysicalArtifactID != "" && request.PhysicalArtifactID != artifact {
		return fmt.Errorf("pi project trust removal differs from the managed package")
	}
	projects := []string{filepath.Dir(root)}
	if canonical, err := clients.CanonicalDirectory(projects[0], os.Lstat, filepath.EvalSymlinks); err == nil && canonical != projects[0] {
		projects = append(projects, canonical)
	}
	err = updatePiTrust(env.NativeConfig, request.Client.ConfigRoot, piTrustDirRevoke, func(document *piTrustDocument) {
		for _, project := range projects {
			if owners, ok := document.Projects[project]; ok {
				delete(owners, artifact)
				if len(owners) == 0 {
					delete(document.Projects, project)
				}
			}
		}
	})
	if errors.Is(err, errPiTrustAbsent) {
		return nil
	}
	return err
}

// observePiProjectTrust is the read-only verifier's view: the current hook
// bytes and this installation's exact owned names are recorded. It never
// derives digests from live content, so a changed resource is reported by
// piTrustBlocker instead of becoming a reason to record its new content.
func observePiProjectTrust(kernel nativeconfig.Kernel, request domain.ActivationRequest) (bool, error) {
	project, user, err := piTrustTarget(request.Client, request.Plan)
	if err != nil {
		return false, err
	}
	artifact, owner, err := piTrustNames(request.Plan, request.Delivery.NativeObjects)
	if err != nil {
		return false, err
	}
	if err := checkPiTrustHookDir(user, piTrustDirObserve); errors.Is(err, errPiTrustAbsent) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	_, entryPath, projectsPath := piTrustHookPaths(user)
	hook, err := kernel.ReadExactFile(entryPath)
	if err != nil || !hook.Exists || !bytes.Equal(hook.Body, []byte(piTrustHookSource)) {
		return false, err
	}
	snapshot, err := kernel.ReadExactFile(projectsPath)
	if err != nil {
		return false, err
	}
	document, err := decodePiTrust(snapshot)
	if err != nil {
		return false, err
	}
	recorded, ok := document.Projects[project][artifact]
	return ok && piTrustSameNames(recorded.Extensions, owner.Extensions) && piTrustSameNames(recorded.Skills, owner.Skills) &&
		piTrustSameNames(recorded.MCPServers, owner.MCPServers), nil
}

func piTrustSameNames(left, right map[string]string) bool {
	return slices.Equal(slices.Sorted(maps.Keys(left)), slices.Sorted(maps.Keys(right)))
}

// piProjectTrustAction grants trust during confirmed activation, observes it
// during read-only verification and explains every outcome honestly.
func piProjectTrustAction(env clients.Env, request domain.ActivationRequest) string {
	var err error
	if request.VerifyOnly {
		var recorded bool
		if recorded, err = observePiProjectTrust(env.NativeConfig, request); err == nil && !recorded {
			return piProjectTrustPendingAction
		}
	} else {
		err = grantPiProjectTrust(env, request)
	}
	if err == nil {
		var blocker string
		if blocker, err = piTrustBlocker(env.NativeConfig, request.Client.ConfigRoot, request.Plan.TargetContext.ProjectRoot); err == nil {
			if blocker == "" {
				return piProjectTrustReadyAction
			}
			return fmt.Sprintf(piProjectTrustBlockedFormat, blocker)
		}
	}
	return fmt.Sprintf(piProjectTrustFailedFormat, err)
}

// completePiProjectTrust replaces the generic project review action once the
// package is installed. Activation stays Prepared: a recorded project is not
// proof that Pi loaded it.
func completePiProjectTrust(env clients.Env, request domain.ActivationRequest, out domain.ActivationOutcome) domain.ActivationOutcome {
	if request.Plan.Scope != domain.ScopeProject || out.Verification != domain.VerificationInstalled {
		return out
	}
	actions := make([]string, 0, len(out.UserActions)+1)
	for _, action := range out.UserActions {
		if action != piProjectComponentAction && action != piProjectExtensionAction {
			actions = append(actions, action)
		}
	}
	out.UserActions = shared.AppendUnique(actions, piProjectTrustAction(env, request))
	return out
}
