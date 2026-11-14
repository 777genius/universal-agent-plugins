package geminihooks

import (
	"fmt"

	"github.com/tailscale/hujson"
)

func previousGroups(req Request, names map[string][]location) (map[string]OwnedGroup, error) {
	old := map[string]OwnedGroup{}
	if req.Previous != nil || req.Operation != Install {
		if err := verify(names, req.Previous); err != nil {
			return nil, err
		}
		for _, g := range req.Previous.Groups {
			old[g.Name] = g
		}
	}
	return old, nil
}

func desiredGroups(req Request, hooks *hujson.Object, names map[string][]location, old map[string]OwnedGroup) (map[string]hujson.Value, *Receipt, error) {
	desired := map[string]hujson.Value{}
	receipt := &Receipt{Version: 1}
	if req.Operation == Remove {
		return desired, receipt, nil
	}
	if len(req.Hooks) == 0 {
		return nil, nil, fmt.Errorf("empty desired hook set; use remove")
	}
	for _, h := range req.Hooks {
		if err := validateEventArray(hooks, h.Event); err != nil {
			return nil, nil, err
		}
		if _, ok := desired[h.Name]; ok {
			return nil, nil, fmt.Errorf("duplicate requested name")
		}
		v, err := desiredGroup(req.Shell, h)
		if err != nil {
			return nil, nil, err
		}
		if _, owned := old[h.Name]; !owned && len(names[nativeKey(h.Name)]) > 0 {
			return nil, nil, fmt.Errorf("name collision %q; no adoption", h.Name)
		}
		desired[h.Name] = v
		digest, err := groupDigest(h.Event, v)
		if err != nil {
			return nil, nil, err
		}
		receipt.Groups = append(receipt.Groups, OwnedGroup{Event: h.Event, Name: h.Name, Digest: digest})
	}
	return desired, receipt, nil
}

func validateEventArray(hooks *hujson.Object, event string) error {
	if hooks != nil {
		if m := member(hooks, event); m != nil {
			if _, ok := m.Value.Value.(*hujson.Array); !ok {
				return fmt.Errorf("requested event must be an array")
			}
		}
	}
	return nil
}

// Compare sets by name/event/full group digest, independent of receipt order.
func sameGroups(old map[string]OwnedGroup, receipt *Receipt) bool {
	same := len(old) == len(receipt.Groups)
	for _, g := range receipt.Groups {
		if old[g.Name] != g {
			same = false
		}
	}
	return same
}

// Replace same-event owned groups in place; remove retired/moved groups
// in reverse index order. Preserve surrounding comments on removal too.
func reconcileOwned(hooks *hujson.Object, names map[string][]location, old map[string]OwnedGroup, desired map[string]hujson.Value) {
	if hooks == nil {
		return
	}
	for _, event := range hooks.Members {
		a, ok := event.Value.Value.(*hujson.Array)
		if !ok {
			continue
		}
		reconcileEvent(a, nativeString(event.Name.Value.(hujson.Literal)), names, old, desired)
	}
}

func reconcileEvent(a *hujson.Array, event string, names map[string][]location, old map[string]OwnedGroup, desired map[string]hujson.Value) {
	for i := len(a.Elements) - 1; i >= 0; i-- {
		for name := range old {
			loc := names[nativeKey(name)][0]
			if loc.array != a || loc.index != i {
				continue
			}
			if v, ok := desired[name]; ok && nativeKey(old[name].Event) == event {
				v.BeforeExtra, v.AfterExtra = a.Elements[i].BeforeExtra, a.Elements[i].AfterExtra
				a.Elements[i] = v
				delete(desired, name)
			} else {
				removeGroup(a, i)
			}
		}
	}
}

func removeGroup(a *hujson.Array, i int) {
	comments := append(append(hujson.Extra{}, a.Elements[i].BeforeExtra...), a.Elements[i].AfterExtra...)
	if i+1 < len(a.Elements) {
		a.Elements[i+1].BeforeExtra = append(comments, a.Elements[i+1].BeforeExtra...)
	} else {
		a.AfterExtra = append(comments, a.AfterExtra...)
	}
	a.Elements = append(a.Elements[:i], a.Elements[i+1:]...)
}

func appendDesired(hooks *hujson.Object, specs []HookSpec, desired map[string]hujson.Value) {
	for _, h := range specs {
		v, pending := desired[h.Name]
		if !pending {
			continue
		}
		m := member(hooks, h.Event)
		if m == nil {
			addMember(hooks, h.Event, hujson.Value{Value: &hujson.Array{}})
			m = member(hooks, h.Event)
		}
		a := m.Value.Value.(*hujson.Array)
		a.Elements = append(a.Elements, v)
	}
}
