package nativeconfig

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func transitionFixture(t *testing.T, codec Codec, ext string, names ...string) (TransitionRequest, string) {
	t.Helper()
	root := t.TempDir()
	paths := Paths{JSON: filepath.Join(root, "opencode.json"), JSONC: filepath.Join(root, "opencode.jsonc")}
	path := filepath.Join(root, "opencode."+ext)
	body := `{"unknownRoot": { "keep": [ 1, 2 ] }, "mcpServers":{"owned":{"foreign":true}}}`
	if ext == "jsonc" {
		body = "// foreign root comment\n" + body
	}
	mustWrite(t, path, body)
	requests := make([]Request, 0, len(names))
	for _, name := range names {
		requests = append(requests, Request{Paths: paths, Codec: codec, Action: ActionAdd, Name: name, Server: Server{Type: "remote", URL: "https://mcp.test/" + name}})
	}
	receipts, err := New().ApplyBatch(requests)
	if err != nil {
		t.Fatal(err)
	}
	target := CodecOpenCodeV2
	if codec == target {
		target = CodecOpenCode
	}
	req := TransitionRequest{Paths: paths, SourceCodec: codec, TargetCodec: target, PersistPrepared: func(PreparedTransition) error { return nil }}
	for i, name := range names {
		desired, err := DesiredReceipt(path, target, name, requests[i].Server, Placeholders{})
		if err != nil {
			t.Fatal(err)
		}
		req.Entries = append(req.Entries, TransitionEntry{LogicalID: "opencode-mcp:" + name, Name: name, SourceOwned: receipts[i], TargetServer: requests[i].Server, DesiredReceipt: desired})
	}
	return req, path
}

func assertTransitionOwnership(t *testing.T, req TransitionRequest, receipts []Receipt) {
	t.Helper()
	if len(receipts) != len(req.Entries) {
		t.Fatalf("incomplete receipts: %+v", receipts)
	}
	for i, entry := range req.Entries {
		present, owned, err := New().Inspect(req.Paths, req.TargetCodec, entry.Name, &receipts[i])
		if err != nil || !present || !owned {
			t.Fatalf("target not receipt-owned: %v %v %v", present, owned, err)
		}
		present, owned, err = New().Inspect(req.Paths, req.SourceCodec, entry.Name, &entry.SourceOwned)
		// Legacy Inspect sees the container named servers as a flat member;
		// it cannot verify that container with the old leaf receipt.
		container := req.SourceCodec == CodecOpenCode && entry.Name == "servers"
		if err != nil || owned || present && !container {
			t.Fatalf("source remains: %v %v", present, err)
		}
	}
}

// Regression: two ordinary codec calls could collide on the same logical name,
// lose comments, leave a nested source container, or transfer receipt authority.
func TestDialectTransitionRoundTrip(t *testing.T) {
	for _, ext := range []string{"json", "jsonc"} {
		for _, name := range []string{"owned", "servers"} {
			t.Run(ext+"/"+name, func(t *testing.T) {
				req, path := transitionFixture(t, CodecOpenCode, ext, name)
				if err := os.Chmod(path, 0o640); err != nil {
					t.Fatal(err)
				}
				original := req.Entries[0].SourceOwned
				v2, err := New().ApplyDialectTransition(req)
				if err != nil {
					t.Fatal(err)
				}
				assertTransitionOwnership(t, req, v2)
				entry := readV2Config(t, path)["mcp"].(map[string]any)["servers"].(map[string]any)[name].(map[string]any)
				if entry["disabled"] != false || entry["url"] != "https://mcp.test/"+name || v2[0].Digest == original.Digest {
					t.Fatalf("wrong native V2/digest: %#v %+v", entry, v2)
				}
				req.SourceCodec, req.TargetCodec = req.TargetCodec, req.SourceCodec
				req.Entries[0].SourceOwned = v2[0]
				req.Entries[0].DesiredReceipt = original
				v1, err := New().ApplyDialectTransition(req)
				if err != nil {
					t.Fatal(err)
				}
				assertTransitionOwnership(t, req, v1)
				if v1[0] != original {
					t.Fatalf("legacy receipt semantics changed: %+v != %+v", v1[0], original)
				}
				body := mustRead(t, path)
				for _, fragment := range []string{`"unknownRoot": { "keep": [ 1, 2 ] }`, `"mcpServers":{"owned":{"foreign":true}}`} {
					if !strings.Contains(body, fragment) {
						t.Fatalf("foreign node changed: %s", body)
					}
				}
				if ext == "jsonc" && !strings.Contains(body, "// foreign root comment") {
					t.Fatal("foreign comment lost")
				}
				info, err := os.Stat(path)
				if err != nil || info.Mode().Perm() != 0o640 {
					t.Fatalf("preimage mode not preserved: %v %v", info, err)
				}
			})
		}
	}
}

