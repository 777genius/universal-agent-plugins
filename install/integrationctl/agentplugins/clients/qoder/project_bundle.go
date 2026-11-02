package qoder

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/filetree"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/packagesnapshot"
)

const projectNativeView = ".agentplugins-qoder-native"

func selectedProjectSkills(plan domain.DeliveryPlan) (map[string]bool, error) {
	names := map[string]bool{}
	for _, component := range plan.Components {
		if component.Support == domain.SupportUnsupported {
			continue
		}
		if component.Kind != domain.ComponentSkill || (component.Support != domain.SupportNative && (component.Support != domain.SupportProjected || component.Source != domain.ComponentSourceCommand)) {
			return nil, fmt.Errorf("qoder project component is unqualified")
		}
		if err := pathpolicy.ValidateLeafID(component.Name); err != nil {
			return nil, err
		}
		if names[component.Name] {
			return nil, fmt.Errorf("qoder project skill identity is duplicated")
		}
		names[component.Name] = true
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("qoder project preparation requires a supported skill")
	}
	return names, nil
}

func projectBundle(ctx context.Context, in clients.ProjectionInput) ([]domain.NativeObjectOwnership, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateProjectPlan(domain.DetectedClient{ClientID: domain.ClientQoder, ConfigRoot: in.Plan.TargetContext.UserConfigRoot}, in.Plan); err != nil {
		return nil, err
	}
	if !filepath.IsAbs(in.StagingPath) || filepath.Clean(in.StagingPath) != in.StagingPath || filepath.Dir(in.StagingPath) != in.Plan.TargetRoot || !strings.HasPrefix(filepath.Base(in.StagingPath), ".agentplugins-staging-") {
		return nil, fmt.Errorf("qoder project staging differs from exact selected managed base")
	}
	if err := pathpolicy.RequireContainedChild(in.Plan.TargetContext.ProjectRoot, in.StagingPath); err != nil {
		return nil, err
	}
	names, err := selectedProjectSkills(in.Plan)
	if err != nil {
		return nil, err
	}
	view := filepath.Join(in.StagingPath, projectNativeView)
	if _, err := os.Lstat(view); err == nil {
		return nil, fmt.Errorf("source contains reserved Qoder native view")
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if in.Envelope.Manifest.Name != in.Plan.DeclaredName || in.Envelope.Manifest.Version != in.Plan.DeclaredVersion {
		return nil, fmt.Errorf("qoder projected manifest differs from selected identity")
	}
	for name := range names {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		skill, ok := in.Envelope.Skills[name]
		if !ok || filepath.ToSlash(skill.RelativePath) != "skills/"+name+"/SKILL.md" {
			return nil, fmt.Errorf("qoder project skill has no exact portable directory: %s", name)
		}
		source := filepath.Join(in.StagingPath, "skills", name)
		if err := pathpolicy.RequireContainedChild(in.StagingPath, source); err != nil {
			return nil, err
		}
		if err := filetree.CopyDir(source, filepath.Join(view, "skills", name)); err != nil {
			return nil, err
		}
	}
	manifest := shared.ManifestFromEnvelope(in.Envelope, shared.WithAuthorNameEmail())
	manifest["skills"] = "./skills/"
	if err := shared.WriteJSON(filepath.Join(view, ".qoder-plugin", "plugin.json"), manifest); err != nil {
		return nil, err
	}
	return nil, verifyProjectView(in.StagingPath, in.Plan)
}

// verifyProjectView checks the complete importable surface. Raw source payload
// remains outside this directory and cannot acquire native discovery semantics.
func verifyProjectView(active string, plan domain.DeliveryPlan) error {
	names, err := selectedProjectSkills(plan)
	if err != nil {
		return err
	}
	view := filepath.Join(active, projectNativeView)
	if err := pathpolicy.RequireContainedChild(active, view); err != nil {
		return err
	}
	if err := verifyProjectViewPaths(active, view, names); err != nil {
		return err
	}
	manifest, err := readProjectManifest(filepath.Join(view, ".qoder-plugin", "plugin.json"))
	if err != nil {
		return err
	}
	if manifest["name"] != plan.DeclaredName || manifest["skills"] != "./skills/" || (plan.DeclaredVersion != "" && manifest["version"] != plan.DeclaredVersion) {
		return fmt.Errorf("qoder native view manifest differs from selected identity")
	}
	allowed := map[string]bool{"name": true, "version": true, "description": true, "homepage": true, "repository": true, "license": true, "author": true, "keywords": true, "skills": true}
	for key := range manifest {
		if !allowed[key] {
			return fmt.Errorf("qoder native view manifest contains an unqualified native resource")
		}
	}
	return nil
}

func verifyProjectViewPaths(active, view string, names map[string]bool) error {
	for root, expected := range map[string]map[string]bool{
		view:                                 {".qoder-plugin": true, "skills": true},
		filepath.Join(view, ".qoder-plugin"): {"plugin.json": true},
		filepath.Join(view, "skills"):        names,
	} {
		entries, err := os.ReadDir(root)
		if err != nil {
			return err
		}
		if len(entries) != len(expected) {
			return fmt.Errorf("qoder native view has an unexpected path set")
		}
		for _, entry := range entries {
			if !expected[entry.Name()] {
				return fmt.Errorf("qoder native view contains an unselected path")
			}
			if err := pathpolicy.RequireContainedChild(active, filepath.Join(root, entry.Name())); err != nil {
				return err
			}
		}
	}
	for name := range names {
		skill := filepath.Join(view, "skills", name, "SKILL.md")
		if err := pathpolicy.RequireContainedChild(active, skill); err != nil {
			return err
		}
		info, err := os.Lstat(skill)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("qoder projected skill is not a regular file")
		}
	}
	return nil
}

func readProjectManifest(path string) (map[string]any, error) {
	file, err := openProjectManifest(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return nil, fmt.Errorf("qoder native manifest is not a bounded regular file")
	}
	body, err := io.ReadAll(io.LimitReader(file, (1<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(body) > 1<<20 {
		return nil, fmt.Errorf("qoder native manifest exceeds byte limit")
	}
	return shared.DecodeStrictJSONObject(body)
}

func verifyProjectBundle(ctx context.Context, request domain.ActivationRequest) error {
	if len(request.Delivery.NativeObjects) != 1 {
		return fmt.Errorf("qoder project delivery requires its exact managed package receipt")
	}
	object := request.Delivery.NativeObjects[0]
	if object.Kind != "managed_package_directory" || object.ObjectID != "package:qoder:"+request.Plan.PhysicalArtifactID || object.Path != request.Plan.ActivePath || object.LogicalName != request.DeclaredName || object.ManagedDigest == "" || object.ManagedDigest != request.Delivery.ArtifactDigest {
		return fmt.Errorf("qoder project managed receipt differs from delivered identity")
	}
	digest, err := (packagesnapshot.Builder{}).Digest(ctx, request.Plan.ActivePath)
	if err != nil {
		return err
	}
	if digest != object.ManagedDigest {
		return fmt.Errorf("qoder project package digest differs from its managed receipt")
	}
	if err := verifyProjectView(request.Plan.ActivePath, request.Plan); err != nil {
		return err
	}
	// The closed-view reads cannot adopt a package changed during verification.
	digest, err = (packagesnapshot.Builder{}).Digest(ctx, request.Plan.ActivePath)
	if err != nil {
		return err
	}
	if digest != object.ManagedDigest {
		return fmt.Errorf("qoder project package changed during verification")
	}
	return nil
}
