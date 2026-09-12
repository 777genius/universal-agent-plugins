// Package packageview captures bounded local inputs for standard authoring.
// It does not parse manifests or assess conformance. Open reads only plugin.json
// and exact legacy metadata; after core decoding, Capture explicitly authorizes
// component and opaque-tree capture. Never call Capture after a fatal core/schema
// result. Convert these private input records to conformance types in the caller.
//
// Linux uses openat2/O_PATH. Darwin requires quiescent local APFS; Windows
// requires local fixed-drive NTFS. Native execution remains a release gate.
// The profile assumes a trusted kernel/mount namespace and ordinary local files;
// it is not an atomic filesystem snapshot or protection from the same principal
// modifying private storage. No source writes, execution, discovery, or network
// operations are provided. Reads may affect kernel filesystem accounting.
package packageview

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
)

const ScopeID = "agentplugins-captured-input-sha256-v1"

const TreeAlgorithm = "agentplugins-tree-sha256-v1"

type State string

const (
	Absent     State = "absent"
	Present    State = "present"
	Unreadable State = "unreadable"
	WrongKind  State = "wrong_kind"
	Blocked    State = "blocked"
)

// Document contains private exact bytes. Returned values are independent copies.
// These records are not public reports; all raw data is excluded from JSON.
type Document struct {
	Path  string `json:"-"`
	State State
	Bytes []byte `json:"-"`
}
type Skill struct {
	Directory string `json:"-"`
	Document  Document
}

// Observation describes one logical path without interpreting its content.
// Target is exact link text, not a command or a path to open outside this lease.
type Observation struct {
	Path       string `json:"-"`
	Kind       string
	State      State
	Target     string `json:"-"`
	Size       int64
	Executable bool
	Captured   bool
}

// Finding has only allowlisted codes/layers and an escaped, bounded relative
// location. It never contains source roots, contents, raw errors or link targets.
// Ordering is Layer, Code, Location. No finding is normative.
type Finding struct{ Layer, Code, Location string }

// Coverage describes acquisition only, never schema/profile/runtime evaluation.
// TreeComplete permits only the two existing installer exclusions: exact root
// .git and non-directory .plugin-kit-ai.lock. Legacy and unsafe omissions disable
// it. InventoryComplete concerns physical non-following enumeration of that scope.
type Coverage struct {
	ComponentsRequested bool
	SkillsEnumerated    bool
	InventoryComplete   bool
	TreeComplete        bool
}
type Identity struct {
	ScopeID       string
	Digest        string
	TreeAlgorithm string `json:",omitempty"`
	TreeDigest    string `json:",omitempty"`
}

// Input is private adapter evidence, not an installation snapshot or report DTO.
// Legacy is metadata-only, including safely resolved alias identity protection.
type Input struct {
	Plugin     Document
	MCP        Document
	SkillsRoot State
	Skills     []Skill
	Legacy     State
	Inventory  []Observation
	Coverage   Coverage
	Identity   Identity
	Findings   []Finding
}

// Limits are finite host budgets, not standard requirements. Zero selects the
// default; negative values or values above defaults are rejected. TotalBytes
// includes regular bytes, duplicate document aliases, and link text.
type Limits struct {
	Entries, Depth                                                          int
	FileBytes, TotalBytes, PluginBytes, MCPBytes, SkillBytes, DocumentBytes int64
}

func (v Limits) bounded() (Limits, error) {
	defaults := Limits{10000, 64, 64 << 20, 256 << 20, 1 << 20, 4 << 20, 1 << 20, 16 << 20}
	ints := []*int{&v.Entries, &v.Depth}
	di := []int{defaults.Entries, defaults.Depth}
	for i, p := range ints {
		if *p < 0 || *p > di[i] {
			return v, fail("invalid_limits")
		}
		if *p == 0 {
			*p = di[i]
		}
	}
	sizes := []*int64{&v.FileBytes, &v.TotalBytes, &v.PluginBytes, &v.MCPBytes, &v.SkillBytes, &v.DocumentBytes}
	ds := []int64{defaults.FileBytes, defaults.TotalBytes, defaults.PluginBytes, defaults.MCPBytes, defaults.SkillBytes, defaults.DocumentBytes}
	for i, p := range sizes {
		if *p < 0 || *p > ds[i] {
			return v, fail("invalid_limits")
		}
		if *p == 0 {
			*p = ds[i]
		}
	}
	return v, nil
}