// Regression: validating only the source dialect would leave foreign keys
// ignored/rejected by the destination. Disabled wrong-dialect leaves also fail.
func TestDialectTransitionRetainedWrongDialectFailsClosed(t *testing.T) {
	for _, codec := range []Codec{CodecOpenCode, CodecOpenCodeV2} {
		req, path := transitionFixture(t, codec, "json", "alpha", "beta")
		foreign := Request{Paths: req.Paths, Codec: codec, Action: ActionAdd, Name: "foreign", Server: Server{Type: "remote", URL: "https://foreign.test"}}
		if codec == CodecOpenCodeV2 {
			foreign.Server.OpenCodeV2 = &OpenCodeV2Options{Disabled: true}
		}
		if _, err := New().Apply(foreign); err != nil {
			t.Fatal(err)
		}
		before := mustRead(t, path)
		req.PersistPrepared = func(PreparedTransition) error { t.Fatal("unqualified root prepared"); return nil }
		if receipts, err := New().ApplyDialectTransition(req); !errors.Is(err, ErrNativeMigrationRequired) || len(receipts) != 0 {
			t.Fatalf("wrong-dialect foreign root accepted: %+v %v", receipts, err)
		}
		assertBytes(t, path, before)
		for _, entry := range req.Entries {
			if present, owned, err := New().Inspect(req.Paths, codec, entry.Name, &entry.SourceOwned); err != nil || !present || !owned {
				t.Fatalf("source authority lost: %v %v %v", present, owned, err)
			}
		}
	}
}

