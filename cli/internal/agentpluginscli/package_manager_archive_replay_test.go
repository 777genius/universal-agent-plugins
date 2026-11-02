//go:build uappm_e2e

package agentpluginscli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func pmArchiveReplay(t *testing.T) {
	graph := pmArchiveGraph{"a": {"c"}, "b": {"c"}, "c": nil}
	origin, original := pmArchiveInstallGraph(t, graph, "a", "b")
	mp, lp := original.app.dependencyPaths()
	agreement := freshRestoreFiles(t, mp, lp)
	active := original.activeFiles(t)
	requests := origin.counts()
	original.run(t, "install", "--locked", "--yes", "--format", "json")
	pmArchiveAssertEqual(t, "warm locked source requests", requests, origin.counts())
	pmArchiveAssertEqual(t, "warm locked bytes/mtime", active, original.activeFiles(t))
	pmArchiveAssertEqual(t, "warm agreement", agreement, freshRestoreFiles(t, mp, lp))

	// Copy only the two portable files into a different home, state and store.
	fresh := newPMArchiveEnvironment(t, origin, false)
	pmArchiveAgreement(t, original, fresh)
	if _, err := os.Lstat(fresh.app.EnvironmentStoreRoot); !os.IsNotExist(err) {
		t.Fatalf("cold store exists: %v", err)
	}
	fresh.run(t, "install", "--locked", "--yes", "--format", "json")
	fresh.assertGraph(t, origin, graph, pmArchiveAll(graph, "gemini"))
	fresh.assertInstalled(t, pmArchiveAll(graph, "gemini"))
	for _, name := range []string{"a", "b", "c"} {
		key := "GET /" + name + ".zip"
		delta := origin.counts()[key] - requests[key]
		t.Logf("single cold locked replay %s: %d requests", key, delta)
		if delta != 1 {
			t.Fatalf("single cold replay fetched locked %s %d times; want exactly once", name, delta)
		}
	}
	for request := range origin.counts() {
		if request != "GET /a.zip" && request != "GET /b.zip" && request != "GET /c.zip" {
			t.Fatalf("unexpected replay request %s", request)
		}
	}
	freshMP, freshLP := fresh.app.dependencyPaths()
	if dependencyRead(t, mp) != dependencyRead(t, freshMP) || dependencyRead(t, lp) != dependencyRead(t, freshLP) {
		t.Fatal("cold restore rewrote portable agreement")
	}
	pmArchiveAssertEqual(t, "original active environment", active, original.activeFiles(t))
	requests = origin.counts()
	fresh.run(t, "install", "--locked", "--yes", "--format", "json")
	pmArchiveAssertEqual(t, "second cold-root replay requests", requests, origin.counts())
}

func pmArchiveRefusals(t *testing.T) {
	for _, mode := range []string{"changed-archive", "missing-leaf", "corrupt-store"} {
		t.Run(mode, func(t *testing.T) {
			graph := pmArchiveGraph{"a": {"c"}, "c": nil}
			origin, original := pmArchiveInstallGraph(t, graph, "a")
			fixture := newPMArchiveEnvironment(t, origin, false)
			pmArchiveAgreement(t, original, fixture)
			wantError := "checksum"
			if mode == "corrupt-store" {
				fixture = original
				pin, _ := fixture.lock(t).Package("c")
				stored, err := fixture.app.dependencyStore().Get(t.Context(), pin.DigestAlgorithm, pin.TreeDigest)
				if err != nil {
					t.Fatal(err)
				}
				dependencyWrite(t, filepath.Join(stored.Root, "marker.txt"), "tampered cache bytes\n")
				wantError = "store"
			} else {
				origin.mu.Lock()
				if mode == "changed-archive" {
					origin.payloads["/c.zip"] = append(append([]byte{}, origin.payloads["/c.zip"]...), []byte("changed original archive bytes")...)
				} else {
					delete(origin.payloads, "/c.zip")
					wantError = "404"
				}
				origin.mu.Unlock()
			}
			mp, lp := fixture.app.dependencyPaths()
			agreement, active := freshRestoreFiles(t, mp, lp), fixture.activeFiles(t)
			_, err := fixture.executePM(t.Context(), t, "", false, "install", "--locked", "--yes", "--format", "json")
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), wantError) {
				t.Fatalf("%s did not fail at intended boundary: %v", mode, err)
			}
			pmArchiveAssertEqual(t, "failed replay agreement", agreement, freshRestoreFiles(t, mp, lp))
			pmArchiveAssertEqual(t, "failed replay active files", active, fixture.activeFiles(t))
		})
	}
	t.Run("incompatible-parent-checksums", pmArchiveChecksumConflict)
}