// Reader has no ambient project/profile defaults. TempDir must be an explicit
// trusted scratch parent outside the selected source. It is never cleanup
// authority. Only the exact MkdirTemp child is owned by the returned lease.
type Reader struct {
	TempDir   string
	Limits    Limits
	Generated GeneratedStaging
}

// GeneratedStaging authorizes relaxing only the Darwin profile's read-only-mount
// source requirement, for exactly one caller-proven directory. Every other
// containment, symlink, type and mutation check stays exactly as strict; other
// platforms are unaffected (they never required a read-only mount). The zero
// value proves nothing and changes no behavior.
//
// It can be built only from a live *os.Root handle to that directory, never
// from a path string, so a caller that holds only a path -- every ordinary
// validate/inspect/test request -- can never construct one. Open still reopens
// by path and independently reverifies identity against this proof with
// os.SameFile before trusting it, so a stale or mismatched proof is rejected,
// not silently ignored.
type GeneratedStaging struct{ info os.FileInfo }

// NewGeneratedStaging captures live identity for dir by Stat-ing the already
// open handle. Callers must pass the exact directory they exclusively created
// and still hold open; the caller-held handle -- not a reopened path -- is the
// source of truth.
func NewGeneratedStaging(dir *os.Root) (GeneratedStaging, error) {
	if dir == nil {
		return GeneratedStaging{}, fail("generated_staging_required")
	}
	info, err := dir.Stat(".")
	if err != nil {
		return GeneratedStaging{}, fail("generated_staging_required")
	}
	return GeneratedStaging{info: info}, nil
}
func (g GeneratedStaging) present() bool { return g.info != nil }
func (g GeneratedStaging) matches(fi os.FileInfo) bool {
	return g.info != nil && fi != nil && os.SameFile(g.info, fi)
}

// Error is safe to print/serialize. Cancellation remains errors.Is-compatible.
// CleanupFailed signals that an operation's private artifact may remain.
type Error struct {
	Code          string
	CleanupFailed bool
	cancellation  error
}

func (e *Error) Error() string {
	if e.CleanupFailed {
		return "package input: " + e.Code + "; private cleanup failed"
	}
	return "package input: " + e.Code
}
func (e *Error) Unwrap() error { return e.cancellation }
func fail(code string) *Error  { return &Error{Code: code} }
func contextError(ctx context.Context) error {
	if e := ctx.Err(); e != nil {
		return &Error{Code: "canceled", cancellation: e}
	}
	return nil
}

// Lease is an operation-owned, serialized capture. Its zero value cannot remove
// anything. Close is idempotent (including returning the same cleanup error).
// Data returns deep copies; it remains usable after successful capture/Close.
// After a failed Capture it returns zero Input, never stale identity over partial
// bytes. No physical path or mutable cleanup token is exported. Callers defer
// Close immediately after Open.
type Lease struct {
	mu                sync.Mutex
	source            *source
	scratchClose      func() error
	private           string
	privateInfo       os.FileInfo
	limits            Limits
	input             Input
	directoryEntries  map[string][]string
	linkInfos         map[string]os.FileInfo
	observations      map[string]os.FileInfo
	contents          map[string][]byte
	total, documents  int64
	nodes             int
	completed, closed bool
	closeErr          error
	// Scheduling/fault seams are package-private and never runtime options.
	hooks *captureHooks
}
type captureHooks struct {
	scratchOpen func(string, int) // Darwin scratch traversal only
	nativeOpen  func(string, int) // immediately before Darwin single-name openat

	beforeDataOpen func(string)
	dataOpenError  func(string) error
	afterNameCheck func(string)
	afterChunk     func(string)
	metadata       func(string) error
	cleanup        func() error
}