// Regression: equality or a partial projection comparison could adopt a
// foreign destination or overwrite an edited source/owned target.
func TestDialectTransitionExactSourceAndDestination(t *testing.T) {
	for _, scenario := range []string{"edited source", "equal foreign", "owned target", "stale target", "incompatible target", "missing target"} {
		t.Run(scenario, func(t *testing.T) {
			req, path := transitionFixture(t, CodecOpenCode, "jsonc", "owned")
			entry := &req.Entries[0]
			target, err := DesiredReceipt(path, req.TargetCodec, entry.Name, entry.TargetServer, entry.TargetPlaceholders)
			if err != nil {
				t.Fatal(err)
			}
			body := mustRead(t, path)
			if scenario == "edited source" {
				body = strings.Replace(body, `"type":"remote"`, `"userAdded":true,"type":"remote"`, 1)
			} else if scenario != "missing target" {
				body = strings.Replace(body, `"mcp":{`, `"mcp":{ // foreign container comment`+"\n"+`"servers":{"owned":{"type":"remote","url":"https://mcp.test/owned","disabled":false}},`, 1)
			}
			if scenario != "edited source" && scenario != "equal foreign" {
				entry.TargetOwned = &target
			}
			if scenario == "stale target" {
				body = strings.Replace(body, `"disabled":false`, `"disabled":false,"userAdded":true`, 1)
			}
			if scenario == "incompatible target" {
				entry.TargetServer.URL = "https://mcp.test/new"
				entry.DesiredReceipt, err = DesiredReceipt(path, req.TargetCodec, entry.Name, entry.TargetServer, entry.TargetPlaceholders)
				if err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "owned target" {
				body = strings.Replace(body, `"disabled":false`, `/* retained inside compatible target */ "disabled":false`, 1)
			}
			mustWrite(t, path, body)
			if scenario == "owned target" {
				req.PersistPrepared = func(p PreparedTransition) error {
					if p.Entries[0].PreviouslyOwnedTarget == nil || *p.Entries[0].PreviouslyOwnedTarget != target {
						t.Fatal("missing separately owned target authority")
					}
					*p.Entries[0].PreviouslyOwnedTarget = Receipt{}
					*entry.TargetOwned = Receipt{}
					return nil
				}
			}
			wantTarget := target
			receipts, err := New().ApplyDialectTransition(req)
			if scenario == "owned target" {
				if err != nil || len(receipts) != 1 || receipts[0] != wantTarget {
					t.Fatalf("compatible target rejected: %+v %v", receipts, err)
				}
				assertTransitionOwnership(t, req, receipts)
				if !strings.Contains(mustRead(t, path), "// foreign container comment") {
					t.Fatal("foreign container comment lost")
				}
				if !strings.Contains(mustRead(t, path), "/* retained inside compatible target */") {
					t.Fatal("compatible target rebuilt and lost its comment")
				}
			} else {
				want := ErrNotOwned
				if scenario == "equal foreign" || scenario == "incompatible target" {
					want = ErrCollision
				}
				if !errors.Is(err, want) || len(receipts) != 0 {
					t.Fatalf("%s accepted: %+v %v", scenario, receipts, err)
				}
				assertBytes(t, path, body)
			}
		})
	}
}

// Regression: source names could be checked against themselves, a foreign
// destination name could be missed, or disabled V2 could use V1 enabled logic.
// Mixed roots are transient input only: the complete proposed target is clean.
func TestDialectTransitionTargetNamespaceAfterRemoval(t *testing.T) {
	for _, codec := range []Codec{CodecOpenCode, CodecOpenCodeV2} {
		for _, inactive := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%v", codec, inactive), func(t *testing.T) {
				req, path := transitionFixture(t, codec, "jsonc", "api server")
				body := mustRead(t, path)
				foreign := `"api/server": { "type":"remote", "url":"https://foreign.test", "keep":[ 1, 2 ]`
				if req.TargetCodec == CodecOpenCodeV2 {
					foreign += fmt.Sprintf(`, "disabled":%v }`, inactive)
					body = strings.Replace(body, `"mcp":{`, `"mcp":{ "servers":{ // foreign target comment`+"\n"+foreign+`},`, 1)
				} else {
					foreign += fmt.Sprintf(`, "enabled":%v }`, !inactive)
					body = strings.Replace(body, `"mcp":{`, `"mcp":{ // foreign target comment`+"\n"+foreign+`,`, 1)
				}
				mustWrite(t, path, body)
				receipts, err := New().ApplyDialectTransition(req)
				if inactive {
					if err != nil {
						t.Fatal(err)
					}
					assertTransitionOwnership(t, req, receipts)
					if after := mustRead(t, path); !strings.Contains(after, foreign) || !strings.Contains(after, "// foreign target comment") {
						t.Fatalf("compatible foreign nodes/comments changed: %s", after)
					}
				} else {
					var conflict *OpenCodeNamespaceConflict
					if !errors.As(err, &conflict) || len(receipts) != 0 {
						t.Fatalf("target namespace missed: %+v %v", receipts, err)
					}
					assertBytes(t, path, body)
				}
			})
		}
	}
	// Two proposed names also collide, even without any retained foreign leaf.
	req, path := transitionFixture(t, CodecOpenCode, "json", "alpha", "beta")
	before := mustRead(t, path)
	// Change beta's name and native source in the fixture to alpha_tool; seeding
	// those names in one ordinary batch would already reject their namespace.
	body := strings.Replace(before, `"beta":`, `"alpha_tool":`, 1)
	mustWrite(t, path, body)
	source, err := DesiredReceipt(path, CodecOpenCode, "alpha_tool", req.Entries[1].TargetServer, Placeholders{})
	if err != nil {
		t.Fatal(err)
	}
	req.Entries[1].Name, req.Entries[1].LogicalID, req.Entries[1].SourceOwned = "alpha_tool", "opencode-mcp:alpha_tool", source
	req.Entries[1].DesiredReceipt, err = DesiredReceipt(path, req.TargetCodec, "alpha_tool", req.Entries[1].TargetServer, Placeholders{})
	if err != nil {
		t.Fatal(err)
	}
	var conflict *OpenCodeNamespaceConflict
	if receipts, err := New().ApplyDialectTransition(req); !errors.As(err, &conflict) || len(receipts) != 0 {
		t.Fatalf("proposed namespace missed: %+v %v", receipts, err)
	}
	assertBytes(t, path, body)
}

