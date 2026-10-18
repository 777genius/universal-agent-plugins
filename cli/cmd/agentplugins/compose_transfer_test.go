package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
)

// A confirmed import must retain authenticated observations without overwriting
// a concurrent newer cache, accepting equivocation, or trusting staged metadata.
func TestDependencyTransferDirectoryPreviewCommit(t *testing.T) {
	for _, scenario := range []string{"promote", "absent", "concurrent higher", "conflict", "untrusted", "corrupt"} {
		t.Run(scenario, func(t *testing.T) {
			original, staged, bundle := transferDirectoryClients(t)
			if scenario != "absent" {
				if err := staged.Cache.Store(bundle, staged.Trust); err != nil {
					t.Fatal(err)
				}
			}
			before := prepareTransferDirectoryCommit(t, scenario, original, staged, bundle)
			err := commitDependencyDirectoryPreview(original, staged)
			if scenario == "conflict" {
				if !errors.Is(err, directoryv1.ErrSequenceConflict) {
					t.Fatalf("conflicting authenticated bytes accepted: %v", err)
				}
			} else if scenario == "untrusted" || scenario == "corrupt" {
				if err == nil {
					t.Fatal("unverified staged cache accepted")
				}
			} else if err != nil {
				t.Fatal(err)
			}
			actual, readErr := os.ReadFile(original.Cache.Path)
			if scenario == "promote" || scenario == "concurrent higher" {
				loaded, err := original.Cache.Load(original.Trust)
				if err != nil {
					t.Fatal(err)
				}
				wantedSequence := bundle.Snapshot.Sequence
				if scenario == "concurrent higher" {
					wantedSequence++
				}
				if loaded.Snapshot.Sequence != wantedSequence {
					t.Fatalf("observed sequence %d lost: got %d", wantedSequence, loaded.Snapshot.Sequence)
				}
			} else if before == nil {
				if !os.IsNotExist(readErr) {
					t.Fatalf("failed/unused promotion created durable cache: %v", readErr)
				}
			} else if readErr != nil || !bytes.Equal(before, actual) {
				t.Fatal("promotion overwrote concurrent conflicting cache")
			}
		})
	}
}

func transferDirectoryClients(t *testing.T) (*directoryv1.Client, *directoryv1.Client, directoryv1.VerifiedBundle) {
	t.Helper()
	original, err := newConformanceDirectoryClient(writeConformanceFixture(t, t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	original.Cache.Path = filepath.Join(t.TempDir(), "global", "cache.json")
	preview, err := cloneDependencyDirectoryClient(original, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	staged, ok := preview.(*directoryv1.Client)
	if !ok {
		t.Fatal("unexpected test preview client")
	}
	bundle, err := original.Embedded.Verify(original.Trust)
	if err != nil {
		t.Fatal(err)
	}
	return original, staged, bundle
}

func prepareTransferDirectoryCommit(t *testing.T, scenario string, original *directoryv1.Client, staged *directoryv1.Client, bundle directoryv1.VerifiedBundle) []byte {
	t.Helper()
	if scenario == "concurrent higher" || scenario == "conflict" {
		candidate := transferSignedDirectoryBundle(t, bundle, scenario == "concurrent higher", original.Trust)
		if err := original.Cache.Store(candidate, original.Trust); err != nil {
			t.Fatal(err)
		}
		body, err := os.ReadFile(original.Cache.Path)
		if err != nil {
			t.Fatal(err)
		}
		return body
	}
	if scenario == "untrusted" {
		original.Trust.Keys = append([]directoryv1.TrustedKey(nil), original.Trust.Keys...)
		original.Trust.Keys[0].PublicKey = ed25519.NewKeyFromSeed(bytes.Repeat([]byte{18}, ed25519.SeedSize)).Public().(ed25519.PublicKey)
	}
	if scenario == "corrupt" {
		if err := os.WriteFile(staged.Cache.Path, []byte("invalid cache"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return nil
}

func transferSignedDirectoryBundle(t *testing.T, previous directoryv1.VerifiedBundle, advance bool, trust directoryv1.TrustStore) directoryv1.VerifiedBundle {
	t.Helper()
	snapshot := previous.Snapshot
	snapshot.PublicationID += "-different"
	if advance {
		snapshot.Sequence++
	}
	body, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	var envelope directoryv1.Envelope
	if err := json.Unmarshal(previous.EnvelopeBytes, &envelope); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(body)
	envelope.Sequence, envelope.SnapshotDigest = snapshot.Sequence, "sha256:"+hex.EncodeToString(digest[:])
	privateKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{17}, ed25519.SeedSize))
	envelope.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, directoryv1.SnapshotSignatureMessage(body)))
	signed, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := directoryv1.VerifyBundle(body, signed, trust)
	if err != nil {
		t.Fatal(err)
	}
	return bundle
}
