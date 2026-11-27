package agentpluginscli

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/commandext"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Real Directory signature verification/client, public Cobra commands and managed
// transactions; immutable source acquisition and detected Cursor are controlled
// fixtures. This is not production-issuer, real Git transport or vendor proof.
type projectionDirectoryFixture struct {
	rolloutDirectoryFixture
	snapshot domain.DirectorySnapshot
	command  string
	guide    string
	foreign  string
	data     string
	expected string
}

func newProjectionDirectoryFixture(t *testing.T, rewind bool) projectionDirectoryFixture {
	t.Helper()
	r := newRolloutDirectoryFixture(t, []domain.ClientID{domain.ClientCursor}, nil)
	r.retainV1Only()
	writeCommandFixtureFile(t, filepath.Join(r.v1.root, filepath.FromSlash(commandext.RelativeDirectory), "echo.md"),
		"---\ndescription: Echo the text\ndisable-model-invocation: false\n---\nReply with UAP-ECHO.\n")
	writeCommandFixtureFile(t, filepath.Join(r.v1.root, "skills", "guide", "SKILL.md"),
		"---\nname: guide\ndescription: Unrelated guide\n---\nRetain this guide.\n")
	dependencyWrite(t, filepath.Join(r.v1.root, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"demo":{"type":"stdio","command":"go","args":["version"],"env":{"DATA":"${PLUGIN_DATA}"}}}}`)
	loaded, err := r.cli.app.acquireLocal(t.Context(), r.v1.root)
	if err != nil {
		t.Fatal(err)
	}
	r.v1.tree, r.v1.manifest = loaded.envelope.TreeDigest, loaded.envelope.ManifestDigest
	if err := loaded.cleanup(); err != nil {
		t.Fatal(err)
	}
	snapshot := r.directory.bundle.Snapshot
	snapshot.Sequence, snapshot.PublicationID = 44, "projection-test-44"
	snapshot.GeneratedAt, snapshot.ExpiresAt = "2026-08-22T00:00:00Z", "2026-08-23T00:00:00Z"
	snapshot.Products[0].Categories = []string{"tools"}
	snapshot.Products[0].MinimumCapabilities.Skills = "required"
	release := &snapshot.Distributions[0].Releases[0]
	release.TreeDigest, release.ManifestDigest = r.v1.tree, r.v1.manifest
	release.Components = []string{"skills", "extensions", "mcp"}
	for index := range snapshot.Evidence {
		snapshot.Evidence[index].PackageTreeDigest = r.v1.tree
	}
	f := projectionDirectoryFixture{rolloutDirectoryFixture: r, snapshot: snapshot}
	f.useSignedSnapshot(t)
	output, _, err := f.cli.execute(false, "add", "--no-save", "rollout-demo", "--target", "cursor")
	if err != nil {
		t.Fatalf("signed immutable command fixture did not install: %v\n%s", err, output)
	}
	state, err := f.cli.store.Load()
	if err != nil || len(state.Installations) != 1 || len(state.EnvironmentReceipts) != 0 {
		t.Fatalf("fixture must be one imperative installation: %+v, %v", state, err)
	}
	installation := state.Installations[0]
	binding := onlyCLIClient(installation)
	f.command = filepath.Join(binding.TargetLocator, "skills", "demo-echo", "SKILL.md")
	f.guide = filepath.Join(binding.TargetLocator, "skills", "guide", "SKILL.md")
	f.expected = dependencyRead(t, f.command)
	if !strings.Contains(f.expected, "UAP-ECHO") || len(installation.DataReceipts) != 1 {
		t.Fatal("fixture lacks rendered command or persistent data authority")
	}
	for _, receipt := range installation.DataReceipts {
		f.data = filepath.Join(receipt.Locator, "keep.txt")
	}
	dependencyWrite(t, f.data, "persistent data sentinel\n")
	clients, err := f.cli.app.Detector.Detect(t.Context())
	if err != nil || len(clients) != 1 {
		t.Fatal(clients, err)
	}
	f.foreign = filepath.Join(clients[0].ConfigRoot, "skills", "foreign", "SKILL.md")
	writeCommandFixtureFile(t, f.foreign, "foreign skill sentinel\n")
	if rewind {
		rewindCommandProjection(t, f.cli, "demo-echo")
		if _, err := os.Stat(f.command); !os.IsNotExist(err) {
			t.Fatalf("old projection still contains the generated command: %v", err)
		}
	}
	return f
}

// Sign only the immutable test tuple, never production trust or source bytes.
func (f *projectionDirectoryFixture) useSignedSnapshot(t *testing.T) {
	t.Helper()
	body, err := json.Marshal(f.snapshot)
	if err != nil {
		t.Fatal(err)
	}
	body = append(body, '\n')
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{29}, ed25519.SeedSize))
	trust := directoryv1.TrustStore{Keys: []directoryv1.TrustedKey{{
		ID: "projection-test-only", PublicKey: private.Public().(ed25519.PublicKey), State: directoryv1.KeyCurrent,
	}}}
	digest := sha256.Sum256(body)
	envelope, err := json.Marshal(directoryv1.Envelope{
		EnvelopeSchemaVersion: 1, SnapshotSchemaVersion: 1, Sequence: f.snapshot.Sequence,
		KeyID: trust.Keys[0].ID, Algorithm: "Ed25519", SignatureDomain: directoryv1.SignatureDomain,
		SnapshotDigest: "sha256:" + hex.EncodeToString(digest[:]),
		Signature:      base64.StdEncoding.EncodeToString(ed25519.Sign(private, directoryv1.SnapshotSignatureMessage(body))),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := directoryv1.VerifyBundle(body, envelope, trust); err != nil {
		t.Fatalf("test Directory tuple is invalid: %v", err)
	}
	tampered := bytes.Replace(body, []byte("projection-test-44"), []byte("projection-test-45"), 1)
	if bytes.Equal(body, tampered) || !json.Valid(tampered) {
		t.Fatal("signature negative must alter valid JSON without resigning")
	}
	if _, err := directoryv1.VerifyBundle(tampered, envelope, trust); err == nil {
		t.Fatal("production verifier accepted tampered test Directory")
	}
	client := &directoryv1.Client{
		Origin: "https://projection.invalid/registry/schemas/1/", Trust: trust,
		Embedded:                 directoryv1.EmbeddedBundle{Snapshot: body, Envelope: envelope},
		RequireEmbeddedBootstrap: true, Cache: directoryv1.Cache{Path: filepath.Join(t.TempDir(), "directory.json")},
		HTTPClient: &http.Client{Transport: projectionDirectoryTransport{sequence: f.snapshot.Sequence, snapshot: body, envelope: envelope}},
		Now:        func() time.Time { return time.Date(2026, 8, 22, 1, 0, 0, 0, time.UTC) },
	}
	f.cli.app.DirectoryClient = client
}

type projectionDirectoryTransport struct {
	sequence uint64
	snapshot []byte
	envelope []byte
}

func (transport projectionDirectoryTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	stem := fmt.Sprintf("%020d", transport.sequence)
	pointer, err := json.Marshal(directoryv1.Pointer{
		PointerSchemaVersion: 1, SnapshotSchemaVersion: 1, Sequence: transport.sequence,
		SnapshotPath: "snapshots/" + stem + ".json", EnvelopePath: "snapshots/" + stem + ".envelope.json",
		FetchContract: directoryv1.FetchContract{HTTPSRequired: true, SameOriginRedirectsOnly: true, MaxRedirects: 2,
			LatestMaxBytes: directoryv1.MaxLatestBytes, SnapshotMaxBytes: directoryv1.MaxSnapshotBytes,
			EnvelopeMaxBytes: directoryv1.MaxEnvelopeBytes, RetryAttempts: 1},
	})
	if err != nil {
		return nil, err
	}
	status, body := http.StatusOK, pointer
	switch {
	case request.Method != http.MethodGet:
		status, body = http.StatusMethodNotAllowed, nil
	case strings.HasSuffix(request.URL.Path, "/latest.json"):
	case strings.HasSuffix(request.URL.Path, "/snapshots/"+stem+".json"):
		body = transport.snapshot
	case strings.HasSuffix(request.URL.Path, "/snapshots/"+stem+".envelope.json"):
		body = transport.envelope
	default:
		status, body = http.StatusNotFound, nil
	}
	return &http.Response{StatusCode: status, Header: make(http.Header),
		Body: io.NopCloser(bytes.NewReader(body)), Request: request}, nil
}

func (f projectionDirectoryFixture) installation(t *testing.T) domain.Installation {
	t.Helper()
	state, err := f.cli.store.Load()
	if err != nil || len(state.Installations) != 1 || len(state.EnvironmentReceipts) != 0 {
		t.Fatalf("imperative installation/ownership changed: %+v, %v", state, err)
	}
	return state.Installations[0]
}

func (f projectionDirectoryFixture) assertPreserved(t *testing.T, before domain.Installation) {
	t.Helper()
	after := f.installation(t)
	if before.InstallationID != after.InstallationID || !reflect.DeepEqual(before.Source, after.Source) ||
		!reflect.DeepEqual(before.Package, after.Package) || !reflect.DeepEqual(before.Directory, after.Directory) ||
		!reflect.DeepEqual(before.DataReceipts, after.DataReceipts) ||
		!reflect.DeepEqual(onlyCLIClient(before).PackageRevision, onlyCLIClient(after).PackageRevision) ||
		onlyCLIClient(before).ClientBindingID != onlyCLIClient(after).ClientBindingID {
		t.Fatal("same-release update changed immutable identity, owner or data receipts")
	}
	if dependencyRead(t, f.foreign) != "foreign skill sentinel\n" ||
		dependencyRead(t, f.data) != "persistent data sentinel\n" ||
		!strings.Contains(dependencyRead(t, f.guide), "Retain this guide.") {
		t.Fatal("update changed foreign, persistent data or unrelated projection bytes")
	}
}

func TestUpdateAllCurrentDirectoryRefreshesOlderCommandProjection(t *testing.T) {
	for _, mode := range []string{"targeted", "batch"} {
		t.Run(mode, func(t *testing.T) {
			f := newProjectionDirectoryFixture(t, true)
			before := f.installation(t)
			files := savedCheckFiles(t, f.cli.root)
			args := []string{"update", "demo", "--target", "cursor", "--format", "json"}
			if mode == "batch" {
				args = []string{"update", "--all", "--format", "json"}
			}
			preview, _, err := f.cli.execute(false, append(args, "--dry-run")...)
			if err != nil || !reflect.DeepEqual(files, savedCheckFiles(t, f.cli.root)) {
				t.Fatalf("signed projection preview mutated or refused: %v\n%s", err, preview)
			}
			output, _, err := f.cli.execute(false, args...)
			if err != nil {
				t.Fatalf("same-release refresh refused instead of refreshing: %v\n%s", err, output)
			}
			if mode == "batch" {
				result := decodeUpdateAllSourceResult(t, output)
				if result.Updated != 1 || result.Skipped != 0 || result.Status != "completed" {
					t.Errorf("current release freshness bypassed the pending projection refresh: %s", output)
				}
			}
			if body, err := os.ReadFile(f.command); err != nil || string(body) != f.expected {
				t.Fatalf("public %s update did not restore independently captured command bytes: %v\n%s", mode, err, output)
			}
			f.assertPreserved(t, before)
			info, err := os.Stat(f.command)
			if err != nil {
				t.Fatal(err)
			}
			repeat, _, err := f.cli.execute(false, args...)
			if err != nil {
				t.Fatalf("repeat refresh: %v\n%s", err, repeat)
			}
			if mode == "batch" {
				result := decodeUpdateAllSourceResult(t, repeat)
				if result.Updated != 0 || result.Skipped != 1 || len(result.Installations) != 1 ||
					result.Installations[0].Plan == nil || !selectedUpdatesUnchanged(*result.Installations[0].Plan) {
					t.Fatalf("repeat must report authoritative applied no-change: %s", repeat)
				}
			}
			after, err := os.Stat(f.command)
			if err != nil || !os.SameFile(info, after) || !info.ModTime().Equal(after.ModTime()) ||
				dependencyRead(t, f.command) != f.expected {
				t.Fatal("repeat replaced or changed the current projection", err)
			}
			f.assertPreserved(t, before)
		})
	}
}

func TestCurrentDirectoryHealthyCommandUpdateIsNoChange(t *testing.T) {
	f := newProjectionDirectoryFixture(t, false)
	before := f.installation(t)
	info, err := os.Stat(f.command)
	if err != nil {
		t.Fatal(err)
	}
	output, _, err := f.cli.execute(false, "update", "demo", "--target", "cursor", "--format", "json")
	var result struct {
		Data updateMultiResult `json:"data"`
	}
	if decodeErr := json.Unmarshal([]byte(output), &result); decodeErr != nil {
		t.Fatal(decodeErr, output)
	}
	if err != nil || !selectedUpdatesUnchanged(result.Data) {
		t.Fatalf("healthy same-release update was not authoritative no-change: %v\n%s", err, output)
	}
	after, err := os.Stat(f.command)
	if err != nil || !os.SameFile(info, after) || !info.ModTime().Equal(after.ModTime()) {
		t.Fatal("healthy command tree was replaced", err)
	}
	f.assertPreserved(t, before)
}

func TestUpdateAllCurrentDirectoryProjectionRetainsRefusals(t *testing.T) {
	for _, scenario := range []string{"revoked", "suspended", "unknown", "foreign-drift", "recorded-pin", "security"} {
		t.Run(scenario, func(t *testing.T) {
			f := newProjectionDirectoryFixture(t, true)
			before := f.installation(t)
			var security *updateAllBlockingSecurity
			switch scenario {
			case "revoked":
				f.snapshot.Sequence++
				f.snapshot.Revocations = []domain.DirectoryRevocation{{DistributionID: "owner/rollout", ReleaseSequence: 1}}
				f.useSignedSnapshot(t)
			case "suspended":
				f.snapshot.Sequence++
				f.snapshot.Distributions[0].Status = domain.DistributionSuspended
				f.useSignedSnapshot(t)
			case "unknown":
				f.cli.app.DirectoryClient = &fixedDirectoryClient{err: errors.New("Directory unavailable")}
			case "foreign-drift":
				dependencyWrite(t, f.guide, "foreign edit must not be overwritten\n")
			case "recorded-pin":
				f.snapshot.Sequence++
				f.snapshot.Distributions[0].Releases[0].PackageSource.Repository = "other/rollout"
				f.useSignedSnapshot(t)
			case "security":
				security = &updateAllBlockingSecurity{name: "demo"}
				f.cli.app.SecurityEvaluator = security
				unchanged := savedCheckFiles(t, f.cli.root)
				control, _, controlErr := f.cli.execute(false, "update", "demo", "--target", "cursor", "--yes", "--format", "json")
				if controlErr == nil || security.calls == 0 || !strings.Contains(controlErr.Error(), "--accept-security-risk") {
					t.Fatalf("targeted preparation did not establish blocking security control: calls=%d err=%v\n%s", security.calls, controlErr, control)
				}
				if !reflect.DeepEqual(unchanged, savedCheckFiles(t, f.cli.root)) {
					t.Fatal("targeted security refusal changed persistent files")
				}
				security.calls = 0
			}
			files := savedCheckFiles(t, f.cli.root)
			output, _, err := f.cli.execute(false, "update", "--all", "--yes", "--format", "json")
			if err == nil {
				t.Errorf("current-release update silently bypassed %s refusal: %s", scenario, output)
			}
			if security != nil && security.calls == 0 {
				t.Error("current-release update never reached the security evaluator")
			}
			if !reflect.DeepEqual(files, savedCheckFiles(t, f.cli.root)) {
				t.Fatal("refused current-release update changed persistent files")
			}
			if _, err := os.Stat(f.command); !os.IsNotExist(err) {
				t.Fatal("refusal refreshed the command before complete authorization", err)
			}
			if !reflect.DeepEqual(before.Clients, f.installation(t).Clients) ||
				dependencyRead(t, f.foreign) != "foreign skill sentinel\n" ||
				dependencyRead(t, f.data) != "persistent data sentinel\n" {
				t.Fatal("refusal changed binding authority, foreign bytes or persistent data")
			}
		})
	}
}
