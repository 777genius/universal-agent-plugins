// The audit oracle composes production parsers and projectors. It never installs
// a package, invokes a native client, or writes the observed installation.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/loader"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/windsurf"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/packagesnapshot"
)

type request struct {
	SourceRoot         string `json:"source_root"`
	TreeDigest         string `json:"tree_digest"`
	ManifestDigest     string `json:"manifest_digest"`
	PhysicalArtifactID string `json:"physical_artifact_id"`
	ActivePath         string `json:"active_path"`
	DataPath           string `json:"data_path"`
	CLI                string `json:"cli"`
	CLIVersion         string `json:"cli_version"`
	WorkRoot           string `json:"work_root"`
}

type result struct {
	TreeDigest     string `json:"tree_digest"`
	ManifestDigest string `json:"manifest_digest"`
	DeliveryDigest string `json:"delivery_digest"`
	StagingPath    string `json:"staging_path"`
}

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var input request
	decoder := json.NewDecoder(io.LimitReader(os.Stdin, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("oracle requires one complete request")
	}
	if err := validatePaths(input); err != nil {
		return err
	}
	output, err := project(context.Background(), input)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(output)
}

func validatePaths(input request) error {
	for _, path := range []string{input.SourceRoot, input.ActivePath, input.WorkRoot, input.CLI} {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path {
			return fmt.Errorf("oracle paths must be absolute and clean")
		}
	}
	if input.DataPath != "" && (!filepath.IsAbs(input.DataPath) || filepath.Clean(input.DataPath) != input.DataPath) {
		return fmt.Errorf("oracle data path must be absolute and clean")
	}
	if err := pathpolicy.ValidateLeafID(input.PhysicalArtifactID); err != nil {
		return err
	}
	if err := privateWorkRoot(input); err != nil {
		return err
	}
	entries, err := os.ReadDir(input.WorkRoot)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("oracle work directory must be empty")
	}
	return nil
}

func privateWorkRoot(input request) error {
	info, err := os.Lstat(input.WorkRoot)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		return fmt.Errorf("oracle scratch root must be a private directory")
	}
	resolved, err := filepath.EvalSymlinks(input.WorkRoot)
	if err != nil || resolved != input.WorkRoot {
		return fmt.Errorf("oracle scratch root must be canonical")
	}
	for _, path := range []string{input.SourceRoot, input.ActivePath, input.DataPath, input.CLI} {
		if path == "" {
			continue
		}
		for _, pair := range [][2]string{{input.WorkRoot, path}, {path, input.WorkRoot}} {
			relative, err := filepath.Rel(pair[0], pair[1])
			if err != nil || relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))) {
				return fmt.Errorf("oracle scratch overlaps an observed authority")
			}
		}
	}
	return nil
}

func load(ctx context.Context, input request) (domain.PackageEnvelope, func() error, error) {
	snapshot, err := (packagedigest.Builder{TempRoot: input.WorkRoot}).Snapshot(ctx, input.SourceRoot, domain.SourceIdentity{})
	if err != nil {
		return domain.PackageEnvelope{}, nil, err
	}
	cleanup := func() error { return packagedigest.Remove(snapshot) }
	registry, err := specregistry.New()
	if err != nil {
		return domain.PackageEnvelope{}, cleanup, err
	}
	envelope, err := (loader.Loader{Registry: registry}).LoadSnapshot(ctx, snapshot)
	if err != nil {
		return envelope, cleanup, err
	}
	if envelope.TreeDigest != input.TreeDigest || envelope.ManifestDigest != input.ManifestDigest {
		return envelope, cleanup, fmt.Errorf("independently pinned source tree or normalized manifest differs")
	}
	return envelope, cleanup, nil
}

func project(ctx context.Context, input request) (result, error) {
	envelope, cleanup, err := load(ctx, input)
	if cleanup != nil {
		defer func() { _ = cleanup() }()
	}
	if err != nil {
		return result{}, err
	}
	registry, err := clients.NewRegistry(windsurf.New())
	if err != nil {
		return result{}, err
	}
	plan, err := planProjection(ctx, input, envelope, registry)
	if err != nil {
		return result{}, err
	}
	launcher, err := launcherSource(input, envelope, plan)
	if err != nil {
		return result{}, err
	}
	stager := providers.Stager{Registry: registry, Paths: pathpolicy.Policy{}, LauncherSource: launcher,
		SnapshotBuilder: packagesnapshot.Builder{TempRoot: input.WorkRoot}}
	delivery, err := stageProjection(ctx, input, envelope, plan, stager)
	if err != nil {
		return result{}, err
	}
	// Stage and all policy checks use private scratch paths. MCP is the only
	// path-bearing Windsurf output: reuse its production renderer to bind the
	// expected bytes to the actual final locators without writing to them.
	plan.ActivePath = input.ActivePath
	if err := windsurf.ProjectWindsurfMCP(delivery.StagingPath, envelope, plan, input.DataPath); err != nil {
		return result{}, err
	}
	digest, err := stager.SnapshotBuilder.Digest(ctx, delivery.StagingPath)
	return result{envelope.TreeDigest, envelope.ManifestDigest, digest, delivery.StagingPath}, err
}

func stageProjection(ctx context.Context, input request, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, stager providers.Stager) (domain.StagedDelivery, error) {
	dataPath := input.DataPath
	if dataPath == "" {
		dataPath = filepath.Join(input.WorkRoot, "data")
		if err := os.Mkdir(dataPath, 0o700); err != nil {
			return domain.StagedDelivery{}, err
		}
	}
	return stager.StageWithPluginData(ctx, envelope, plan, "independent-projection", domain.CompatibilityHints{}, dataPath)
}

func launcherSource(input request, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) (*managedstdio.Source, error) {
	for _, name := range shared.SupportedMCPNames(plan) {
		if envelope.MCP.Servers[name].Type == "stdio" {
			return managedstdio.NewSource(input.CLI, input.CLIVersion)
		}
	}
	return nil, nil
}

func planProjection(ctx context.Context, input request, envelope domain.PackageEnvelope, registry *clients.Registry) (domain.DeliveryPlan, error) {
	profile := filepath.Join(input.WorkRoot, "profile", "devin")
	if err := os.MkdirAll(profile, 0o700); err != nil {
		return domain.DeliveryPlan{}, err
	}
	client := domain.DetectedClient{ClientID: domain.ClientWindsurf, Status: domain.DetectionDetected,
		ExecutablePath: filepath.Join(input.WorkRoot, "devin"), NativeConfigRoot: profile, Version: "3000.12.57"}
	plans := planner.Planner{Registry: registry, Paths: pathpolicy.Policy{}, ManagedRoot: filepath.Join(input.WorkRoot, "managed")}
	return plans.Plan(ctx, domain.PlanRequest{Envelope: envelope, Client: client, Scope: domain.ScopeUser,
		PhysicalArtifactID: input.PhysicalArtifactID})
}
