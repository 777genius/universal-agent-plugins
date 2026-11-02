package environmentstore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestDependencyFilesSurviveSnapshotAndStoreAndRemainDigestProtected(t *testing.T) {
	ctx := context.Background()
	source := t.TempDir()
	files := map[string]string{
		"plugin.json": `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"dependency-artifact"}`,
		"uap.toml":    "schema_version = 1\n[dependencies]\n\"acme/docs\" = \"^1.2\"\n",
		"uap.lock":    "author-local-lock-bytes\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(source, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	snapshot, err := (packagedigest.Builder{TempRoot: t.TempDir()}).Snapshot(ctx, source, domain.SourceIdentity{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = packagedigest.Remove(snapshot) })
	store := Store{Root: filepath.Join(t.TempDir(), "store")}
	published, err := store.Put(ctx, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range files {
		got, err := os.ReadFile(filepath.Join(published.Root, name))
		if err != nil || string(got) != want {
			t.Fatalf("published %s = %q, %v; want exact authored bytes %q", name, got, err, want)
		}
	}
	// The portable manifest stays intact while either manager file is changed.
	// Neither an ignored nested lock nor TOML may escape package integrity checks.
	for _, name := range []string{"uap.toml", "uap.lock"} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(filepath.Join(published.Root, name), []byte("changed\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := store.Get(ctx, snapshot.DigestAlgorithm, snapshot.TreeDigest); !errors.Is(err, ErrMismatch) {
				t.Fatalf("changed %s accepted under original tree pin: %v", name, err)
			}
			if _, err := store.Put(ctx, snapshot); err != nil {
				t.Fatalf("restore original artifact: %v", err)
			}
			if _, err := store.Get(ctx, snapshot.DigestAlgorithm, snapshot.TreeDigest); err != nil {
				t.Fatal(err)
			}
		})
	}
}
