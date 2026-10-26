package agentpluginscli

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

type projectVersionDetector struct {
	clients, probed []domain.DetectedClient
	targets         [][]domain.ClientID
	err             error
	broadCalls      int
}

func (detector *projectVersionDetector) Detect(context.Context) ([]domain.DetectedClient, error) {
	return append([]domain.DetectedClient(nil), detector.clients...), nil
}

func (detector *projectVersionDetector) DetectWithVersionProbe(context.Context) ([]domain.DetectedClient, error) {
	detector.broadCalls++
	return nil, errors.New("broad version probes must not execute")
}

func (detector *projectVersionDetector) DetectTargetsWithVersionProbe(_ context.Context, targets []domain.ClientID) ([]domain.DetectedClient, error) {
	detector.targets = append(detector.targets, append([]domain.ClientID(nil), targets...))
	return append([]domain.DetectedClient(nil), detector.probed...), detector.err
}

func projectVersionClient() domain.DetectedClient {
	return domain.DetectedClient{
		ClientID: domain.ClientKimi, DisplayName: "original Kimi", Status: domain.DetectionDetected,
		ExecutablePath: "/fixture/bin/kimi", ConfigRoot: "/fixture/user/.kimi", ProfileNamespace: "original-user",
		Surfaces: []domain.ClientSurface{{ID: "cli", Detected: true, Evidence: "original discovery"}},
	}
}

func TestProjectLifecycleVersionProbePreservesDiscovery(t *testing.T) {
	original := projectVersionClient()
	other := domain.DetectedClient{ClientID: domain.ClientGemini, Status: domain.DetectionDetected, Version: "ambient"}
	probed := original
	probed.Version, probed.DisplayName, probed.Surfaces = "2.3.1", "new unrelated display fact", nil
	detector := &projectVersionDetector{clients: []domain.DetectedClient{original, other}, probed: []domain.DetectedClient{
		probed, {ClientID: domain.ClientGemini, Version: "must not replace ambient discovery"},
	}}
	selected, detected, err := preflightProjectLifecycleTargets(context.Background(), App{Scope: domain.ScopeProject, Detector: detector}, &options{}, []domain.ClientID{domain.ClientKimi}, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	want := original
	want.Version = "2.3.1"
	if !reflect.DeepEqual(selected, []domain.DetectedClient{want}) || !reflect.DeepEqual(detected[domain.ClientKimi], want) || !reflect.DeepEqual(detected[domain.ClientGemini], other) {
		t.Fatalf("version probe replaced discovery facts: selected=%+v detected=%+v", selected, detected)
	}
	if !reflect.DeepEqual(detector.clients[0], original) {
		t.Fatal("version probe mutated the original discovery slice")
	}
	if !reflect.DeepEqual(detector.targets, [][]domain.ClientID{{domain.ClientKimi}}) || detector.broadCalls != 0 {
		t.Fatalf("unexpected probes: targeted=%v broad=%d", detector.targets, detector.broadCalls)
	}
}

func TestProjectLifecycleVersionProbeRejectsUnboundResults(t *testing.T) {
	tests := []struct {
		name   string
		change func(*projectVersionDetector)
	}{
		{"missing", func(d *projectVersionDetector) { d.probed = nil }},
		{"duplicate", func(d *projectVersionDetector) { d.probed = append(d.probed, d.probed[0]) }},
		{"empty version", func(d *projectVersionDetector) { d.probed[0].Version = "" }},
		{"blank version", func(d *projectVersionDetector) { d.probed[0].Version = " \t" }},
		{"not detected", func(d *projectVersionDetector) { d.probed[0].Status = domain.DetectionNotDetected }},
		{"detection error", func(d *projectVersionDetector) { d.probed[0].DetectionError = errors.New("failed detection") }},
		{"executable changed", func(d *projectVersionDetector) { d.probed[0].ExecutablePath += "-other" }},
		{"config root changed", func(d *projectVersionDetector) { d.probed[0].ConfigRoot += "-other" }},
		{"profile namespace changed", func(d *projectVersionDetector) { d.probed[0].ProfileNamespace += "-other" }},
		{"profile authority changed", func(d *projectVersionDetector) { d.probed[0].ProfileAuthority = &domain.ProfileAuthority{} }},
		{"probe error", func(d *projectVersionDetector) { d.err = errors.New("version command failed") }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			original := projectVersionClient()
			probed := original
			probed.Version = "2.3.1"
			detector := &projectVersionDetector{probed: []domain.DetectedClient{probed}}
			test.change(detector)
			detected := map[domain.ClientID]domain.DetectedClient{domain.ClientKimi: original}
			err := ensureSelectedProjectVersions(context.Background(), App{Scope: domain.ScopeProject, Detector: detector}, &options{}, []domain.ClientID{domain.ClientKimi}, detected)
			if err == nil {
				t.Fatal("unbound or unsuccessful version observation was accepted")
			}
			if !reflect.DeepEqual(detected, map[domain.ClientID]domain.DetectedClient{domain.ClientKimi: original}) {
				t.Fatal("failed version observation changed discovery facts")
			}
			if !reflect.DeepEqual(detector.targets, [][]domain.ClientID{{domain.ClientKimi}}) || detector.broadCalls != 0 {
				t.Fatalf("unexpected probes: targeted=%v broad=%d", detector.targets, detector.broadCalls)
			}
		})
	}
}