func pmArchiveChecksumConflict(t *testing.T) {
	origin := newPMArchiveOrigin(t, pmArchiveGraph{"a": {"c"}, "b": {"c"}, "c": nil})
	checksum := sha256.Sum256(origin.payloads["/c.zip"])
	correct, different := hex.EncodeToString(checksum[:]), strings.Repeat("0", 64)
	// Keep the origin/source ordering fixed and swap which parent's checksum
	// matches C. Ignoring either demand would let one of these permutations pass.
	discoveredBoth := false
	for _, validParent := range []string{"a", "b"} {
		t.Run(validParent+"-valid", func(t *testing.T) {
			for _, name := range []string{"a", "b"} {
				digest := different
				if name == validParent {
					digest = correct
				}
				origin.sidecar(t, name, fmt.Sprintf("schema_version = 1\n[dependencies.c]\narchive = %q\nsha256 = %q\n", origin.url("c"), digest))
			}
			fixture := newPMArchiveEnvironment(t, origin, false)
			fixture.manifest(t, origin, []string{"a", "b"}, map[string][]string{"gemini": {"a", "b"}})
			mp, lp := fixture.app.dependencyPaths()
			before, active, requests := freshRestoreFiles(t, mp, lp), fixture.activeFiles(t), origin.counts()
			_, err := fixture.executePM(t.Context(), t, "", false, "install", "--yes", "--format", "json")
			if err == nil || (!strings.Contains(err.Error(), "dependency conflict") && !strings.Contains(err.Error(), "archive original-byte SHA256")) {
				t.Fatalf("two-parent incompatible checksums did not fail at graph/integrity boundary: %v", err)
			}
			t.Logf("incompatible parent archive checksums refused: %v", err)
			after := origin.counts()
			// A bad first demand may abort acquisition before the other parent is
			// visited. Its swapped case must reach both declarations and refuse.
			discoveredBoth = discoveredBoth || (after["GET /a.zip"] > requests["GET /a.zip"] && after["GET /b.zip"] > requests["GET /b.zip"])
			pmArchiveAssertEqual(t, "checksum conflict agreement", before, freshRestoreFiles(t, mp, lp))
			pmArchiveAssertEqual(t, "checksum conflict active files", active, fixture.activeFiles(t))
		})
	}
	if !discoveredBoth {
		t.Fatal("neither checksum permutation reached both parent declarations")
	}
}

func pmArchiveLockRefusals(t *testing.T) {
	graph := pmArchiveGraph{"a": {"c"}, "c": nil}
	origin, original := pmArchiveInstallGraph(t, graph, "a")
	_, originalLP := original.app.dependencyPaths()
	lockBytes := dependencyRead(t, originalLP)
	for _, corruption := range []string{"missing", "malformed", "missing-node", "forged-edge", "stale"} {
		for _, locked := range []bool{false, true} {
			if corruption == "missing" && !locked {
				continue
			} // ordinary install may create its first lock
			name := corruption + "/ordinary"
			if locked {
				name = corruption + "/locked"
			}
			t.Run(name, func(t *testing.T) {
				fixture := newPMArchiveEnvironment(t, origin, false)
				pmArchiveAgreement(t, original, fixture)
				mp, lp := fixture.app.dependencyPaths()
				var broken environment.Lock
				if err := json.Unmarshal([]byte(lockBytes), &broken); err != nil {
					t.Fatal(err)
				}
				switch corruption {
				case "missing":
					if err := os.Remove(lp); err != nil {
						t.Fatal(err)
					}
				case "malformed":
					dependencyWrite(t, lp, "{ broken JSON\n")
				case "missing-node":
					broken.Packages = broken.Packages[:1]
					pmArchiveWriteJSON(t, lp, broken)
				case "forged-edge":
					broken.Packages[1].RequiredBy[0].Alias = "ghost"
					pmArchiveWriteJSON(t, lp, broken)
				case "stale":
					dependencyWrite(t, mp, strings.Replace(dependencyRead(t, mp), "/a.zip", "/elsewhere.zip", 1))
				}
				before, active := freshRestoreFiles(t, mp, lp), fixture.activeFiles(t)
				args := []string{"install", "--yes", "--format", "json"}
				if locked {
					args = append(args, "--locked")
				}
				_, err := fixture.executePM(t.Context(), t, "", false, args...)
				if err == nil {
					t.Fatalf("accepted %s lock", corruption)
				}
				pmArchiveAssertEqual(t, "invalid agreement bytes/absence", before, freshRestoreFiles(t, mp, lp))
				pmArchiveAssertEqual(t, "invalid agreement active files", active, fixture.activeFiles(t))
			})
		}
	}
}

func pmArchivePreviews(t *testing.T) {
	for _, mode := range []string{"dry-run", "decline", "cancel-download"} {
		t.Run(mode, func(t *testing.T) {
			origin := newPMArchiveOrigin(t, pmArchiveGraph{"a": {"c"}, "c": nil})
			fixture := newPMArchiveEnvironment(t, origin, false)
			fixture.manifest(t, origin, []string{"a"}, map[string][]string{"gemini": {"a"}})
			mp, lp := fixture.app.dependencyPaths()
			before, active := freshRestoreFiles(t, mp, lp), fixture.activeFiles(t)
			if mode == "cancel-download" {
				origin.mu.Lock()
				origin.blocked, origin.started = "/a.zip", make(chan struct{}, 1)
				started := origin.started
				origin.mu.Unlock()
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				done := make(chan error, 1)
				go func() {
					_, err := fixture.executePM(ctx, t, "", false, "install", "--yes", "--format", "json")
					done <- err
				}()
				select {
				case <-started:
					cancel()
				case err := <-done:
					t.Fatalf("command ended before cancellable source request: %v", err)
				case <-time.After(30 * time.Second):
					t.Fatal("source request did not begin before hang guard")
				}
				select {
				case err := <-done:
					if err == nil {
						t.Fatal("canceled download reported success")
					}
				case <-time.After(30 * time.Second):
					t.Fatal("source cancellation exceeded hang guard")
				}
			} else {
				args, input, terminal := []string{"install"}, "n\n", true
				if mode == "dry-run" {
					args, input, terminal = []string{"install", "--dry-run", "--yes", "--format", "json"}, "", false
				}
				if _, err := fixture.executePM(t.Context(), t, input, terminal, args...); err != nil {
					t.Fatalf("%s: %v", mode, err)
				}
			}
			pmArchiveAssertEqual(t, "preview agreement bytes/absence", before, freshRestoreFiles(t, mp, lp))
			pmArchiveAssertEqual(t, "preview active files", active, fixture.activeFiles(t))
			pmArchiveAssertNoActive(t, fixture)
		})
	}
}
