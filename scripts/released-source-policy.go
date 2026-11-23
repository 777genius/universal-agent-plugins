//go:build ignore

// Harness-only adapter. Build this FILE with cwd and GOWORK in a detached,
// clean checkout of producerCommit; never build against the harness checkout.
package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const producerCommit = "4858d973afcf6cf31b49bba98bdf06482dce6226"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type policyOptions struct {
	root, cache, client, scope, target, digest, at string
	sequence                                    uint64
	observed                                    time.Time
}

func run() error {
	options, err := readOptions()
	if err != nil {
		return err
	}
	bundle, err := authenticatedProducerCache(options)
	if err != nil {
		return err
	}
	selections, ineligible, err := resolveDependencies(bundle.Snapshot, options)
	if err != nil {
		return err
	}
	return emitReport(bundle, options, selections, ineligible)
}

func readOptions() (policyOptions, error) {
	var options policyOptions
	flag.StringVar(&options.root, "producer-root", "", "exact producer checkout used for compilation")
	flag.StringVar(&options.cache, "cache", "", "held case-local Directory v1 cache copy")
	flag.StringVar(&options.client, "client", "", "selected native client")
	flag.StringVar(&options.scope, "scope", "", "user or project")
	flag.StringVar(&options.target, "target", "", "linux-amd64, darwin-arm64 or windows-amd64")
	flag.Uint64Var(&options.sequence, "sequence", 0, "exact runtime sequence joined to every child")
	flag.StringVar(&options.digest, "digest", "", "exact runtime digest joined to every child")
	flag.StringVar(&options.at, "at", "", "RFC3339 observation time")
	flag.Parse()
	if flag.NArg() != 0 || options.root == "" || options.cache == "" || options.sequence == 0 || options.digest == "" {
		return options, errors.New("all exact provenance inputs are required")
	}
	if err := validateTuple(options); err != nil {
		return options, err
	}
	observed, err := time.Parse(time.RFC3339, options.at)
	options.observed = observed
	return options, err
}

func validateTuple(options policyOptions) error {
	if (options.client != "hermes" && options.client != "antigravity" && options.client != "pi") ||
		(options.scope != "user" && options.scope != "project") ||
		(options.client == "hermes" && options.scope != "user") {
		return errors.New("unreviewed native tuple")
	}
	if options.target != "linux-amd64" && options.target != "darwin-arm64" && options.target != "windows-amd64" {
		return errors.New("unreviewed platform")
	}
	return nil
}

func authenticatedProducerCache(options policyOptions) (directoryv1.VerifiedBundle, error) {
	if err := requireProducer(options.root); err != nil {
		return directoryv1.VerifiedBundle{}, err
	}
	trust, err := producerTrust()
	if err != nil {
		return directoryv1.VerifiedBundle{}, err
	}
	embedded, err := producerBootstrap(options.root, trust)
	if err != nil {
		return embedded, err
	}
	bundle, err := authenticateCache(options.cache, trust, options.sequence, options.digest, options.observed)
	if err != nil {
		return bundle, err
	}
	return bundle, checkFloor(bundle, embedded)
}

func resolveDependencies(snapshot domain.DirectorySnapshot, options policyOptions) (map[string]any, []string, error) {
	selections := map[string]any{}
	ineligible := []string{}
	for _, dependency := range []struct {
		distribution string
		release      uint64
		version      string
	}{
		{"777genius/notion", 1, "0.1.0"},
		{"777genius/hubspot-crm", 1, "0.1.0"},
		{"777genius/playwright-bridge", 2, "0.0.80"},
	} {
		request := dependencyRequest(options, dependency.distribution, dependency.release)
		selected, err := selectForPlacement(snapshot, request)
		if errors.Is(err, domain.ErrDirectoryIneligible) {
			ineligible = append(ineligible, dependency.distribution)
			continue
		}
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", dependency.distribution, err)
		}
		if selected.DistributionID != dependency.distribution || selected.ReleaseSequence != dependency.release || selected.PackageVersion != dependency.version {
			return nil, nil, errors.New("policy selected a different immutable dependency release")
		}
		selections[dependency.distribution] = selectionReport(selected)
	}
	return selections, ineligible, nil
}

func dependencyRequest(options policyOptions, distribution string, release uint64) domain.DirectoryResolveRequest {
	platform := strings.Split(options.target, "-")
	return domain.DirectoryResolveRequest{
		Selector: distribution, DesiredReleaseSequence: release,
		Scope: domain.InstallScope(options.scope), Operation: domain.DirectoryInstall,
		SchemaVersion: "1.0.0", InstallerVersion: "0.1.116",
		OS: platform[0], Architecture: platform[1],
		Targets: []domain.ClientID{domain.ClientID(options.client)},
	}
}

func selectForPlacement(snapshot domain.DirectorySnapshot, request domain.DirectoryResolveRequest) (domain.DirectorySelection, error) {
	// V1 Project source placement authenticates a User publication; it never
	// turns that publication into a Project grant.
	if request.Scope == domain.ScopeProject {
		return domain.ResolveDirectoryV1ProjectSource(snapshot, request)
	}
	return domain.ResolveDirectory(snapshot, request)
}

