package packageview

import (
	"context"
	"errors"
	"io"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
)

// Capture is the explicit second stage, called only after the caller accepts the
// captured core document/schema. It captures MCP, immediate Skills and inert
// inventory. It never interprets them. On fatal I/O/change/budget/cancel it closes
// the lease and returns no partial Input. Component availability findings retain
// safe siblings. Repeated successful calls return independent copies.
func (l *Lease) Capture(ctx context.Context) (_ Input, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return Input{}, fail("lease_closed")
	}
	defer l.finish(&err)
	if err = contextError(ctx); err != nil {
		return Input{}, err
	}
	if l.completed {
		return clone(l.input), nil
	}
	if l.input.Plugin.State != Present {
		return Input{}, fail("core_unavailable")
	}
	l.input.Coverage.ComponentsRequested = true
	l.input.MCP, err = l.document(ctx, "mcp.json", l.limits.MCPBytes)
	if err != nil {
		return Input{}, err
	}
	names, state, err := l.list(ctx, "skills")
	if err != nil {
		return Input{}, err
	}
	l.input.SkillsRoot = state
	l.input.Coverage.SkillsEnumerated = state == Present || state == Absent
	if state != Present && state != Absent {
		l.find("host", "skills_"+string(state), "skills")
	}
	for _, name := range names {
		rel := "skills/" + name
		p, e := l.metadata(rel, false)
		if e != nil {
			d := Document{Path: rel + "/SKILL.md", State: stateOf(e)}
			l.input.Skills = append(l.input.Skills, Skill{rel, d})
			l.find("host", "skill_"+string(d.State), rel)
			continue
		}
		isdir := p.info.IsDir()
		ce := p.file.Close()
		if ce != nil {
			return Input{}, fail("close_failed")
		}
		if !isdir {
			continue
		}
		d, e := l.document(ctx, rel+"/SKILL.md", l.limits.SkillBytes)
		if e != nil {
			return Input{}, e
		}
		l.input.Skills = append(l.input.Skills, Skill{rel, d})
	}
	l.input.Coverage.InventoryComplete = true
	l.input.Coverage.TreeComplete = l.input.Legacy == Absent && l.input.MCP.State != Unreadable && l.input.MCP.State != Blocked && l.input.SkillsRoot != Unreadable && l.input.SkillsRoot != Blocked
	for _, skill := range l.input.Skills {
		if skill.Document.State == Unreadable || skill.Document.State == Blocked {
			l.input.Coverage.TreeComplete = false
		}
	}
	if err = l.walk(ctx, ".", 0); err != nil {
		return Input{}, err
	}
	// Revalidate every captured file and enumerated directory. This detects common
	// same-inode changes as well as replacements; it is not an atomic source lock.
	paths := make([]string, 0, len(l.observations))
	for p := range l.observations {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, rel := range paths {
		if err = contextError(ctx); err != nil {
			return Input{}, err
		}
		p, e := l.source.pin(rel, false)
		if e != nil {
			return Input{}, fail("source_changed")
		}
		ok := same(l.observations[rel], p.info)
		if ok && p.info.IsDir() {
			if e := l.verifyDirectory(ctx, rel, p); e != nil {
				_ = p.file.Close()
				return Input{}, e
			}
		}
		if ok && p.info.Mode().IsRegular() {
			if e := l.verifyCaptured(ctx, rel, p); e != nil {
				_ = p.file.Close()
				return Input{}, e
			}
		}
		ce := p.file.Close()
		if !ok {
			return Input{}, fail("source_changed")
		}
		if ce != nil {
			return Input{}, fail("close_failed")
		}
	}
	// Link identities/text were captured with no-follow handles. Verify again
	// before claiming coverage; regular targets were validated independently.
	for _, o := range l.input.Inventory {
		if o.Kind != "symlink" || !o.Captured {
			continue
		}
		p, e := l.source.pin(o.Path, true)
		if e != nil {
			return Input{}, fail("source_changed")
		}
		target, e := p.link(l.limits.FileBytes)
		ok := same(l.linkInfos[o.Path], p.info)
		ce := p.file.Close()
		if e != nil || !ok || target != o.Target {
			return Input{}, fail("source_changed")
		}
		if ce != nil {
			return Input{}, fail("close_failed")
		}
	}
	legacy, e := l.legacy()
	if e != nil {
		return Input{}, e
	}
	if legacy != l.input.Legacy {
		return Input{}, fail("source_changed")
	}
	if l.input.Coverage.TreeComplete {
		retained := make(map[string]Observation, len(l.input.Inventory))
		for _, o := range l.input.Inventory {
			retained[o.Path] = o
		}
		for _, o := range l.input.Inventory {
			if err = contextError(ctx); err != nil {
				return Input{}, err
			}
			if o.Kind == "symlink" && !retainedLinkResolves(o.Path, retained) {
				l.input.Coverage.TreeComplete = false
				break
			}
		}
	}
	if l.input.Coverage.TreeComplete {
		entries := make([]packagedigest.CapturedEntry, 0, len(l.input.Inventory))
		for _, o := range l.input.Inventory {
			e := packagedigest.CapturedEntry{Path: o.Path, Kind: o.Kind, Executable: o.Executable, Target: o.Target}
			if o.Kind == "file" {
				e.Content = l.contents[o.Path]
			}
			entries = append(entries, e)
		}
		digest, e := packagedigest.DigestCaptured(ctx, entries)
		if errors.Is(e, packagedigest.ErrCapturedPolicy) {
			l.input.Coverage.TreeComplete = false
			l.find("installer_policy", "tree_digest_policy_unavailable", ".")
		} else if e != nil {
			if ctx.Err() != nil {
				return Input{}, contextError(ctx)
			}
			return Input{}, fail("digest_failed")
		} else {
			l.input.Identity.TreeAlgorithm = TreeAlgorithm
			l.input.Identity.TreeDigest = digest
		}
	}
	if !l.input.Coverage.TreeComplete {
		l.find("host", "complete_tree_identity_unavailable", ".")
	} else {
		l.input.Findings = slices.DeleteFunc(l.input.Findings, func(f Finding) bool { return f.Layer == "host" && f.Code == "complete_tree_identity_unavailable" })
	}
	if err = contextError(ctx); err != nil {
		return Input{}, err
	}
	l.identity()
	if err = contextError(ctx); err != nil {
		return Input{}, err
	}
	if err = os.Chmod(l.private, 0500); err != nil {
		return Input{}, fail("storage_failed")
	}
	if err = l.source.close(); err != nil {
		return Input{}, fail("close_failed")
	}
	l.source = nil
	l.completed = true
	return clone(l.input), nil
}

