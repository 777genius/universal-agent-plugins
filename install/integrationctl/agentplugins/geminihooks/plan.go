package geminihooks

import (
	"bytes"
	"fmt"
	"unicode/utf8"

	"github.com/tailscale/hujson"
)

var events = map[string]bool{
	"BeforeTool": true, "AfterTool": true, "BeforeAgent": true, "AfterAgent": true,
	"Notification": true, "SessionStart": true, "SessionEnd": true, "PreCompress": true,
	"BeforeModel": true, "AfterModel": true, "BeforeToolSelection": true,
}

type location struct {
	event string
	array *hujson.Array
	index int
}

// Index names across all event arrays, including future events. Foreign
// unknown values are opaque; VerifyOwned must not validate sibling schemas.
func indexNames(hooks *hujson.Object) map[string][]location {
	names := map[string][]location{}
	if hooks == nil {
		return names
	}
	for _, m := range hooks.Members {
		event := nativeString(m.Name.Value.(hujson.Literal))
		array, ok := m.Value.Value.(*hujson.Array)
		if !ok {
			continue
		}
		indexArrayNames(names, event, array)
	}
	return names
}

func indexArrayNames(names map[string][]location, event string, array *hujson.Array) {
	for i, g := range array.Elements {
		obj, ok := g.Value.(*hujson.Object)
		if !ok {
			continue
		}
		hs := member(obj, "hooks")
		if hs == nil {
			continue
		}
		entries, ok := hs.Value.Value.(*hujson.Array)
		if !ok {
			continue
		}
		for _, entry := range entries.Elements {
			if h, ok := entry.Value.(*hujson.Object); ok {
				if name := stringMember(h, "name"); name != "" {
					names[name] = append(names[name], location{event, array, i})
				}
			}
		}
	}
}

func verify(names map[string][]location, receipt *Receipt) error {
	if receipt == nil || receipt.Version != 1 || len(receipt.Groups) == 0 {
		return fmt.Errorf("missing/invalid receipt")
	}
	seen := map[string]bool{}
	for _, g := range receipt.Groups {
		if !events[g.Event] || g.Name == "" || !utf8.ValidString(g.Name) || seen[g.Name] {
			return fmt.Errorf("invalid receipt selectors")
		}
		seen[g.Name] = true
		matches := names[nativeKey(g.Name)]
		if len(matches) != 1 || matches[0].event != nativeKey(g.Event) {
			return fmt.Errorf("missing or colliding owned name %q", g.Name)
		}
		loc := matches[0]
		v := loc.array.Elements[loc.index]
		obj := v.Value.(*hujson.Object)
		hs := member(obj, "hooks").Value.Value.(*hujson.Array)
		digest, err := groupDigest(g.Event, v)
		if err != nil {
			return err
		}
		if len(hs.Elements) != 1 || digest != g.Digest {
			return fmt.Errorf("owned group %q changed", g.Name)
		}
	}
	return nil
}

// VerifyOwned checks only receipted groups; sibling changes do not invalidate
// ownership. It checks document grammar/ambiguity, not effective native policy.
func VerifyOwned(settings []byte, receipt *Receipt) error {
	_, root, err := parseSettings(settings)
	if err == nil {
		var hooks *hujson.Object
		hooks, err = hooksObject(root, false)
		if err == nil {
			err = verify(indexNames(hooks), receipt)
		}
	}
	if err != nil {
		return fmt.Errorf("%w: %w", ErrConflict, err)
	}
	return nil
}

func desiredGroup(shell Shell, h HookSpec) (hujson.Value, error) {
	if !events[h.Event] || h.Name == "" || h.Timeout < 0 {
		return hujson.Value{}, fmt.Errorf("invalid hook specification")
	}
	if err := safeString(h.Name); err != nil {
		return hujson.Value{}, err
	}
	if err := safeString(h.Matcher); err != nil {
		return hujson.Value{}, err
	}
	command, err := RenderArgv(shell, h.Argv)
	if err != nil {
		return hujson.Value{}, err
	}
	hook := &hujson.Object{}
	for _, kv := range [][2]string{{"type", "command"}, {"name", h.Name}, {"command", command}} {
		addMember(hook, kv[0], hujson.Value{Value: hujson.String(kv[1])})
	}
	if h.Timeout > 0 {
		addMember(hook, "timeout", hujson.Value{Value: hujson.Int(int64(h.Timeout))})
	}
	group := &hujson.Object{}
	if h.Matcher != "" {
		addMember(group, "matcher", hujson.Value{Value: hujson.String(h.Matcher)})
	}
	addMember(group, "hooks", hujson.Value{Value: &hujson.Array{Elements: []hujson.Value{{Value: hook}}}})
	return hujson.Value{Value: group}, nil
}

// Plan validates every input before editing its private AST. On conflict it
// returns original settings and no receipt; a no-op preserves bytes exactly.
func Plan(req Request) (Result, error) {
	unchanged := req.Settings
	if len(req.Settings) <= MaxSettingsBytes {
		unchanged = append([]byte(nil), req.Settings...)
	}
	conflict := func(err error) (Result, error) {
		return Result{Desired: unchanged, Conflict: true}, fmt.Errorf("%w: %w", ErrConflict, err)
	}
	if req.Operation != Install && req.Operation != Update && req.Operation != Remove {
		return conflict(fmt.Errorf("unsupported operation"))
	}
	ast, root, err := parseSettings(req.Settings)
	if err != nil {
		return conflict(err)
	}
	hooks, err := hooksObject(root, false)
	if err != nil {
		return conflict(err)
	}
	names := indexNames(hooks)
	old, err := previousGroups(req, names)
	if err != nil {
		return conflict(err)
	}
	desired, receipt, err := desiredGroups(req, hooks, names, old)
	if err != nil {
		return conflict(err)
	}
	if sameGroups(old, receipt) {
		return Result{Desired: unchanged, Receipt: receipt, NoOp: true}, nil
	}
	reconcileOwned(hooks, names, old, desired)
	if req.Operation != Remove {
		hooks, _ = hooksObject(root, true)
		appendDesired(hooks, req.Hooks, desired)
	} else {
		receipt = nil
	}
	body := ast.Pack()
	// Retain original comments/opaque data; never format through a lossy DTO.
	if _, _, err := parseSettings(body); err != nil {
		return conflict(err)
	}
	return Result{Desired: body, Receipt: receipt, NoOp: bytes.Equal(body, req.Settings)}, nil
}