func (r Reader) Open(ctx context.Context, exactRoot string) (_ *Lease, err error) {
	return r.open(ctx, exactRoot, nil)
}
func (r Reader) open(ctx context.Context, exactRoot string, hooks *captureHooks) (*Lease, error) {
	var l *Lease
	err := sourceIO(func() (err error) { l, err = r.openPhase(ctx, exactRoot, hooks); return }, func() {
		if l != nil {
			l.input = Input{}
			_ = l.close()
		}
	})
	if err != nil {
		markCleanupFailure(err, l)
		return nil, err
	}
	return l, nil
}
func (r Reader) openPhase(ctx context.Context, exactRoot string, hooks *captureHooks) (_ *Lease, err error) {
	if err = contextError(ctx); err != nil {
		return nil, err
	}
	limits, err := r.Limits.bounded()
	if err != nil {
		return nil, err
	}
	if exactRoot == "" || r.TempDir == "" {
		return nil, fail("explicit_roots_required")
	}
	// GeneratedStaging (r.Generated) is intentionally not threaded through here:
	// every platform's openSource already ignores it (see each source_*.go's
	// comment), since this profile already admits ordinary writable local
	// sources without needing that narrower proof. openSourceContext instead
	// carries ctx through for platforms (Darwin v2) that observe cancellation
	// during acquisition.
	s, err := openSourceContext(ctx, exactRoot)
	if err != nil {
		return nil, err
	}
	l := &Lease{source: s, limits: limits, observations: map[string]os.FileInfo{}, linkInfos: map[string]os.FileInfo{}, directoryEntries: map[string][]string{}, contents: map[string][]byte{}, hooks: hooks}
	defer l.finish(&err)
	s.sourceHooks(hooks)
	tmp, release, err := scratchParent(s, exactRoot, r.TempDir)
	if err != nil {
		return nil, err
	}
	l.scratchClose = release
	l.private, err = os.MkdirTemp(tmp, "packageview-*")
	if err != nil {
		return nil, fail("scratch_unavailable")
	}
	l.privateInfo, err = os.Lstat(l.private)
	if err != nil {
		return nil, fail("scratch_unavailable")
	}
	l.input = Input{Plugin: Document{Path: "plugin.json", State: Blocked}, MCP: Document{Path: "mcp.json", State: Blocked}, SkillsRoot: Blocked}
	l.input.Legacy, err = l.legacy()
	if err != nil {
		return nil, err
	}
	if l.input.Legacy != Absent {
		l.find("host", "legacy_manifest_ignored", "plugin/plugin.yaml")
	}
	l.input.Plugin, err = l.document(ctx, "plugin.json", limits.PluginBytes)
	if err != nil {
		return nil, err
	}
	l.find("host", "complete_tree_identity_unavailable", ".")
	l.identity()
	if err = contextError(ctx); err != nil {
		return nil, err
	}
	return l, nil
}