// Regression: unsorted/duplicate entries or codec/ID reinterpretation could
// persist an ambiguous journal. Invalid inputs must not call the durable port.
func TestDialectTransitionClosedRequest(t *testing.T) {
	for _, scenario := range []string{"same codec", "other source", "other target", "duplicate", "unsorted", "wrong ID", "no callback", "empty", "wrong desired", "missing desired", "wrong receipt codec", "missing source", "same paths"} {
		t.Run(scenario, func(t *testing.T) {
			req, path := transitionFixture(t, CodecOpenCode, "json", "alpha", "beta")
			req.PersistPrepared = func(PreparedTransition) error { t.Fatal("invalid request prepared"); return nil }
			switch scenario {
			case "same codec":
				req.TargetCodec = req.SourceCodec
			case "other source":
				req.SourceCodec = CodecGemini
			case "other target":
				req.TargetCodec = CodecMCPServers
			case "duplicate":
				req.Entries[1] = req.Entries[0]
			case "unsorted":
				req.Entries[0], req.Entries[1] = req.Entries[1], req.Entries[0]
			case "wrong ID":
				req.Entries[0].LogicalID = "opencode-v2-mcp:alpha"
			case "no callback":
				req.PersistPrepared = nil
			case "empty":
				req.Entries = nil
			case "wrong desired":
				req.Entries[0].DesiredReceipt = req.Entries[0].SourceOwned
			case "missing desired":
				req.Entries[0].DesiredReceipt = Receipt{}
			case "wrong receipt codec":
				req.Entries[0].SourceOwned.Codec = CodecOpenCodeV2
			case "missing source":
				req.Entries[0].Name, req.Entries[0].LogicalID = "absent", "opencode-mcp:absent"
			case "same paths":
				req.Paths.JSONC = req.Paths.JSON
			}
			before := mustRead(t, path)
			if receipts, err := New().ApplyDialectTransition(req); err == nil || len(receipts) != 0 {
				t.Fatalf("invalid request accepted: %+v %v", receipts, err)
			}
			assertBytes(t, path, before)
		})
	}
}

// Regression: persistence after replacement, aliased byte slices/receipts or
// path drift would make the recovery record untrustworthy. Observe exact real
// bytes in the callback, then deliberately mutate every exposed mutable fact.
func TestDialectTransitionPreparedFactsDetachedAndBeforeWrite(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			req, path := transitionFixture(t, CodecOpenCode, "jsonc", "alpha", "beta")
			before := mustRead(t, path)
			if err := os.Chmod(path, 0o640); err != nil {
				t.Fatal(err)
			}
			want := make([]Receipt, len(req.Entries))
			for i := range req.Entries {
				desired, err := DesiredReceipt(path, req.TargetCodec, req.Entries[i].Name, req.Entries[i].TargetServer, Placeholders{})
				if err != nil {
					t.Fatal(err)
				}
				want[i] = desired
				req.Entries[i].DesiredReceipt = desired
			}
			var targetBytes []byte
			called := 0
			failure := errors.New("journal fsync failed")
			req.PersistPrepared = func(p PreparedTransition) error {
				called++
				assertBytes(t, path, before)
				if p.Path != path || !p.Original.Exists || p.Original.Mode != 0o640 || string(p.Original.Body) != before || p.SourceCodec != req.SourceCodec || p.TargetCodec != req.TargetCodec {
					t.Fatalf("incorrect preimage binding: %+v", p)
				}
				hash := sha256.Sum256(p.TargetBytes)
				if p.TargetHash != fmt.Sprintf("sha256:%x", hash) || len(p.Entries) != 2 {
					t.Fatal("incorrect target binding")
				}
				for i, e := range p.Entries {
					if e.LogicalID != req.Entries[i].LogicalID || e.Name != req.Entries[i].Name || e.SourceReceipt != req.Entries[i].SourceOwned || e.TargetReceipt != want[i] || e.PreviouslyOwnedTarget != nil {
						t.Fatal("incorrect receipt binding")
					}
				}
				targetBytes = bytes.Clone(p.TargetBytes)
				p.TargetBytes[0], p.Original.Body[0] = '!', '!'
				p.Entries[0].TargetReceipt.Digest = "callback mutation"
				p.Entries[0].SourceReceipt.Name = "callback mutation"
				// The request's entry slice/pointers remain caller-owned too.
				req.Entries[0].Name = "caller mutation"
				req.Entries[1].DesiredReceipt = Receipt{}
				req.Entries[1].TargetServer.URL = "https://mutated.test"
				if fail {
					return failure
				}
				return nil
			}
			receipts, err := New().ApplyDialectTransition(req)
			if called != 1 {
				t.Fatalf("callback count: %d", called)
			}
			if fail {
				if !errors.Is(err, failure) || len(receipts) != 0 {
					t.Fatalf("journal failure ignored: %+v %v", receipts, err)
				}
				assertBytes(t, path, before)
			} else {
				if err != nil || !reflect.DeepEqual(receipts, want) {
					t.Fatalf("callback changed receipts: %+v %v", receipts, err)
				}
				assertBytes(t, path, string(targetBytes))
				for _, receipt := range receipts {
					if present, owned, err := New().Inspect(req.Paths, req.TargetCodec, receipt.Name, &receipt); err != nil || !present || !owned {
						t.Fatalf("mutation changed target: %v %v %v", present, owned, err)
					}
				}
			}
		})
	}
}

