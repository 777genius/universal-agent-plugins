package securityv1

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/securityscan"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const (
	treeDigest     = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	manifestDigest = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

type fixture struct {
	pointer, snapshot, envelope []byte
	trust                       TrustStore
}

func makeFixture(t *testing.T) fixture {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	requirement := securityscan.DefaultRequirement()
	snapshot := Snapshot{
		SecuritySchemaVersion: 1,
		Sequence:              7,
		PublicationID:         "security-test-7",
		SourceCommit:          strings.Repeat("c", 40),
		GeneratedAt:           "2026-09-05T00:00:00Z",
		ExpiresAt:             "2026-10-05T00:00:00Z",
		Complete:              true,
		Discovery:             DiscoveryIdentity{Sequence: 20, SnapshotDigest: "sha256:" + strings.Repeat("d", 64)},
		Scanner:               requirement.Scanner,
		Policy:                requirement.Policy,
		Coverage:              Coverage{Subjects: 1, Checked: 1},
		Records: []Record{{
			Subject: domain.SecuritySubject{TreeDigest: treeDigest, ManifestDigest: manifestDigest},
			Outcome: "warnings", Counts: domain.SecurityCounts{Warnings: 1, Total: 1}, ScannedFiles: 2,
			ReportDigest: "sha256:" + strings.Repeat("e", 64),
			Findings: []domain.SecurityFinding{{
				Code: "SEC301", Disposition: "warning", Severity: "warn", Confidence: "high",
				Category: "security", Path: "mcp.json", Line: 2, Message: "review this endpoint",
			}},
		}},
	}
	snapshotBody, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	snapshotBody = append(snapshotBody, '\n')
	digest := sha256.Sum256(snapshotBody)
	envelopeBody, err := json.Marshal(Envelope{
		EnvelopeSchemaVersion: 1, SnapshotSchemaVersion: 1, Sequence: 7,
		KeyID: "security-test", Algorithm: "Ed25519", SignatureDomain: SignatureDomain,
		SnapshotDigest: "sha256:" + hex.EncodeToString(digest[:]),
		Signature:      base64.StdEncoding.EncodeToString(ed25519.Sign(private, SignatureMessage(snapshotBody))),
	})
	if err != nil {
		t.Fatal(err)
	}
	envelopeBody = append(envelopeBody, '\n')
	pointerBody, err := json.Marshal(Pointer{
		PointerSchemaVersion: 1, SnapshotSchemaVersion: 1, Sequence: 7,
		SnapshotPath: "snapshots/00000000000000000007.json",
		EnvelopePath: "snapshots/00000000000000000007.envelope.json",
		FetchContract: FetchContract{MaxRedirects: 0, LatestMaxBytes: MaxLatestBytes,
			SnapshotMaxBytes: MaxSnapshotBytes, EnvelopeMaxBytes: MaxEnvelopeBytes, RetryAttempts: 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	return fixture{pointer: append(pointerBody, '\n'), snapshot: snapshotBody, envelope: envelopeBody,
		trust: TrustStore{KeyID: "security-test", PublicKey: public}}
}

func TestVerifyAndLookupExactSubject(t *testing.T) {
	value := makeFixture(t)
	snapshot, err := Verify(value.pointer, value.snapshot, value.envelope, value.trust)
	if err != nil {
		t.Fatal(err)
	}
	assessment := Lookup(snapshot, domain.SecuritySubject{TreeDigest: treeDigest, ManifestDigest: manifestDigest})
	if assessment == nil || assessment.Outcome != domain.SecurityWarnings || assessment.Counts.Warnings != 1 {
		t.Fatalf("exact assessment = %+v", assessment)
	}
	if Lookup(snapshot, domain.SecuritySubject{TreeDigest: "sha256:" + strings.Repeat("f", 64), ManifestDigest: manifestDigest}) != nil {
		t.Fatal("a changed tree digest reused an assessment")
	}
	if err := assessment.Validate(securityscan.DefaultRequirement(), assessment.Subject); err != nil {
		t.Fatalf("signed assessment does not satisfy the local contract: %v", err)
	}
}

func TestVerifyRejectsTamperingDuplicateKeysAndWrongDomain(t *testing.T) {
	value := makeFixture(t)
	tampered := append([]byte(nil), value.snapshot...)
	tampered[len(tampered)-2] ^= 1
	if _, err := Verify(value.pointer, tampered, value.envelope, value.trust); err != ErrDigestMismatch {
		t.Fatalf("tamper error = %v", err)
	}
	duplicate := []byte(strings.Replace(string(value.pointer), `"sequence":7`, `"sequence":7,"sequence":7`, 1))
	if _, err := ParsePointer(duplicate); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate key error = %v", err)
	}
	wrongDomain := append([]byte(nil), value.envelope...)
	wrongDomain = []byte(strings.Replace(string(wrongDomain), SignatureDomain, "UAP-DISCOVERY-INDEX-ED25519-V1", 1))
	if _, err := Verify(value.pointer, value.snapshot, wrongDomain, value.trust); err == nil {
		t.Fatal("wrong signature domain was accepted")
	}
}

func TestClientFetchesOnceAndFallsBackForUnknownSubject(t *testing.T) {
	value := makeFixture(t)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls++
		var body []byte
		switch request.URL.Path {
		case "/security/latest.json":
			body = value.pointer
		case "/security/snapshots/00000000000000000007.json":
			body = value.snapshot
		case "/security/snapshots/00000000000000000007.envelope.json":
			body = value.envelope
		default:
			http.NotFound(writer, request)
			return
		}
		_, _ = writer.Write(body)
	}))
	defer server.Close()
	client := &Client{Origin: server.URL + "/security/", HTTPClient: server.Client(), Trust: value.trust,
		AllowHTTPForTests: true, Now: func() time.Time { return time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC) }}
	assessment, err := client.Lookup(context.Background(), domain.SecuritySubject{TreeDigest: treeDigest, ManifestDigest: manifestDigest})
	if err != nil || assessment == nil {
		t.Fatalf("first lookup = %+v, %v", assessment, err)
	}
	missing, err := client.Lookup(context.Background(), domain.SecuritySubject{TreeDigest: treeDigest, ManifestDigest: "sha256:" + strings.Repeat("f", 64)})
	if err != nil || missing != nil || calls != 3 {
		t.Fatalf("memoized lookup = %+v, %v calls=%d", missing, err, calls)
	}
}
