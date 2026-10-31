//go:build uappm_e2e

package agentpluginscli

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/environmentstore"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func packageManagerAdversarialCases() map[string]func(*testing.T) {
	return map[string]func(*testing.T){
		"foreign-native-settings":   pmAdversarialForeignSettings,
		"managed-native-drift":      pmAdversarialNativeDrift,
		"hostile-archives":          pmAdversarialArchives,
		"cache-metadata-corruption": pmAdversarialCacheMetadata,
		"malformed-state":           pmAdversarialMalformedState,
		"component-name-collision":  pmAdversarialComponentCollision,
		"stale-lock-metadata":       pmAdversarialLockMetadata,
		"symlink-lock-refusal":      pmAdversarialSymlinkMutationLock,
		"foreign-skill-refusal":     pmAdversarialForeignSkillCollision,
		"native-config-obstruction": pmAdversarialNativeConfigObstruction,
	}
}

type pmZIPMember struct {
	name, body string
	mode       os.FileMode
}

// A slice deliberately preserves duplicate and colliding member names. Existing
// valid package members are retained so each refusal reaches the archive guard.
func pmRewriteZIP(t *testing.T, origin *pmArchiveOrigin, name string, replace map[string]string, extra ...pmZIPMember) {
	t.Helper()
	origin.mu.Lock()
	defer origin.mu.Unlock()
	body := origin.payloads["/"+name+".zip"]
	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	members := make([]pmZIPMember, 0, len(reader.File)+len(extra))
	for _, member := range reader.File {
		input, err := member.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, readErr := io.ReadAll(input)
		closeErr := input.Close()
		if readErr != nil || closeErr != nil {
			t.Fatalf("read valid ZIP member: %v %v", readErr, closeErr)
		}
		if updated, ok := replace[member.Name]; ok {
			content = []byte(updated)
		}
		members = append(members, pmZIPMember{name: member.Name, body: string(content), mode: member.Mode()})
	}
	members = append(members, extra...)
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, member := range members {
		header := &zip.FileHeader{Name: member.name, Method: zip.Deflate}
		mode := member.mode
		if mode == 0 {
			mode = 0600
		}
		header.SetMode(mode)
		output, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(output, member.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	origin.payloads["/"+name+".zip"] = buffer.Bytes()
}

func pmNativeDocument(t *testing.T, fixture pmArchiveEnvironment) map[string]any {
	t.Helper()
	var document map[string]any
	if err := json.Unmarshal([]byte(dependencyRead(t, filepath.Join(fixture.clients["gemini"].ConfigRoot, "settings.json"))), &document); err != nil {
		t.Fatal(err)
	}
	return document
}

func pmAdversarialForeignSettings(t *testing.T) {
	graph := pmArchiveGraph{"a": nil, "b": nil}
	origin := newPMArchiveOrigin(t, graph)
	fixture := newPMArchiveEnvironment(t, origin, false)
	fixture.manifest(t, origin, []string{"a"}, map[string][]string{"gemini": {"a"}})
	fixture.run(t, "install", "--yes", "--format", "json")
	initial := fixture.assertInstalled(t, pmArchiveTargets{"a": {"gemini"}})
	data := pmEvolutionSeedData(t, fixture)
	dataBefore := freshRestoreFiles(t, data)
	document := pmNativeDocument(t, fixture)
	foreign := map[string]any{"httpUrl": "https://foreign.test/mcp", "headers": map[string]any{"X-User-Edited": "keep exact value"}, "timeout": float64(147)}
	preference := map[string]any{"language": "日本語", "enabled": true, "nested": []any{"space é", float64(7)}}
	document["mcpServers"].(map[string]any)["foreign.test"] = foreign
	document["userPreference"] = preference
	pmArchiveWriteJSON(t, filepath.Join(fixture.clients["gemini"].ConfigRoot, "settings.json"), document)
	for index, args := range [][]string{
		{"add", origin.url("b"), "--alias", "b", "--target", "gemini", "--format", "json"},
		{"remove", "a", "--target", "gemini", "--format", "json"},
		{"remove", "b", "--target", "gemini", "--format", "json"},
	} {
		fixture.run(t, args...)
		after := pmNativeDocument(t, fixture)
		pmArchiveAssertEqual(t, "user-edited foreign MCP value", foreign, after["mcpServers"].(map[string]any)["foreign.test"])
		pmArchiveAssertEqual(t, "user-edited top-level preference", preference, after["userPreference"])
		pmArchiveAssertEqual(t, "foreign settings journey retained unrelated data", dataBefore, freshRestoreFiles(t, data))
		expected := pmArchiveTargets{}
		switch index {
		case 0:
			expected = pmArchiveAll(graph, "gemini")
		case 1:
			expected["b"] = []string{"gemini"}
		}
		fixture.assertGraph(t, origin, graph, expected)
		bindings := fixture.assertInstalled(t, expected)
		if index == 0 {
			pmEvolutionSameBindings(t, initial, bindings, "a/gemini")
		}
	}
	fixture.assertGraph(t, origin, graph, pmArchiveTargets{})
	fixture.assertInstalled(t, pmArchiveTargets{})
	if _, err := os.Lstat(initial["a/gemini"].TargetPath); !os.IsNotExist(err) {
		t.Fatalf("removed A survived while preserving foreign settings: %v", err)
	}
}

func pmAdversarialNativeDrift(t *testing.T) {
	for _, operation := range []string{"update", "remove"} {
		t.Run(operation, func(t *testing.T) {
			graph := pmArchiveGraph{"a": nil}
			origin, fixture := pmArchiveInstallGraph(t, graph, "a")
			data := pmEvolutionSeedData(t, fixture)
			document := pmNativeDocument(t, fixture)
			original := document["mcpServers"].(map[string]any)["a"]
			foreign := map[string]any{"httpUrl": "https://user-edit.test/keep", "headers": map[string]any{"X-User": "preserve"}}
			document["mcpServers"].(map[string]any)["a"] = foreign
			settingsPath := filepath.Join(fixture.clients["gemini"].ConfigRoot, "settings.json")
			pmArchiveWriteJSON(t, settingsPath, document)
			preservedPaths := []string{settingsPath, filepath.Join(fixture.clients["gemini"].ConfigRoot, "skills", "a"), data}
			preserved := freshRestoreFiles(t, preservedPaths...)
			args := []string{"remove", "a", "--target", "gemini", "--format", "json"}
			if operation == "update" {
				origin.sidecar(t, "a", "schema_version = 1\n# independently changed archive payload\n")
				fixture.run(t, "deps", "update", "a", "--format", "json")
				args = []string{"install", "--locked", "--yes", "--format", "json"}
			}
			active, requests := fixture.activeFiles(t), origin.counts()
			_, err := fixture.executePM(t.Context(), t, "", false, args...)
			if err == nil || !strings.Contains(err.Error(), "changed outside agentplugins") {
				t.Fatalf("edited native MCP ownership did not refuse %s: %v", operation, err)
			}
			view, inspectErr := fixture.app.Installer.Inspect(t.Context())
			if inspectErr != nil || view.Recovery.Required {
				t.Fatalf("edited native ownership left unrecoverable uncertainty on %s: %+v %v", operation, view.Recovery, inspectErr)
			}
			pmArchiveAssertEqual(t, "native drift preserved literal native/user data bytes", preserved, freshRestoreFiles(t, preservedPaths...))
			if operation == "remove" {
				// Saved intent may already have removed A. Its known no-effect
				// native refusal must preserve actual ownership and remain retryable.
				// Beginning and acknowledging the durable attempt rewrites the
				// state file. Require its exact bytes/mode, allowing only its mtime.
				after := fixture.activeFiles(t)
				stateBefore, stateAfter := active[fixture.store.Path], after[fixture.store.Path]
				if stateBefore.body != stateAfter.body || stateBefore.mode != stateAfter.mode {
					t.Fatal("known no-effect native refusal changed durable ownership/receipt bytes")
				}
				delete(active, fixture.store.Path)
				delete(after, fixture.store.Path)
				pmArchiveAssertEqual(t, "edited native removal actual payload effects", active, after)
				fixture.assertGraph(t, origin, graph, pmArchiveTargets{})
			} else {
				if len(view.Installations) != 1 || len(view.Installations[0].Bindings) != 1 {
					t.Fatalf("native update lost its owned binding: %+v", view)
				}
				binding := view.Installations[0].Bindings[0]
				pin, _ := fixture.lock(t).Package("a")
				if binding.TreeDigest != pin.TreeDigest || binding.Activation != "failed" || binding.Verification != "failed" {
					t.Fatalf("native update hid committed partial lifecycle: %+v", binding)
				}
				if !readDependencyStatusTestFacts(t, fixture.run(t, "deps", "status", "--format", "json"))["partial"] {
					t.Fatal("native update hid partial status")
				}
			}
			pmArchiveAssertEqual(t, "native ownership refusal acquisition", requests, origin.counts())
			pmArchiveAssertEqual(t, "literal native user edit", foreign, pmNativeDocument(t, fixture)["mcpServers"].(map[string]any)["a"])
			// Resolve only the injected foreign edit. Retry the same intended command.
			document["mcpServers"].(map[string]any)["a"] = original
			pmArchiveWriteJSON(t, settingsPath, document)
			fixture.run(t, args...)
			expected := pmArchiveTargets{}
			if operation == "update" {
				expected["a"] = []string{"gemini"}
			}
			fixture.assertInstalled(t, expected)
		})
	}
}

func pmAdversarialArchives(t *testing.T) {
	for _, hostile := range []struct {
		name, diagnostic string
		members          []pmZIPMember
	}{
		{"traversal", "unsafe path", []pmZIPMember{{name: "../outside-sentinel", body: "must never escape"}}},
		{"duplicate", "duplicate", []pmZIPMember{{name: "publisher/collision.txt", body: "first"}, {name: "publisher/collision.txt", body: "second"}}},
		{"case-collision", "collide", []pmZIPMember{{name: "publisher/Case.txt", body: "first"}, {name: "publisher/case.txt", body: "second"}}},
		{"file-directory", "file/directory conflict", []pmZIPMember{{name: "publisher/conflict", body: "file"}, {name: "publisher/conflict/child", body: "child"}}},
		{"symlink-member", "links and special entries", []pmZIPMember{{name: "publisher/escape-link", body: "../../outside-sentinel", mode: os.ModeSymlink | 0777}}},
	} {
		for _, mode := range []string{"fresh", "update"} {
			t.Run(hostile.name+"/"+mode, func(t *testing.T) {
				graph := pmArchiveGraph{"a": nil}
				origin := newPMArchiveOrigin(t, graph)
				fixture := newPMArchiveEnvironment(t, origin, false)
				fixture.manifest(t, origin, []string{"a"}, map[string][]string{"gemini": {"a"}})
				if mode == "update" {
					fixture.run(t, "install", "--yes", "--format", "json")
					fixture.assertInstalled(t, pmArchiveAll(graph, "gemini"))
				}
				pmEvolutionSeedData(t, fixture)
				sentinel := filepath.Join(fixture.root, "outside-sentinel")
				dependencyWrite(t, sentinel, "outside user file remains untouched\n")
				pmRewriteZIP(t, origin, "a", nil, hostile.members...)
				before, requests := pmEvolutionFiles(t, fixture), origin.counts()
				args := []string{"install", "--yes", "--format", "json"}
				if mode == "update" {
					args = []string{"deps", "update", "a", "--format", "json"}
				}
				_, err := fixture.executePM(t.Context(), t, "", false, args...)
				if err == nil || !strings.Contains(err.Error(), hostile.diagnostic) {
					t.Fatalf("otherwise-valid hostile ZIP did not reach %s guard: %v", hostile.name, err)
				}
				if origin.counts()["GET /a.zip"] <= requests["GET /a.zip"] {
					t.Fatal("hostile archive was never requested")
				}
				pmArchiveAssertEqual(t, "hostile archive active effects/agreement", before, pmEvolutionFiles(t, fixture))
				if dependencyRead(t, sentinel) != "outside user file remains untouched\n" {
					t.Fatal("archive refusal changed an unrelated external user sentinel")
				}
				acquirer := fixture.app.SourceAcquirer.(sourceacquisition.Acquirer)
				entries, readErr := os.ReadDir(acquirer.TempRoot)
				if readErr != nil || len(entries) != 0 {
					t.Fatalf("hostile acquisition retained owned temporary files: %v %v", entries, readErr)
				}
				if mode == "fresh" {
					pmArchiveAssertNoActive(t, fixture)
				}
			})
		}
	}
}

func pmAdversarialCacheMetadata(t *testing.T) {
	for _, corruption := range []string{"malformed-metadata", "missing-metadata", "wrong-schema", "wrong-tree-digest", "missing-package-file"} {
		t.Run(corruption, func(t *testing.T) {
			graph := pmArchiveGraph{"a": {"c"}, "c": nil}
			origin, fixture := pmArchiveInstallGraph(t, graph, "a")
			pmEvolutionSeedData(t, fixture)
			pin, _ := fixture.lock(t).Package("c")
			stored, err := fixture.app.dependencyStore().Get(t.Context(), pin.DigestAlgorithm, pin.TreeDigest)
			if err != nil {
				t.Fatal(err)
			}
			metadataPath := filepath.Join(filepath.Dir(stored.Root), "metadata.json")
			metadataBytes := dependencyRead(t, metadataPath)
			switch corruption {
			case "malformed-metadata":
				dependencyWrite(t, metadataPath, "{ malformed immutable metadata\n")
			case "missing-metadata":
				if err := os.Remove(metadataPath); err != nil {
					t.Fatal(err)
				}
			case "wrong-schema", "wrong-tree-digest":
				metadata := stored.Metadata
				if corruption == "wrong-schema" {
					metadata.SchemaVersion = 999
				} else {
					metadata.TreeDigest = "sha256:" + strings.Repeat("0", 64)
				}
				pmArchiveWriteJSON(t, metadataPath, metadata)
			case "missing-package-file":
				if err := os.Remove(filepath.Join(stored.Root, "marker.txt")); err != nil {
					t.Fatal(err)
				}
			}
			before, cache, requests := pmEvolutionFiles(t, fixture), freshRestoreFiles(t, fixture.app.EnvironmentStoreRoot), origin.counts()
			_, err = fixture.executePM(t.Context(), t, "", false, "install", "--locked", "--yes", "--format", "json")
			if !errors.Is(err, environmentstore.ErrMismatch) {
				t.Fatalf("damaged cache metadata did not fail closed as a mismatch: %v", err)
			}
			pmArchiveAssertEqual(t, "corrupt cache active effects/agreement", before, pmEvolutionFiles(t, fixture))
			pmArchiveAssertEqual(t, "corrupt cache not overwritten", cache, freshRestoreFiles(t, fixture.app.EnvironmentStoreRoot))
			pmArchiveAssertEqual(t, "corrupt cache no fallback requests", requests, origin.counts())
			dependencyWrite(t, metadataPath, metadataBytes)
			if corruption == "missing-package-file" {
				dependencyWrite(t, filepath.Join(stored.Root, "marker.txt"), pmArchiveMarker("c"))
			}
			fixture.run(t, "install", "--locked", "--yes", "--format", "json")
			fixture.assertInstalled(t, pmArchiveAll(graph, "gemini"))
		})
	}
}

func pmAdversarialMalformedState(t *testing.T) {
	graph := pmArchiveGraph{"a": {"c"}, "c": nil}
	origin, fixture := pmArchiveInstallGraph(t, graph, "a")
	pmEvolutionSeedData(t, fixture)
	saved := dependencyRead(t, fixture.store.Path)
	corrupt := "{ corrupted active installation state\n"
	dependencyWrite(t, fixture.store.Path, corrupt)
	before, requests := pmEvolutionFiles(t, fixture), origin.counts()
	for _, args := range [][]string{
		{"install", "--locked", "--yes", "--format", "json"},
		{"remove", "a", "--target", "gemini", "--format", "json"},
		{"deps", "recover", "--yes", "--format", "json"},
	} {
		_, err := fixture.executePM(t.Context(), t, "", false, args...)
		if !errors.Is(err, statev2.ErrStateCorrupt) {
			t.Fatalf("malformed active state was not refused at state decoding for %v: %v", args, err)
		}
		pmArchiveAssertEqual(t, "malformed active state preserved", before, pmEvolutionFiles(t, fixture))
		pmArchiveAssertEqual(t, "malformed state no acquisition", requests, origin.counts())
		if dependencyRead(t, fixture.store.Path) != corrupt {
			t.Fatal("malformed state was overwritten")
		}
	}
	dependencyWrite(t, fixture.store.Path, saved)
	fixture.run(t, "install", "--locked", "--yes", "--format", "json")
	fixture.assertInstalled(t, pmArchiveAll(graph, "gemini"))
}

func pmAdversarialComponentCollision(t *testing.T) {
	for _, identical := range []bool{false, true} {
		t.Run(fmt.Sprintf("identical-%t", identical), func(t *testing.T) {
			graph := pmArchiveGraph{"a": nil, "b": nil}
			origin := newPMArchiveOrigin(t, graph)
			fixture := newPMArchiveEnvironment(t, origin, false)
			fixture.manifest(t, origin, []string{"a"}, map[string][]string{"gemini": {"a"}})
			fixture.run(t, "install", "--yes", "--format", "json")
			initial := fixture.assertInstalled(t, pmArchiveTargets{"a": {"gemini"}})
			data := pmEvolutionSeedData(t, fixture)
			url := "https://different.test/b"
			if identical {
				url = "https://example.test/pm/a"
			}
			mcp := fmt.Sprintf(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"a":{"type":"streamable-http","url":%q}}}`, url)
			pmRewriteZIP(t, origin, "b", map[string]string{"publisher/mcp.json": mcp})
			// Native component ownership is checked during activation. A failed
			// activation can already have committed B's materialized binding and
			// desired agreement; the test requires truthful partial state, not a
			// new all-graph rollback promise.
			settingsPath := filepath.Join(fixture.clients["gemini"].ConfigRoot, "settings.json")
			preservedPaths := []string{initial["a/gemini"].TargetPath, filepath.Join(fixture.clients["gemini"].ConfigRoot, "skills", "a"), settingsPath, data}
			preserved, requests := freshRestoreFiles(t, preservedPaths...), origin.counts()
			_, err := fixture.executePM(t.Context(), t, "", false, "add", origin.url("b"), "--alias", "b", "--target", "gemini", "--format", "json")
			if err == nil || (!strings.Contains(strings.ToLower(err.Error()), "mcp") && !strings.Contains(strings.ToLower(err.Error()), "namespace")) {
				t.Fatalf("different-source MCP name collision did not refuse: %v", err)
			}
			if origin.counts()["GET /b.zip"] <= requests["GET /b.zip"] {
				t.Fatal("collision test never acquired B")
			}
			pmArchiveAssertEqual(t, "component collision preserved A managed/native bytes", preserved, freshRestoreFiles(t, preservedPaths...))
			fixture.assertGraph(t, origin, graph, pmArchiveAll(graph, "gemini"))
			checkPartial := func() map[string]installer.InspectedBinding {
				t.Helper()
				view, err := fixture.app.Installer.Inspect(t.Context())
				if err != nil || view.Recovery.Required || len(view.EnvironmentReceipts) != 2 {
					t.Fatalf("collision did not retain two committed owned bindings: %+v %v", view, err)
				}
				bindings := map[string]installer.InspectedBinding{}
				for _, installation := range view.Installations {
					for _, binding := range installation.Bindings {
						if binding.Owner == nil {
							t.Fatalf("partial collision lost owner: %+v", binding)
						}
						bindings[binding.Owner.Alias+"/"+binding.ClientID] = binding
					}
				}
				pmEvolutionSameBindings(t, initial, bindings, "a/gemini")
				b := bindings["b/gemini"]
				pin, _ := fixture.lock(t).Package("b")
				if len(bindings) != 2 || b.BindingID == "" || b.Materialization != "materialized" || b.Activation != "failed" || b.Verification != "failed" || b.Owner.SourceKey != pin.SourceKey || b.TreeDigest != pin.TreeDigest {
					t.Fatalf("collision hid B's committed failed lifecycle: %+v", b)
				}
				if dependencyRead(t, filepath.Join(b.TargetPath, "marker.txt")) != pmArchiveMarker("b") {
					t.Fatal("partial collision lost real B payload")
				}
				facts := readDependencyStatusTestFacts(t, fixture.run(t, "deps", "status", "--format", "json"))
				if !facts["partial"] {
					t.Fatal("collision status omitted partial lifecycle fact")
				}
				return bindings
			}
			partial := checkPartial()
			mp, lp := fixture.app.dependencyPaths()
			agreement := freshRestoreFiles(t, mp, lp)
			requests = origin.counts()
			_, err = fixture.executePM(t.Context(), t, "", false, "deps", "sync", "--locked", "--yes", "--format", "json")
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), "mcp") {
				t.Fatalf("same collision retry unexpectedly succeeded: %v", err)
			}
			pmArchiveAssertEqual(t, "collision retry preserved A", preserved, freshRestoreFiles(t, preservedPaths...))
			pmArchiveAssertEqual(t, "collision retry desired agreement", agreement, freshRestoreFiles(t, mp, lp))
			pmArchiveAssertEqual(t, "collision retry exact cached pins", requests, origin.counts())
			pmEvolutionSameBindings(t, partial, checkPartial(), "a/gemini", "b/gemini")
			fixture.run(t, "remove", "b", "--target", "gemini", "--format", "json")
			remaining := fixture.assertInstalled(t, pmArchiveTargets{"a": {"gemini"}})
			pmEvolutionSameBindings(t, initial, remaining, "a/gemini")
			pmArchiveAssertEqual(t, "removing failed B preserved A", preserved, freshRestoreFiles(t, preservedPaths...))
		})
	}
}

func pmAdversarialLockMetadata(t *testing.T) {
	for _, obstruction := range []string{"stale-metadata", "directory-leaf"} {
		t.Run(obstruction, func(t *testing.T) {
			graph := pmArchiveGraph{"a": nil}
			origin := newPMArchiveOrigin(t, graph)
			fixture := newPMArchiveEnvironment(t, origin, false)
			fixture.manifest(t, origin, []string{"a"}, map[string][]string{"gemini": {"a"}})
			if obstruction == "stale-metadata" {
				dependencyWrite(t, fixture.app.EnvironmentLockPath, `{"pid":-1,"acquired_at":"1900-01-01T00:00:00Z","stale":true}`)
			} else {
				if err := os.Mkdir(fixture.app.EnvironmentLockPath, 0700); err != nil {
					t.Fatal(err)
				}
				before, requests := pmEvolutionFiles(t, fixture), origin.counts()
				_, err := fixture.executePM(t.Context(), t, "", false, "deps", "lock", "--format", "json")
				if err == nil || !strings.Contains(err.Error(), "mutation lock") {
					t.Fatalf("blocked lock leaf not refused: %v", err)
				}
				pmArchiveAssertEqual(t, "lock-path refusal active effects/agreement", before, pmEvolutionFiles(t, fixture))
				pmArchiveAssertEqual(t, "lock-path refusal source requests", requests, origin.counts())
				if err := os.Remove(fixture.app.EnvironmentLockPath); err != nil {
					t.Fatal(err)
				}
			}
			fixture.run(t, "deps", "lock", "--format", "json")
			fixture.assertGraph(t, origin, graph, pmArchiveAll(graph, "gemini"))
			var metadata struct {
				PID        int    `json:"pid"`
				AcquiredAt string `json:"acquired_at"`
			}
			if err := json.Unmarshal([]byte(dependencyRead(t, fixture.app.EnvironmentLockPath)), &metadata); err != nil || metadata.PID != os.Getpid() || metadata.AcquiredAt == "" || strings.HasPrefix(metadata.AcquiredAt, "1900-") {
				t.Fatalf("successful lock did not replace advisory metadata: %+v %v", metadata, err)
			}
			fixture.run(t, "install", "--locked", "--yes", "--format", "json")
			fixture.assertInstalled(t, pmArchiveAll(graph, "gemini"))
		})
	}
}

func pmAdversarialSymlinkMutationLock(t *testing.T) {
	graph := pmArchiveGraph{"a": nil}
	origin := newPMArchiveOrigin(t, graph)
	fixture := newPMArchiveEnvironment(t, origin, false)
	fixture.manifest(t, origin, []string{"a"}, map[string][]string{"gemini": {"a"}})
	sentinel := filepath.Join(fixture.root, "outside-user-lock-sentinel")
	value := "foreign user file must never be edited by the mutation lock\n"
	dependencyWrite(t, sentinel, value)
	lockPath := fixture.app.EnvironmentLockPath
	if err := os.Symlink(sentinel, lockPath); err != nil {
		if runtime.GOOS != "windows" {
			t.Fatalf("Unix E2E requires a real symlink guard: %v", err)
		}
		// Windows runner accounts may not have Developer Mode symlink privilege.
		// The same path must still refuse an occupied unsafe lock leaf.
		if err := os.Mkdir(lockPath, 0700); err != nil {
			t.Fatalf("create portable unsafe lock leaf: %v", err)
		}
	}
	before, requests := pmEvolutionFiles(t, fixture), origin.counts()
	_, err := fixture.executePM(t.Context(), t, "", false, "deps", "lock", "--format", "json")
	if err == nil || !strings.Contains(err.Error(), "mutation lock") {
		t.Fatalf("unsafe mutation lock path was not refused: %v", err)
	}
	pmArchiveAssertEqual(t, "unsafe lock refused before any owned effects", before, pmEvolutionFiles(t, fixture))
	pmArchiveAssertEqual(t, "unsafe lock made no network requests", requests, origin.counts())
	if dependencyRead(t, sentinel) != value {
		t.Fatal("symlink/directory lock refusal changed foreign user bytes")
	}
	if err := os.Remove(lockPath); err != nil {
		t.Fatal(err)
	}
	fixture.run(t, "deps", "lock", "--format", "json")
	fixture.assertGraph(t, origin, graph, pmArchiveAll(graph, "gemini"))
	fixture.run(t, "install", "--locked", "--yes", "--format", "json")
	fixture.assertInstalled(t, pmArchiveAll(graph, "gemini"))
	if dependencyRead(t, sentinel) != value {
		t.Fatal("successful retry changed external user sentinel")
	}
}

func pmAdversarialForeignSkillCollision(t *testing.T) {
	graph := pmArchiveGraph{"a": nil}
	origin := newPMArchiveOrigin(t, graph)
	fixture := newPMArchiveEnvironment(t, origin, false)
	fixture.manifest(t, origin, []string{"a"}, map[string][]string{"gemini": {"a"}})
	foreignRoot := filepath.Join(fixture.clients["gemini"].ConfigRoot, "skills", "a")
	if err := os.MkdirAll(foreignRoot, 0700); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(foreignRoot, "SKILL.md")
	value := "pre-existing native skill owned by a human; never clobber\n"
	dependencyWrite(t, foreign, value)
	settings := filepath.Join(fixture.clients["gemini"].ConfigRoot, "settings.json")
	preserved := freshRestoreFiles(t, foreign, settings)
	_, err := fixture.executePM(t.Context(), t, "", false, "install", "--yes", "--format", "json")
	if err == nil {
		t.Fatal("installer overwrote a pre-existing foreign native skill")
	}
	pmArchiveAssertEqual(t, "native skill collision retained human files", preserved, freshRestoreFiles(t, foreign, settings))
	if err := os.RemoveAll(foreignRoot); err != nil {
		t.Fatal(err)
	}
	fixture.run(t, "install", "--yes", "--format", "json")
	fixture.assertGraph(t, origin, graph, pmArchiveAll(graph, "gemini"))
	fixture.assertInstalled(t, pmArchiveAll(graph, "gemini"))
}

func pmAdversarialNativeConfigObstruction(t *testing.T) {
	graph := pmArchiveGraph{"a": nil}
	origin := newPMArchiveOrigin(t, graph)
	fixture := newPMArchiveEnvironment(t, origin, false)
	fixture.manifest(t, origin, []string{"a"}, map[string][]string{"gemini": {"a"}})
	settings := filepath.Join(fixture.clients["gemini"].ConfigRoot, "settings.json")
	original := dependencyRead(t, settings)
	if err := os.Remove(settings); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(settings, 0700); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(settings, "human.txt")
	value := "occupied config leaf must survive a failed install\n"
	dependencyWrite(t, foreign, value)
	_, err := fixture.executePM(t.Context(), t, "", false, "install", "--yes", "--format", "json")
	if err == nil {
		t.Fatal("installer accepted native settings.json as an occupied directory")
	}
	if dependencyRead(t, foreign) != value {
		t.Fatal("native configuration refusal removed or edited foreign bytes")
	}
	if err := os.RemoveAll(settings); err != nil {
		t.Fatal(err)
	}
	dependencyWrite(t, settings, original)
	fixture.run(t, "install", "--yes", "--format", "json")
	fixture.assertGraph(t, origin, graph, pmArchiveAll(graph, "gemini"))
	fixture.assertInstalled(t, pmArchiveAll(graph, "gemini"))
}
