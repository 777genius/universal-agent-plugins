package agentpluginscli

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"
	"github.com/spf13/cobra"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

const repairSourceResolutionTimeout = 2 * time.Minute

type repairTargetResult struct {
	Target     string           `json:"target"`
	Status     string           `json:"status"`
	NextAction string           `json:"next_action,omitempty"`
	Output     repairResultData `json:"output"`
}

type repairMultiResult struct {
	OperationID string               `json:"operation_id,omitempty"`
	Batch       bool                 `json:"batch"`
	Status      string               `json:"status"`
	Succeeded   int                  `json:"succeeded"`
	Failed      int                  `json:"failed"`
	Plugin      string               `json:"plugin"`
	DryRun      bool                 `json:"dry_run"`
	Targets     []repairTargetResult `json:"targets"`
}

type repairRevisionRequest struct {
	binding  *domain.ClientBinding
	sequence uint64
	revision string
	source   string
	targets  []domain.ClientID
}

type repairManySession struct {
	ctx              context.Context
	cmd              *cobra.Command
	app              App
	opts             *options
	installation     domain.Installation
	targets          []domain.ClientID
	requests         map[string]*repairRevisionRequest
	targetRevision   map[domain.ClientID]string
	recordedBindings map[domain.ClientID]domain.ClientBinding
	keys             []string
	loadedByRevision map[string]loadedPackage
	detected         map[domain.ClientID]domain.DetectedClient
	inputs           []usecase.AddInput
	service          usecase.Service
	result           repairMultiResult
}

func runRepairMany(ctx context.Context, cmd *cobra.Command, app App, opts *options, selector string, targets []domain.ClientID) error {
	session := &repairManySession{ctx: ctx, cmd: cmd, app: app, opts: opts, targets: targets}
	if err := session.loadInstallation(selector); err != nil {
		return err
	}
	if err := session.collectRevisionRequests(); err != nil {
		return err
	}
	if err := session.preflight(); err != nil {
		return err
	}
	defer session.cleanupLoaded()
	if err := session.resolvePackages(); err != nil {
		return err
	}
	if err := session.authorizePackages(); err != nil {
		return err
	}
	if err := session.buildInputs(); err != nil {
		return err
	}
	return session.planAndApply()
}

func (session *repairManySession) loadInstallation(selector string) error {
	state, err := session.app.StateStore.Load()
	if err != nil {
		return err
	}
	installation, err := selectInstallation(state, selector)
	if err != nil {
		return err
	}
	if installation.NeedsRebind || installation.Package.LoaderKind != domain.LoaderKindAgentPlugins {
		return fmt.Errorf("repair requires a bound Agent Plugins installation")
	}
	session.installation = installation
	return nil
}

func (session *repairManySession) collectRevisionRequests() error {
	session.requests = map[string]*repairRevisionRequest{}
	session.targetRevision = make(map[domain.ClientID]string, len(session.targets))
	session.recordedBindings = make(map[domain.ClientID]domain.ClientBinding, len(session.targets))
	for _, target := range session.targets {
		if err := session.recordTargetRevision(target); err != nil {
			return err
		}
	}
	return nil
}

func (session *repairManySession) recordTargetRevision(target domain.ClientID) error {
	binding, err := selectedRepairBinding(session.installation, target, session.app.scope())
	if err != nil {
		return fmt.Errorf("preflight target %s: %w; no target was changed", target, err)
	}
	if binding.PackageRevision == nil {
		return fmt.Errorf("repair target %s has no recorded package revision", target)
	}
	session.recordedBindings[target] = binding
	key, request, err := session.revisionKey(binding)
	if err != nil {
		return err
	}
	session.targetRevision[target] = key
	if existing := session.requests[key]; existing != nil {
		existing.targets = expandAffectedSurfaceTargets(append(existing.targets, bindingSurfaceTargets(binding)...), session.app.scope())
		return nil
	}
	request.targets = expandAffectedSurfaceTargets(bindingSurfaceTargets(binding), session.app.scope())
	session.requests[key] = &request
	return nil
}

func (session *repairManySession) revisionKey(binding domain.ClientBinding) (string, repairRevisionRequest, error) {
	request := repairRevisionRequest{}
	var key string
	if session.installation.OriginMode == domain.OriginModeDirectory {
		request.sequence = binding.PackageRevision.ReleaseSequence
		request.revision = binding.PackageRevision.ResolvedRevision
		key = "directory:" + binding.PackageRevision.DistributionID + ":" + strconv.FormatUint(request.sequence, 10)
	} else {
		source, err := repairSource(session.installation, binding)
		if err != nil {
			return "", request, err
		}
		request.source = source
		if session.installation.Source.SourceKind != "" {
			cloned := binding
			request.binding = &cloned
		}
		key = "direct:" + source
	}
	key += ":" + binding.PackageRevision.ResolvedRevision + ":" + binding.PackageRevision.TreeDigest + ":" + binding.PackageRevision.ManifestDigest
	if binding.PackageRevision.CatalogEvidence != nil {
		key += ":" + binding.PackageRevision.CatalogEvidence.Digest
	}
	return key, request, nil
}

func (session *repairManySession) preflight() error {
	_, detected, err := preflightProjectLifecycleTargets(
		session.ctx, session.app, session.opts, session.targets, nil,
		!session.opts.dryRun && session.installation.OriginMode == domain.OriginModeDirectory,
		lifecycleInstallIntents(session.installation, session.opts.scope, nil),
	)
	if err != nil {
		return err
	}
	session.detected = detected
	return nil
}

func (session *repairManySession) resolvePackages() error {
	writeProgress(session.app, session.opts.format, "Resolving and validating each unique exact installed package revision once...")
	keys := make([]string, 0, len(session.requests))
	for key := range session.requests {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	session.keys = keys
	session.loadedByRevision = make(map[string]loadedPackage, len(keys))
	for _, key := range keys {
		loaded, err := session.resolveRevision(key)
		if err != nil {
			return err
		}
		session.loadedByRevision[key] = loaded
	}
	return nil
}

func (session *repairManySession) resolveRevision(key string) (loadedPackage, error) {
	request := session.requests[key]
	return resolveRepairSource(session.ctx, repairSourceResolutionTimeout, func(resolutionCtx context.Context) (loadedPackage, error) {
		if session.installation.OriginMode == domain.OriginModeDirectory {
			return session.app.loadInstalledPackage(resolutionCtx, session.installation, request.targets, domain.DirectoryRepair, request.sequence, request.revision, session.detected)
		}
		if request.binding != nil {
			return session.app.loadRecordedDirect(resolutionCtx, session.installation, *request.binding)
		}
		return session.app.loadPackageFor(resolutionCtx, request.source, packageResolutionRequest{
			Targets: request.targets, Operation: domain.DirectoryRepair, Clients: session.detected,
		})
	})
}

func (session *repairManySession) cleanupLoaded() {
	for index := len(session.keys) - 1; index >= 0; index-- {
		loaded, ok := session.loadedByRevision[session.keys[index]]
		if !ok || loaded.cleanup == nil {
			continue
		}
		_ = loaded.cleanup()
	}
}
