package domain_test

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func historicalPhysicalID(name, id string) string {
	sum := sha256.Sum256([]byte(id))
	name = strings.TrimSpace(name)
	if len(name) > 51 {
		name = strings.TrimRight(name[:51], ".-")
	}
	return name + "-" + hex.EncodeToString(sum[:6])
}

func TestPhysicalIdentityHistoricalSafeOutputs(t *testing.T) {
	names := []string{"demo", "con", "ordinary.plugin", strings.Repeat("a", 64), strings.Repeat("a", 50) + ".tail", " trimmed ", "legacy_ID"}
	for _, name := range names {
		for _, id := range []string{"00000000-0000-4000-8000-000000000001", "other", "installation\x00cursor"} {
			want := historicalPhysicalID(name, id)
			if err := pathpolicy.ValidateLeafID(want); err != nil {
				t.Fatal(err)
			}
			if got := domain.ComputePhysicalArtifactID(name, id); got != want {
				t.Fatalf("%q: %q != %q", name, got, want)
			}
		}
	}
}

func TestPortableReservedIdentityFilesystemAndPolicyParity(t *testing.T) {
	names := []string{"con", "con.foo", "aux.tools", "nul.x", "prn.plugin", "clock.plugin", "com0.plugin", "lpt0.plugin", "com10.plugin", "lpt10.plugin", "con." + strings.Repeat("x", 60)}
	for n := '1'; n <= '9'; n++ {
		names = append(names, "com"+string(n)+".plugin", "lpt"+string(n)+".plugin")
	}
	root := t.TempDir()
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			got := domain.ComputePhysicalArtifactID(name, "installation")
			old := historicalPhysicalID(name, "installation")
			if err := pathpolicy.ValidateLeafID(got); err != nil || len(got) > 64 {
				t.Fatalf("unsafe %q: %v", got, err)
			}
			if pathpolicy.ValidateLeafID(old) == nil && got != old {
				t.Fatalf("safe output changed %q -> %q", old, got)
			}
			if pathpolicy.ValidateLeafID(old) != nil && !strings.HasPrefix(got, "p-") {
				t.Fatalf("missing safe prefix: %s", got)
			}
			// On Windows CI this creates real directories, rather than cross-compiling.
			if err := os.Mkdir(filepath.Join(root, got), 0700); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPhysicalIdentityDoesNotSanitizeNonstandardLegacyNames(t *testing.T) {
	for _, name := range []string{"CON.foo", "con..foo", "con.bad--name", " con.foo ", "con.bad_name", "con." + strings.Repeat("x", 61)} {
		if got, want := domain.ComputePhysicalArtifactID(name, "id"), historicalPhysicalID(name, "id"); got != want {
			t.Fatalf("silently sanitized %q: %q", name, got)
		}
	}
}
