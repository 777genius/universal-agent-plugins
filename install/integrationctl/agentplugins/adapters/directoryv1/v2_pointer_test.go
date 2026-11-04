package directoryv1

import (
	"encoding/json"
	"errors"
	"testing"
)

func v2PointerFixture(t *testing.T) Pointer {
	t.Helper()
	p := Pointer{
		PointerSchemaVersion: 2, SnapshotSchemaVersion: 2, Sequence: 1,
		SnapshotPath: "snapshots/00000000000000000001.json",
		EnvelopePath: "snapshots/00000000000000000001.envelope.json",
		FetchContract: FetchContract{
			HTTPSRequired: true, SameOriginRedirectsOnly: true,
			ForwardCredentialsOnRedirect: false, MaxRedirects: 2,
			LatestMaxBytes: MaxLatestBytes, SnapshotMaxBytes: MaxSnapshotBytes,
			EnvelopeMaxBytes: MaxEnvelopeBytes, RetryAttempts: 3,
		},
	}
	return p
}

func TestV2PointerValidationAndExactSignedBundle(t *testing.T) {
	pointer := v2PointerFixture(t)
	wire, err := json.Marshal(pointer)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseV2Pointer(wire)
	if err != nil || got.Sequence != 1 {
		t.Fatalf("signed schema-2 pointer rejected: %+v %v", got, err)
	}
	snapshot, envelope, trust := v2SignedFixture(t)
	if b, err := VerifyV2PointerBundle(got, snapshot, envelope, trust); err != nil ||
		b.Snapshot.SnapshotSchemaVersion != 2 {
		t.Fatalf("valid v2 signed bytes rejected: %+v %v", b, err)
	}
	legacy := got
	legacy.PointerSchemaVersion, legacy.SnapshotSchemaVersion = 1, 1
	body, _ := json.Marshal(legacy)
	if _, err := ParseV2Pointer(body); !errors.Is(err, ErrStrictJSON) {
		t.Fatalf("v1 pointer authorized v2: %v", err)
	}
	if _, err := VerifyV2PointerBundle(legacy, snapshot, envelope, trust); !errors.Is(err, ErrSequenceMismatch) {
		t.Fatalf("unversioned pointer accepted: %v", err)
	}
}

func TestV2PointerRejectsCredentialAndPathDowngrades(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Pointer)
	}{
		{"wrong high water", func(p *Pointer) { p.Sequence = 2 }},
		{"absolute path", func(p *Pointer) { p.SnapshotPath = "/snapshots/00000000000000000001.json" }},
		{"redirect cookies", func(p *Pointer) { p.FetchContract.ForwardCredentialsOnRedirect = true }},
		{"unbounded retries", func(p *Pointer) { p.FetchContract.RetryAttempts = 4 }},
		{"HTTP downgrade", func(p *Pointer) { p.FetchContract.HTTPSRequired = false }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pointer := v2PointerFixture(t)
			tc.mutate(&pointer)
			data, err := json.Marshal(pointer)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ParseV2Pointer(data); !errors.Is(err, ErrStrictJSON) {
				t.Fatalf("unsafe v2 pointer accepted: %v", err)
			}
		})
	}
}