// Regression: a callback or independent writer could invalidate the preimage;
// equal-to-target foreign bytes must not become kernel rollback authority.
func TestDialectTransitionCallbackAndBoundaryEditsPreserved(t *testing.T) {
	for _, scenario := range []string{"callback foreign", "callback target", "boundary foreign", "alternate"} {
		t.Run(scenario, func(t *testing.T) {
			req, path := transitionFixture(t, CodecOpenCode, "json", "alpha", "beta")
			before := mustRead(t, path)
			foreign := `{"foreign":"late writer"}`
			files := &boundaryMutationIO{}
			switch scenario {
			case "callback foreign", "callback target":
				req.PersistPrepared = func(p PreparedTransition) error {
					if scenario == "callback target" {
						foreign = string(p.TargetBytes)
					}
					return os.WriteFile(path, []byte(foreign), 0o640)
				}
			case "boundary foreign":
				files.beforeCAS = func(path string, _ []byte, _ bool, _ []byte) error { return os.WriteFile(path, []byte(foreign), 0o640) }
			case "alternate":
				req.PersistPrepared = func(PreparedTransition) error { return os.WriteFile(req.Paths.JSONC, []byte(foreign), 0o640) }
			}
			receipts, err := NewWithFileIO(files).ApplyDialectTransition(req)
			want := ErrConcurrentChange
			if scenario == "alternate" {
				want = ErrAmbiguousConfig
			}
			if !errors.Is(err, want) || IsCommittedCleanup(err) || len(receipts) != 0 {
				t.Fatalf("late edit accepted: %+v %v", receipts, err)
			}
			if scenario == "alternate" {
				assertBytes(t, path, before)
				assertBytes(t, req.Paths.JSONC, foreign)
			} else {
				assertBytes(t, path, foreign)
			}
		})
	}
}

// Regression: a loop of leaf transactions could expose a partially switched
// file or remove alpha before discovering beta is stale. Assert the on-disk
// before/after images at the only conditional replacement boundary.
func TestDialectTransitionOneMultiEntryReplacement(t *testing.T) {
	for _, stale := range []bool{false, true} {
		req, path := transitionFixture(t, CodecOpenCode, "json", "alpha", "beta")
		before := mustRead(t, path)
		if stale {
			req.Entries[1].SourceOwned.Digest = "stale"
		}
		files := &boundaryMutationIO{}
		files.beforeCAS = func(path string, expected []byte, exists bool, next []byte) error {
			assertBytes(t, path, before)
			if !exists || string(expected) != before {
				t.Fatal("not original CAS")
			}
			for _, name := range []string{"alpha", "beta"} {
				if !strings.Contains(string(next), `"url":"https://mcp.test/`+name+`"`) {
					t.Fatal("missing target entry")
				}
			}
			return nil
		}
		receipts, err := NewWithFileIO(files).ApplyDialectTransition(req)
		if stale {
			if !errors.Is(err, ErrNotOwned) || files.compareCalls != 0 || len(receipts) != 0 {
				t.Fatalf("partially committed stale batch: %+v %v", receipts, err)
			}
			assertBytes(t, path, before)
		} else {
			if err != nil || files.compareCalls != 1 {
				t.Fatalf("not one replacement: %d %v", files.compareCalls, err)
			}
			assertTransitionOwnership(t, req, receipts)
		}
	}
}