func TestProjectLifecycleVersionProbeEligibility(t *testing.T) {
	tests := []struct {
		name   string
		scope  domain.InstallScope
		dryRun bool
		target domain.ClientID
		change func(*domain.DetectedClient)
	}{
		{"user scope", domain.ScopeUser, false, domain.ClientKimi, nil},
		{"dry run", domain.ScopeProject, true, domain.ClientKimi, nil},
		{"unselected", domain.ScopeProject, false, domain.ClientGemini, nil},
		{"known version", domain.ScopeProject, false, domain.ClientKimi, func(c *domain.DetectedClient) { c.Version = "2.3.1" }},
		{"no executable", domain.ScopeProject, false, domain.ClientKimi, func(c *domain.DetectedClient) { c.ExecutablePath = "" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := projectVersionClient()
			if test.change != nil {
				test.change(&client)
			}
			detector := &projectVersionDetector{err: errors.New("must not probe")}
			detected := map[domain.ClientID]domain.DetectedClient{domain.ClientKimi: client}
			if err := ensureSelectedProjectVersions(context.Background(), App{Scope: test.scope, Detector: detector}, &options{dryRun: test.dryRun}, []domain.ClientID{test.target}, detected); err != nil {
				t.Fatal(err)
			}
			if len(detector.targets) != 0 || detector.broadCalls != 0 || !reflect.DeepEqual(detected[domain.ClientKimi], client) {
				t.Fatalf("ineligible operation probed or changed discovery: %+v", detector)
			}
		})
	}
}

type projectVersionBroadOnlyDetector struct {
	staticDetector
	calls int
}

func (detector *projectVersionBroadOnlyDetector) DetectWithVersionProbe(context.Context) ([]domain.DetectedClient, error) {
	detector.calls++
	return nil, errors.New("must not fall back to broad version detection")
}

func TestProjectLifecycleVersionProbeRequiresTargetedCapability(t *testing.T) {
	client := projectVersionClient()
	detector := &projectVersionBroadOnlyDetector{staticDetector: staticDetector{clients: []domain.DetectedClient{client}}}
	selected, _, err := preflightProjectLifecycleTargets(context.Background(), App{Scope: domain.ScopeProject, Detector: detector}, &options{}, []domain.ClientID{domain.ClientKimi}, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if detector.calls != 0 || len(selected) != 1 || selected[0].Version != "" {
		t.Fatal("missing targeted capability executed a broad probe or invented a version")
	}
}

func TestProjectDependencyVersionsValidateCompleteActionSet(t *testing.T) {
	tests := []struct {
		name      string
		actions   []dependencyAction
		wantProbe bool
		wantError bool
	}{
		{"install", []dependencyAction{{ClientID: "kimi", Action: environment.ActionInstall}}, true, false},
		{"update", []dependencyAction{{ClientID: "kimi", Action: environment.ActionUpdate}}, true, false},
		{"repair", []dependencyAction{{ClientID: "kimi", Action: "repair"}}, true, false},
		{"remove", []dependencyAction{{ClientID: "kimi", Action: environment.ActionRemove}}, false, false},
		{"unchanged", []dependencyAction{{ClientID: "kimi", Action: environment.ActionUnchanged}}, false, false},
		{"invalid later target", []dependencyAction{{ClientID: "kimi", Action: environment.ActionInstall}, {ClientID: "gemini", Action: environment.ActionInstall}}, false, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := projectVersionClient()
			probed := client
			probed.Version = "2.3.1"
			detector := &projectVersionDetector{probed: []domain.DetectedClient{probed}}
			adapter := &dependencyInstaller{app: App{Scope: domain.ScopeProject, Detector: detector}, opts: &options{}, detected: map[domain.ClientID]domain.DetectedClient{
				domain.ClientKimi: client, domain.ClientGemini: {ClientID: domain.ClientGemini, Status: domain.DetectionNotDetected},
			}}
			err := adapter.preflightProjectVersions(context.Background(), test.actions)
			if (err != nil) != test.wantError {
				t.Fatalf("error=%v wantError=%v", err, test.wantError)
			}
			if (len(detector.targets) == 1) != test.wantProbe || detector.broadCalls != 0 {
				t.Fatalf("probes=%v broad=%d wantProbe=%v", detector.targets, detector.broadCalls, test.wantProbe)
			}
			if test.wantProbe && (adapter.detected[domain.ClientKimi].Version != "2.3.1" || !reflect.DeepEqual(detector.targets[0], []domain.ClientID{domain.ClientKimi})) {
				t.Fatal("preview/apply map did not receive the selected version")
			}
		})
	}
}

// Skills preparation and Check use this shared preview adapter with ordinary
// skillsOptions, whose dryRun flag is false. Only the mutation-aware CLI wrapper
// may request a new version; preview must reach its existing preparation gate.
func TestProjectDependencyPreviewDoesNotProbeVersions(t *testing.T) {
	client := projectVersionClient()
	detector := &projectVersionDetector{err: errors.New("preview must not execute a client")}
	reachedPreparation := false
	boundary := errors.New("inert preview preparation boundary")
	adapter := &dependencyInstaller{
		app: App{Scope: domain.ScopeProject, Detector: detector}, opts: &options{},
		detected: map[domain.ClientID]domain.DetectedClient{domain.ClientKimi: client},
		beforeApply: func() error {
			reachedPreparation = true
			return boundary
		},
	}
	_, err := preflightDependencyAdapter(context.Background(), adapter, []dependencyAction{{ClientID: "kimi", Action: environment.ActionInstall}})
	if !errors.Is(err, boundary) || !reachedPreparation {
		t.Fatalf("preview did not retain its existing preparation gate: %v", err)
	}
	if len(detector.targets) != 0 || detector.broadCalls != 0 || !reflect.DeepEqual(adapter.detected[domain.ClientKimi], client) {
		t.Fatal("read-only Skills preview executed a client or replaced discovery facts")
	}
}