// list performs incremental enumeration; count/depth precede slice growth.
// Nodes count enumeration work, including the second immediate-Skills visit.
func (l *Lease) list(ctx context.Context, rel string) ([]string, State, error) {
	if e := contextError(ctx); e != nil {
		return nil, Blocked, e
	}
	p, e := l.metadata(rel, false)
	if e != nil {
		return nil, stateOf(e), nil
	}
	defer p.file.Close()
	if !p.info.IsDir() {
		return nil, WrongKind, nil
	}
	f, e := p.reopen(true)
	if e != nil {
		var safe *Error
		if errors.As(e, &safe) {
			return nil, Blocked, e
		}
		return nil, Unreadable, nil
	}
	defer f.Close()
	var names []string
	for {
		if e := contextError(ctx); e != nil {
			return nil, Blocked, e
		}
		items, e := f.ReadDir(1)
		for _, item := range items {
			name := item.Name()
			// Exact existing installer exclusions; never descend .git. A marker
			// directory remains included and will fail installer portability policy.
			if rel == "." && name == ".git" {
				continue
			}
			l.nodes++
			if l.nodes > l.limits.Entries {
				return nil, Blocked, fail("entry_limit")
			}
			if len(name) > 4096 {
				return nil, Blocked, fail("path_limit")
			}
			names = append(names, name)
		}
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, Unreadable, nil
		}
	}
	after, e := f.Stat()
	if e != nil || !same(p.info, after) {
		return nil, Blocked, fail("source_changed")
	}
	if prior, ok := l.observations[rel]; ok && !same(prior, p.info) {
		return nil, Blocked, fail("source_changed")
	}
	l.observations[rel] = p.info
	sort.Strings(names)
	if prior, ok := l.directoryEntries[rel]; ok && !slices.Equal(prior, names) {
		return nil, Blocked, fail("source_changed")
	}
	l.directoryEntries[rel] = names
	return names, Present, nil
}

// verifyDirectory compares exact entry sets as well as metadata. Directory
// timestamp resolution alone cannot detect additions in the same clock tick.
// This pass allocates at most the already-budgeted name count and rejects the
// first unknown/duplicate entry; it never captures newly appeared content.
func (l *Lease) verifyDirectory(ctx context.Context, rel string, p *pinned) error {
	expected := l.directoryEntries[rel]
	remaining := make(map[string]bool, len(expected))
	for _, name := range expected {
		remaining[name] = true
	}
	f, e := p.reopen(true)
	if e != nil {
		return fail("source_changed")
	}
	defer f.Close()
	for {
		if e := contextError(ctx); e != nil {
			return e
		}
		items, e := f.ReadDir(1)
		for _, item := range items {
			name := item.Name()
			if rel == "." && name == ".git" {
				continue
			}
			if !remaining[name] {
				return fail("source_changed")
			}
			delete(remaining, name)
		}
		if e == io.EOF {
			break
		}
		if e != nil {
			return fail("source_changed")
		}
	}
	if len(remaining) != 0 {
		return fail("source_changed")
	}
	after, e := f.Stat()
	if e != nil || !same(p.info, after) {
		return fail("source_changed")
	}
	return nil
}