// Regression: post-write faults could publish target receipts prematurely,
// blindly restore a late user edit, or claim unknown bytes were unchanged.
func TestDialectTransitionPostwriteFailureAndRollbackCAS(t *testing.T) {
	for _, scenario := range []string{"restore", "rollback edit", "unknown bytes", "verify edit"} {
		t.Run(scenario, func(t *testing.T) {
			req, path := transitionFixture(t, CodecOpenCode, "json", "alpha", "beta")
			before := mustRead(t, path)
			primary := errors.New("write reported failure after replacement")
			files := &boundaryMutationIO{}
			files.beforeCAS = func(path string, expected []byte, exists bool, next []byte) error {
				if files.compareCalls == 1 {
					if err := (conditionalOSFiles{}).CompareAndSwap(path, expected, exists, next, 0o640); err != nil {
						return err
					}
					if scenario == "unknown bytes" {
						if err := os.WriteFile(path, []byte("unknown"), 0o640); err != nil {
							return err
						}
					}
					return primary
				}
				if scenario == "rollback edit" {
					return os.WriteFile(path, []byte(`{"user":"rollback edit"}`), 0o640)
				}
				return nil
			}
			kernel := NewWithFileIO(files)
			if scenario == "verify edit" {
				req.Paths.JSONC = "" // Existing seam mutates the third read of the selected file.
				kernel = NewWithFileIO(&mutateOnVerifyIO{})
			}
			receipts, err := kernel.ApplyDialectTransition(req)
			if err == nil || IsCommittedCleanup(err) || len(receipts) != 0 {
				t.Fatalf("postwrite fault published success: %+v %v", receipts, err)
			}
			if scenario != "verify edit" && !errors.Is(err, primary) {
				t.Fatalf("lost primary error: %v", err)
			}
			switch scenario {
			case "restore":
				assertBytes(t, path, before)
				if files.compareCalls != 2 {
					t.Fatal("restore not conditional")
				}
				for _, e := range req.Entries {
					if present, owned, err := New().Inspect(req.Paths, req.SourceCodec, e.Name, &e.SourceOwned); err != nil || !present || !owned {
						t.Fatal("rollback invalidated source authority")
					}
				}
			case "rollback edit":
				if !errors.Is(err, ErrConcurrentChange) {
					t.Fatalf("rollback conflict hidden: %v", err)
				}
				assertBytes(t, path, `{"user":"rollback edit"}`)
			case "unknown bytes":
				assertBytes(t, path, "unknown")
			case "verify edit":
				assertBytes(t, path, `{"client":"third-party"}`)
			}
		})
	}
}

// Regression: treating unlock failure as precommit failure could roll back
// skills/source state despite a complete native switch. Receipts remain usable.
func TestDialectTransitionCommittedCleanupReturnsCompleteReceipts(t *testing.T) {
	req, path := transitionFixture(t, CodecOpenCode, "json", "alpha", "beta")
	cleanup := errors.New("unlock failed")
	kernel := NewWithLockAcquirer(func(paths Paths, codec Codec) (func() error, error) {
		release, err := New().acquireCandidateLocks(paths, codec)
		if err != nil {
			return nil, err
		}
		return func() error { return errors.Join(release(), cleanup) }, nil
	})
	receipts, err := kernel.ApplyDialectTransition(req)
	if !IsCommittedCleanup(err) || !errors.Is(err, cleanup) {
		t.Fatalf("not typed committed cleanup: %+v %v", receipts, err)
	}
	assertTransitionOwnership(t, req, receipts)
	if len(readV2Config(t, path)["mcp"].(map[string]any)["servers"].(map[string]any)) != 2 {
		t.Fatal("partial cleanup commit")
	}
}

// Regression: a codec-specific lock or releasing it before journaling would
// allow an ordinary V2 writer into the prepared-to-replacement gap.
func TestDialectTransitionHoldsSharedLockThroughPersistence(t *testing.T) {
	req, path := transitionFixture(t, CodecOpenCode, "json", "alpha")
	prepared, proceed := make(chan struct{}), make(chan struct{})
	defer func() {
		select {
		case <-proceed:
		default:
			close(proceed)
		}
	}()
	req.PersistPrepared = func(PreparedTransition) error { close(prepared); <-proceed; return nil }
	transitionDone := make(chan error, 1)
	go func() { _, err := New().ApplyDialectTransition(req); transitionDone <- err }()
	select {
	case <-prepared:
	case <-time.After(5 * time.Second):
		t.Fatal("transition did not prepare")
	}
	writerDone := make(chan error, 1)
	go func() {
		_, err := New().Apply(Request{Paths: req.Paths, Codec: CodecOpenCodeV2, Action: ActionAdd, Name: "gamma", Server: Server{Type: "remote", URL: "https://foreign.test"}})
		writerDone <- err
	}()
	select {
	case err := <-writerDone:
		t.Fatalf("writer bypassed held lock: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(proceed)
	for _, done := range []chan error{transitionDone, writerDone} {
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("writer did not finish")
		}
	}
	servers := readV2Config(t, path)["mcp"].(map[string]any)["servers"].(map[string]any)
	if len(servers) != 2 || servers["alpha"] == nil || servers["gamma"] == nil {
		t.Fatalf("writer lost transition: %#v", servers)
	}
}
