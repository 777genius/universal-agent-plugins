package agentpluginscli

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func projectVersionDevin() domain.DetectedClient {
	return domain.DetectedClient{ClientID: domain.ClientWindsurf, DisplayName: "original Devin", Status: domain.DetectionDetected, ExecutablePath: "/fixture/bin/devin", Surfaces: []domain.ClientSurface{{ID: "devin_cli", Detected: true, Evidence: "original discovery"}}}
}

func TestProjectVersionUnionIsIDBoundAtomicAndPreservesEmptyCascadeRoot(t *testing.T) {
	for _, failure := range []bool{false, true} {
		kimi, devin := projectVersionClient(), projectVersionDevin()
		observedKimi, observedDevin := kimi, devin
		observedKimi.Version, observedDevin.Version = "2.3.1", "3000.11.205"
		observedDevin.DisplayName = "must not replace original display"
		if failure {
			observedDevin.ConfigRoot = "/invented/profile"
		}
		detector := &projectVersionDetector{probed: []domain.DetectedClient{observedKimi, observedDevin}}
		detected := map[domain.ClientID]domain.DetectedClient{domain.ClientKimi: kimi, domain.ClientWindsurf: devin}
		err := ensureSelectedProjectVersions(context.Background(), App{Scope: domain.ScopeProject, Detector: detector}, &options{}, []domain.ClientID{domain.ClientKimi, domain.ClientWindsurf, domain.ClientKimi}, detected)
		if (err != nil) != failure || detector.broadCalls != 0 || !reflect.DeepEqual(detector.targets, [][]domain.ClientID{{domain.ClientKimi, domain.ClientWindsurf}}) {
			t.Fatalf("finite union probe differs: %v %+v", err, detector)
		}
		wantKimi, wantDevin := kimi, devin
		if !failure {
			wantKimi.Version, wantDevin.Version = "2.3.1", "3000.11.205"
		}
		if !reflect.DeepEqual(detected, map[domain.ClientID]domain.DetectedClient{domain.ClientKimi: wantKimi, domain.ClientWindsurf: wantDevin}) {
			t.Fatal("version merge changed identity facts or partially accepted a failed batch")
		}
	}
}

func TestProjectDevinVersionProbeRejectsWrongIDAndUnboundResults(t *testing.T) {
	for _, kind := range []string{"missing", "wrong-id", "duplicate", "version", "status", "error", "executable", "config", "profile", "authority", "probe-error"} {
		t.Run(kind, func(t *testing.T) {
			original := projectVersionDevin()
			observed := original
			observed.Version = "3000.11.205"
			detector := &projectVersionDetector{probed: []domain.DetectedClient{observed}}
			switch kind {
			case "missing":
				detector.probed = nil
			case "wrong-id":
				detector.probed[0].ClientID = domain.ClientKimi
			case "duplicate":
				detector.probed = append(detector.probed, observed)
			case "version":
				detector.probed[0].Version = ""
			case "status":
				detector.probed[0].Status = domain.DetectionNotDetected
			case "error":
				detector.probed[0].DetectionError = errors.New("observed failure")
			case "executable":
				detector.probed[0].ExecutablePath += "-other"
			case "config":
				detector.probed[0].ConfigRoot = "/different/legacy"
			case "profile":
				detector.probed[0].ProfileNamespace = "different"
			case "authority":
				detector.probed[0].ProfileAuthority = &domain.ProfileAuthority{}
			case "probe-error":
				detector.err = errors.New("targeted version failure")
			}
			detected := map[domain.ClientID]domain.DetectedClient{domain.ClientWindsurf: original}
			if err := ensureSelectedProjectVersions(context.Background(), App{Scope: domain.ScopeProject, Detector: detector}, &options{}, []domain.ClientID{domain.ClientWindsurf}, detected); err == nil {
				t.Fatal("unbound Devin version was accepted")
			}
			if !reflect.DeepEqual(detected[domain.ClientWindsurf], original) || detector.broadCalls != 0 {
				t.Fatal("failed observation changed original authority")
			}
		})
	}
}

func TestProjectDevinVersionProbeKeepsLegacyPrimaryAndMutationBoundaries(t *testing.T) {
	for _, kind := range []string{"legacy-primary", "secondary-surface", "relative-executable", "known-version", "user", "dry-run", "unselected"} {
		t.Run(kind, func(t *testing.T) {
			client := projectVersionDevin()
			opts, scope, target := &options{}, domain.ScopeProject, domain.ClientWindsurf
			switch kind {
			case "legacy-primary", "secondary-surface":
				client.ExecutablePath = "/fixture/bin/windsurf"
				client.ConfigRoot = "/fixture/user/.codeium/windsurf"
				if kind == "legacy-primary" {
					client.Surfaces = nil
				}
			case "relative-executable":
				client.ExecutablePath = "devin"
			case "known-version":
				client.Version = "3000.11.205"
			case "user":
				scope = domain.ScopeUser
			case "dry-run":
				opts.dryRun = true
			case "unselected":
				target = domain.ClientGemini
			}
			detector := &projectVersionDetector{err: errors.New("must not probe")}
			detected := map[domain.ClientID]domain.DetectedClient{domain.ClientWindsurf: client}
			if err := ensureSelectedProjectVersions(context.Background(), App{Scope: scope, Detector: detector}, opts, []domain.ClientID{target}, detected); err != nil || len(detector.targets) != 0 || detector.broadCalls != 0 {
				t.Fatalf("ineligible Devin operation executed a version probe: %v", err)
			}
		})
	}
}

func TestProjectDevinDependencyVersionsRespectCompleteActionSet(t *testing.T) {
	for _, kind := range []environment.DesiredAction{environment.ActionInstall, environment.ActionUpdate, "repair", environment.ActionUnchanged, environment.ActionRemove, "invalid-later-target"} {
		t.Run(string(kind), func(t *testing.T) {
			client := projectVersionDevin()
			observed := client
			observed.Version = "3000.11.205"
			detector := &projectVersionDetector{probed: []domain.DetectedClient{observed}}
			adapter := &dependencyInstaller{app: App{Scope: domain.ScopeProject, Detector: detector}, opts: &options{}, detected: map[domain.ClientID]domain.DetectedClient{domain.ClientWindsurf: client}}
			actions := []dependencyAction{{ClientID: "windsurf", Action: kind}}
			if kind == "invalid-later-target" {
				actions = []dependencyAction{{ClientID: "windsurf", Action: environment.ActionInstall}, {ClientID: "gemini", Action: environment.ActionInstall}}
			}
			err := adapter.preflightProjectVersions(context.Background(), actions)
			wantProbe := kind == environment.ActionInstall || kind == environment.ActionUpdate || kind == "repair"
			if (err != nil) != (kind == "invalid-later-target") || (len(detector.targets) == 1) != wantProbe || detector.broadCalls != 0 {
				t.Fatalf("dependency version gate differs: %v %+v", err, detector)
			}
		})
	}
}
