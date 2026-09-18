// Command bootstrapgen deterministically generates or verifies the production
// Directory bootstrap source from exact signed publication artifacts.
package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/cmd/agentplugins/internal/bootstrapio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "bootstrapgen:", err)
		os.Exit(1)
	}
}

type bootstrapFlags struct {
	snapshotPath      string
	envelopePath      string
	trustPath         string
	outputPath        string
	checkPath         string
	releaseAtText     string
	expectedKeyID     string
	expectedPublicKey string
}

func run(arguments []string) error {
	flags, err := parseBootstrapFlags(arguments)
	if err != nil {
		return err
	}
	bundle, trust, err := bootstrapio.LoadVerifiedBundle(flags.snapshotPath, flags.envelopePath, flags.trustPath)
	if err != nil {
		return err
	}
	if err := validateBootstrapInputs(bundle, trust, flags); err != nil {
		return err
	}
	source, err := directoryv1.GenerateReleaseBootstrapSource("main", "generatedProductionDirectoryBootstrap", bundle.SnapshotBytes, bundle.EnvelopeBytes, trust)
	if err != nil {
		return fmt.Errorf("generate verified Directory bootstrap: %w", err)
	}
	return writeBootstrapSource(source, flags)
}

func parseBootstrapFlags(arguments []string) (bootstrapFlags, error) {
	set := flag.NewFlagSet("bootstrapgen", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	snapshotPath := set.String("snapshot", "", "exact signed snapshot JSON")
	envelopePath := set.String("envelope", "", "exact detached envelope JSON")
	trustPath := set.String("trust", "", "reviewed trusted-keys JSON")
	outputPath := set.String("output", "", "write generated source to this path (default stdout)")
	checkPath := set.String("check", "", "require this generated source to reproduce exactly")
	releaseAtText := set.String("release-at", "", "require publication usable at this RFC3339 release time")
	expectedKeyID := set.String("expected-key-id", "", "require the envelope to bind this compiled production key ID")
	expectedPublicKey := set.String("expected-public-key", "", "require this compiled production Ed25519 public key")
	if err := set.Parse(arguments); err != nil {
		return bootstrapFlags{}, err
	}
	if set.NArg() != 0 || (*outputPath != "" && *checkPath != "") {
		return bootstrapFlags{}, fmt.Errorf("unexpected arguments or conflicting -output/-check")
	}
	return bootstrapFlags{
		snapshotPath: *snapshotPath, envelopePath: *envelopePath, trustPath: *trustPath,
		outputPath: *outputPath, checkPath: *checkPath, releaseAtText: *releaseAtText,
		expectedKeyID: *expectedKeyID, expectedPublicKey: *expectedPublicKey,
	}, nil
}

func validateBootstrapInputs(bundle directoryv1.VerifiedBundle, trust directoryv1.TrustStore, flags bootstrapFlags) error {
	if bundle.Snapshot.Sequence == 0 {
		return fmt.Errorf("signed Directory bootstrap sequence is zero")
	}
	if err := verifyExpectedProductionKey(bundle, trust, flags); err != nil {
		return err
	}
	return verifyReleaseWindow(bundle, flags.releaseAtText)
}

func verifyExpectedProductionKey(bundle directoryv1.VerifiedBundle, trust directoryv1.TrustStore, flags bootstrapFlags) error {
	if (flags.expectedKeyID == "") != (flags.expectedPublicKey == "") {
		return fmt.Errorf("-expected-key-id and -expected-public-key must be supplied together")
	}
	if flags.expectedKeyID == "" {
		return nil
	}
	decoded, err := base64.StdEncoding.Strict().DecodeString(flags.expectedPublicKey)
	if err != nil || len(decoded) != ed25519.PublicKeySize {
		return fmt.Errorf("-expected-public-key is not an Ed25519 public key")
	}
	if bundle.Envelope.KeyID != flags.expectedKeyID {
		return fmt.Errorf("signed envelope key %q does not match compiled production key %q", bundle.Envelope.KeyID, flags.expectedKeyID)
	}
	for _, key := range trust.Keys {
		if key.ID == flags.expectedKeyID && bytes.Equal(key.PublicKey, decoded) {
			return nil
		}
	}
	return fmt.Errorf("trust input does not contain the exact compiled production key binding")
}

func verifyReleaseWindow(bundle directoryv1.VerifiedBundle, releaseAtText string) error {
	if releaseAtText == "" {
		return nil
	}
	releaseAt, err := time.Parse(time.RFC3339, releaseAtText)
	if err != nil {
		return fmt.Errorf("parse -release-at: %w", err)
	}
	generatedAt, err := time.Parse(time.RFC3339, bundle.Snapshot.GeneratedAt)
	if err != nil {
		return fmt.Errorf("parse signed generated_at: %w", err)
	}
	expiresAt, err := time.Parse(time.RFC3339, bundle.Snapshot.ExpiresAt)
	if err != nil {
		return fmt.Errorf("parse signed expires_at: %w", err)
	}
	if releaseAt.Before(generatedAt) || !releaseAt.Before(expiresAt) {
		return fmt.Errorf("signed Directory bootstrap is not valid for release at %s (valid [%s, %s))", releaseAt.UTC().Format(time.RFC3339), generatedAt, expiresAt)
	}
	return nil
}

func writeBootstrapSource(source []byte, flags bootstrapFlags) error {
	if flags.checkPath != "" {
		checked, err := os.ReadFile(flags.checkPath)
		if err != nil {
			return fmt.Errorf("read checked bootstrap source: %w", err)
		}
		if !bytes.Equal(source, checked) {
			return fmt.Errorf("%s is not reproducibly generated from the exact verified inputs", flags.checkPath)
		}
		return nil
	}
	if flags.outputPath != "" {
		if err := os.WriteFile(flags.outputPath, source, 0o644); err != nil {
			return fmt.Errorf("write generated bootstrap source: %w", err)
		}
		return nil
	}
	_, err := os.Stdout.Write(source)
	return err
}