func (l *Lease) walk(ctx context.Context, dir string, depth int) error {
	if depth > l.limits.Depth {
		return fail("depth_limit")
	}
	names, state, e := l.list(ctx, dir)
	if e != nil {
		return e
	}
	if state != Present {
		l.input.Coverage.InventoryComplete = false
		l.input.Coverage.TreeComplete = false
		l.find("host", "inventory_"+string(state), dir)
		return nil
	}
	for _, name := range names {
		if e := contextError(ctx); e != nil {
			return e
		}
		rel := name
		if dir != "." {
			rel = dir + "/" + name
		}
		if depth+1 > l.limits.Depth || len(rel) > 4096 {
			return fail("depth_limit")
		}
		if rel == "plugin/plugin.yaml" {
			l.input.Inventory = append(l.input.Inventory, Observation{Path: rel, Kind: "excluded", State: l.input.Legacy})
			l.input.Coverage.TreeComplete = false
			continue
		}
		p, e := l.metadata(rel, true)
		if e != nil {
			l.omit(Observation{Path: rel, State: stateOf(e)}, "inventory_"+string(stateOf(e)))
			continue
		}
		if rel == ".plugin-kit-ai.lock" && !p.info.IsDir() {
			if ce := p.file.Close(); ce != nil {
				return fail("close_failed")
			}
			continue
		}
		o := Observation{Path: rel, State: Present, Size: p.info.Size()}
		switch {
		case p.info.IsDir():
			o.Kind = "directory"
			o.Size = 0
			o.Captured = true
		case p.info.Mode()&os.ModeSymlink != 0:
			o.Kind = "symlink"
			target, e := p.link(min(l.limits.FileBytes, l.limits.TotalBytes-l.total))
			if e != nil {
				_ = p.file.Close()
				var safe *Error
				if errors.As(e, &safe) {
					return e
				}
				return fail("link_unavailable")
			}
			o.Target = target
			o.Size = int64(len(target))
			l.total += o.Size
			q, e := l.source.pin(rel, false)
			if e != nil {
				o.State = stateOf(e)
			} else {
				if !q.info.IsDir() && !q.info.Mode().IsRegular() {
					o.State = WrongKind
				}
				if q.info.Mode().IsRegular() && !l.legacyGuard(q.info) {
					o.State = Blocked
				}
				if ce := q.file.Close(); ce != nil {
					_ = p.file.Close()
					return fail("close_failed")
				}
			}
			o.Captured = o.State == Present
			if o.Captured {
				l.linkInfos[rel] = p.info
			}
		case p.info.Mode().IsRegular():
			o.Kind = "file"
			o.Executable = p.info.Mode()&0111 != 0
			if !l.legacyGuard(p.info) {
				o.State = Blocked
			} else {
				_, e := l.read(ctx, rel, p, l.limits.FileBytes)
				if e != nil {
					var safe *Error
					if errors.As(e, &safe) {
						_ = p.file.Close()
						return e
					}
					o.State = Unreadable
				} else {
					o.Captured = true
				}
			}
		default:
			o.Kind = "special"
			o.State = WrongKind
		}
		if ce := p.file.Close(); ce != nil {
			return fail("close_failed")
		}
		if !o.Captured {
			l.omit(o, "inventory_"+string(o.State))
		} else {
			l.input.Inventory = append(l.input.Inventory, o)
		}
		if o.Kind == "directory" {
			if e := l.walk(ctx, rel, depth+1); e != nil {
				return e
			}
		}
	}
	return nil
}
func (l *Lease) omit(o Observation, code string) {
	l.input.Inventory = append(l.input.Inventory, o)
	l.input.Coverage.TreeComplete = false
	if o.Kind == "" || o.State == Unreadable || o.State == Absent {
		l.input.Coverage.InventoryComplete = false
	}
	l.find("host", code, o.Path)
}

// retainedLinkResolves checks representation, not source containment (which the
// rooted reader establishes separately). Consume components before processing
// later "..": even a directory removed from the final spelling must be retained.
// Only inventory is consulted. Inventory paths/targets are already bounded to
// 4096 bytes; at most 40 link expansions bound work and pending components per
// link, matching the existing Linux reader ceiling. No recursion or filesystem IO.
func retainedLinkResolves(rel string, retained map[string]Observation) bool {
	pending := strings.Split(rel, "/")
	var dirs []string
	links := 0
	for len(pending) > 0 {
		part := pending[0]
		pending = pending[1:]
		switch part {
		case "", ".":
			continue
		case "..":
			if len(dirs) == 0 {
				return false
			}
			dirs = dirs[:len(dirs)-1]
			continue
		}
		name := part
		if len(dirs) > 0 {
			name = strings.Join(dirs, "/") + "/" + part
		}
		o, ok := retained[name]
		if !ok || !o.Captured || o.State != Present {
			return false
		}
		switch o.Kind {
		case "directory":
			dirs = append(dirs, part)
		case "file":
			return len(pending) == 0
		case "symlink":
			links++
			if links > 40 || o.Target == "" || strings.HasPrefix(o.Target, "/") {
				return false
			}
			pending = append(strings.Split(o.Target, "/"), pending...)
		default:
			return false
		}
	}
	return true
}