// finish releases operation resources on error and panic, including Open panics
// before a caller has received a lease. Cleanup failure during panic is surfaced
// as a sanitized panic; otherwise the original panic continues unchanged.
func (l *Lease) finish(err *error) {
	p := recover()
	if *err == nil && p == nil {
		return
	}
	l.input = Input{}
	ce := l.close()
	if p != nil {
		if ce != nil {
			panic(&Error{Code: "capture_panicked", CleanupFailed: true})
		}
		panic(p)
	}
	if ce != nil {
		var e *Error
		if errors.As(*err, &e) {
			e.CleanupFailed = true
		} else {
			*err = &Error{Code: "capture_failed", CleanupFailed: true}
		}
	}
}
func isParentRelative(p string) bool { return len(p) > 3 && p[:3] == ".."+string(filepath.Separator) }
func (l *Lease) Data() Input         { l.mu.Lock(); defer l.mu.Unlock(); return clone(l.input) }
func clone(v Input) Input {
	v.Plugin.Bytes = append([]byte(nil), v.Plugin.Bytes...)
	v.MCP.Bytes = append([]byte(nil), v.MCP.Bytes...)
	v.Skills = append([]Skill(nil), v.Skills...)
	for i := range v.Skills {
		v.Skills[i].Document.Bytes = append([]byte(nil), v.Skills[i].Document.Bytes...)
	}
	v.Inventory = append([]Observation(nil), v.Inventory...)
	v.Findings = append([]Finding(nil), v.Findings...)
	return v
}
func (l *Lease) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return l.closeErr
	}
	if l.source == nil && l.private == "" && l.scratchClose == nil {
		return l.close()
	}
	err := sourceIO(func() error {
		if l.source != nil {
			l.source.phaseContext(context.Background())
			if e := l.source.verifyBindings(); e != nil {
				return e
			}
		}
		return l.close()
	}, func() { l.input = Input{}; _ = l.close() })
	if err != nil {
		l.input = Input{}
		markCleanupFailure(err, l)
		l.closeErr = err
	}
	return err
}
func (l *Lease) close() error {
	if l.closed {
		return l.closeErr
	}
	l.closed = true
	failed := false
	if l.source != nil {
		failed = l.source.close() != nil
		l.source = nil
	}
	if l.private != "" {
		info, e := os.Lstat(l.private)
		if os.IsNotExist(e) { /* already absent */
		} else if e != nil || l.privateInfo == nil || !os.SameFile(info, l.privateInfo) || !info.IsDir() {
			failed = true
		} else {
			if l.hooks != nil && l.hooks.cleanup != nil {
				e = l.hooks.cleanup()
			} else {
				e = nil
			}
			if e == nil {
				current, statErr := os.Lstat(l.private)
				if statErr != nil || !os.SameFile(current, l.privateInfo) || !current.IsDir() {
					e = fail("cleanup_failed")
				}
			}
			if e == nil {
				e = os.Chmod(l.private, 0700)
			}
			if e == nil {
				e = os.RemoveAll(l.private)
			}
			failed = failed || e != nil
		}
	}
	if l.scratchClose != nil {
		failed = l.scratchClose() != nil || failed
		l.scratchClose = nil
	}
	if failed {
		l.closeErr = &Error{Code: "cleanup_failed", CleanupFailed: true}
	}
	return l.closeErr
}
func (l *Lease) find(layer, code, p string) {
	loc := strconv.QuoteToASCII(p)
	if len(loc) > 240 {
		sum := sha256.Sum256([]byte(p))
		loc = loc[:160] + "...#" + hex.EncodeToString(sum[:])
	}
	f := Finding{layer, code, loc}
	for _, old := range l.input.Findings {
		if old == f {
			return
		}
	}
	// At most two findings per bounded entry, plus fixed operation findings.
	l.input.Findings = append(l.input.Findings, f)
}
func (l *Lease) identity() {
	sort.Slice(l.input.Inventory, func(i, j int) bool { return l.input.Inventory[i].Path < l.input.Inventory[j].Path })
	sort.Slice(l.input.Findings, func(i, j int) bool {
		a, b := l.input.Findings[i], l.input.Findings[j]
		if a.Layer != b.Layer {
			return a.Layer < b.Layer
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		return a.Location < b.Location
	})
	// Independent capture framing: JSON struct/ordered arrays with exact raw path
	// strings represented as bytes (including invalid UTF-8), never installer ID.
	type record struct {
		Path                 []byte
		State                State
		Kind                 string
		Target               []byte
		Size                 int64
		Executable, Captured bool
		Digest               string
	}
	records := make([]record, 0, len(l.input.Inventory)+len(l.input.Skills)+2)
	addDoc := func(d Document) {
		sum := sha256.Sum256(d.Bytes)
		records = append(records, record{Path: []byte(d.Path), State: d.State, Kind: "document", Size: int64(len(d.Bytes)), Captured: d.State == Present, Digest: hex.EncodeToString(sum[:])})
	}
	addDoc(l.input.Plugin)
	addDoc(l.input.MCP)
	for _, s := range l.input.Skills {
		addDoc(s.Document)
	}
	for _, o := range l.input.Inventory {
		r := record{Path: []byte(o.Path), State: o.State, Kind: o.Kind, Target: []byte(o.Target), Size: o.Size, Executable: o.Executable, Captured: o.Captured}
		if b, ok := l.contents[o.Path]; ok {
			sum := sha256.Sum256(b)
			r.Digest = hex.EncodeToString(sum[:])
		}
		records = append(records, r)
	}
	h := sha256.New()
	_ = json.NewEncoder(h).Encode(struct {
		Scope, Profile     string
		Limits             Limits
		Coverage           Coverage
		Legacy, SkillsRoot State
		Records            []record
		Findings           []Finding
	}{ScopeID, ReadProfile, l.limits, l.input.Coverage, l.input.Legacy, l.input.SkillsRoot, records, l.input.Findings})
	l.input.Identity.ScopeID = ScopeID
	l.input.Identity.Digest = "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func markCleanupFailure(err error, l *Lease) {
	if l == nil || l.closeErr == nil {
		return
	}
	var cleanup *Error
	if !errors.As(l.closeErr, &cleanup) || !cleanup.CleanupFailed {
		return
	}
	var safe *Error
	if errors.As(err, &safe) {
		safe.CleanupFailed = true
	}
}