func selectionReport(selected domain.DirectorySelection) map[string]any {
	return map[string]any{
		"distribution": selected.DistributionID, "release_sequence": selected.ReleaseSequence,
		"package_version": selected.PackageVersion, "repository": selected.Source.Repository,
		"package_path": selected.Source.Path, "resolved_revision": selected.Source.Revision,
		"tree_digest": selected.TreeDigest, "manifest_digest": selected.ManifestDigest,
	}
}

func emitReport(bundle directoryv1.VerifiedBundle, options policyOptions, selections map[string]any, ineligible []string) error {
	status := "eligible"
	if len(ineligible) != 0 {
		status = "ineligible"
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{
		"producer_commit": producerCommit, "snapshot_schema": 1,
		"snapshot_sequence": bundle.Snapshot.Sequence, "snapshot_digest": bundle.Digest,
		"key_id": bundle.Envelope.KeyID, "client": options.client, "scope": options.scope,
		"target": options.target, "observed_at": options.at, "signed_project_grant": false,
		"policy_status": status, "ineligible_distributions": ineligible, "selections": selections,
	})
}

func authenticateCache(path string, trust directoryv1.TrustStore, sequence uint64, digest string, observed time.Time) (directoryv1.VerifiedBundle, error) {
	bundle, err := (directoryv1.Cache{Path: path}).Load(trust)
	if err != nil {
		return bundle, err
	}
	if bundle.Snapshot.SnapshotSchemaVersion != 1 || bundle.Snapshot.Sequence != sequence || bundle.Digest != digest {
		return bundle, errors.New("cache does not authenticate the exact runtime lineage")
	}
	generated, err := time.Parse(time.RFC3339, bundle.Snapshot.GeneratedAt)
	if err != nil {
		return bundle, err
	}
	expires, err := time.Parse(time.RFC3339, bundle.Snapshot.ExpiresAt)
	if err != nil {
		return bundle, err
	}
	if observed.Before(generated) || !observed.Before(expires) {
		return bundle, errors.New("runtime publication outside validity window")
	}
	return bundle, nil
}

func producerTrust() (directoryv1.TrustStore, error) {
	trust := directoryv1.TrustStore{}
	for index, pair := range [][2]string{{"uap-directory-2026-01", "HalXARjat+v3ylTPLMAnvuavRo4ZfrF+DbWwsjlp2bI="}, {"uap-directory-2026-11", "k0F0ZlRXsgUa3+OMOUle6GdpihXUUcHI+42wNeuGAMc="}} {
		key, err := base64.StdEncoding.Strict().DecodeString(pair[1])
		if err != nil {
			return trust, err
		}
		state := directoryv1.KeyCurrent
		if index == 1 {
			state = directoryv1.KeyNext
		}
		trust.Keys = append(trust.Keys, directoryv1.TrustedKey{ID: pair[0], PublicKey: ed25519.PublicKey(key), State: state})
	}
	return trust, nil
}

func requireProducer(root string) error {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	head, err := exec.Command("git", "-C", absolute, "rev-parse", "HEAD").Output()
	if err != nil || strings.TrimSpace(string(head)) != producerCommit {
		return errors.New("policy adapter requires exact frozen producer checkout")
	}
	// Build orchestration must use this root/go.work. Clean tracked inputs
	// prevent a locally modified resolver from masquerading as producer policy.
	if err := exec.Command("git", "-C", absolute, "diff", "--exit-code", "HEAD", "--").Run(); err != nil {
		return errors.New("producer tracked inputs are modified")
	}
	status, err := exec.Command("git", "-C", absolute, "status", "--porcelain", "--untracked-files=all", "--ignored").Output()
	if err != nil || len(bytes.TrimSpace(status)) != 0 {
		return errors.New("producer checkout is not clean including untracked inputs")
	}
	return nil
}

func producerBootstrap(root string, trust directoryv1.TrustStore) (directoryv1.VerifiedBundle, error) {
	base := filepath.Join(root, "cli", "cmd", "agentplugins")
	snapshot, err := os.ReadFile(filepath.Join(base, "directory_bootstrap_inputs", "snapshot.json"))
	if err != nil {
		return directoryv1.VerifiedBundle{}, err
	}
	envelope, err := os.ReadFile(filepath.Join(base, "directory_bootstrap_inputs", "envelope.json"))
	if err != nil {
		return directoryv1.VerifiedBundle{}, err
	}
	bundle, err := directoryv1.VerifyBundle(snapshot, envelope, trust)
	if err != nil {
		return bundle, err
	}
	generated, err := directoryv1.GenerateReleaseBootstrapSource("main", "generatedProductionDirectoryBootstrap", snapshot, envelope, trust)
	if err != nil {
		return bundle, err
	}
	actual, err := os.ReadFile(filepath.Join(base, "directory_bootstrap_generated.go"))
	if err != nil || !bytes.Equal(generated, actual) {
		return bundle, errors.New("producer bootstrap inputs do not reproduce compiled authority")
	}
	return bundle, nil
}

func checkFloor(bundle, embedded directoryv1.VerifiedBundle) error {
	if bundle.Snapshot.Sequence < embedded.Snapshot.Sequence {
		return errors.New("runtime cache below producer bootstrap floor")
	}
	if bundle.Snapshot.Sequence == embedded.Snapshot.Sequence && bundle.Digest != embedded.Digest {
		return errors.New("runtime cache equivocates producer bootstrap")
	}
	return nil
}
