package cursorhooks

import (
	"fmt"

	"github.com/tailscale/hujson"
)

type location struct {
	event string
	array *hujson.Array
	index int
}

func desiredEntry(shell ShellContract, spec HookSpec) (hujson.Value, error) {
	if err := absolutePath(spec.Selector); err != nil {
		return hujson.Value{}, err
	}
	command, err := RenderArgv(shell, specArgv(spec))
	if err != nil {
		return hujson.Value{}, err
	}
	obj := &hujson.Object{}
	addMember(obj, "type", hujson.Value{Value: hujson.String("command")})
	addMember(obj, "command", hujson.Value{Value: hujson.String(command)})
	addMember(obj, "timeout", hujson.Value{Value: hujson.Int(5)})
	addMember(obj, "failClosed", hujson.Value{Value: hujson.Bool(false)})
	return hujson.Value{Value: obj}, nil
}

func commandIdentity(entry hujson.Value) string {
	if obj, ok := entry.Value.(*hujson.Object); ok {
		if m := member(obj, "command"); m != nil {
			if s, ok := m.Value.Value.(hujson.Literal); ok && s.Kind() == '"' {
				return stringIdentity(s)
			}
		}
	}
	return ""
}

func matches(d *document, command string) []location {
	var found []location
	if d.hooks != nil {
		for _, event := range d.hooks.Members {
			arr := event.Value.Value.(*hujson.Array)
			for i, entry := range arr.Elements {
				if commandIdentity(entry) == command {
					found = append(found, location{
						event: stringIdentity(event.Name.Value.(hujson.Literal)), array: arr, index: i,
					})
				}
			}
		}
	}
	return found
}

func validateReceipt(r *Receipt) (hujson.Value, error) {
	if r == nil || r.Version != 1 || r.Event != "stop" || !validDigest(r.RemainderDigest) {
		return hujson.Value{}, fmt.Errorf("invalid ownership receipt")
	}
	entry, err := desiredEntry(r.Shell, r.Spec)
	if err != nil {
		return entry, err
	}
	if valueDigest("stop-entry", entry) != r.EntryDigest {
		return entry, fmt.Errorf("receipt does not bind fixed prior specification")
	}
	return entry, nil
}

func validDigest(s string) bool {
	if len(s) != 71 || s[:7] != "sha256:" {
		return false
	}
	for _, c := range s[7:] {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func ownedLocation(d *document, r *Receipt, entry hujson.Value) (*location, error) {
	found := matches(d, commandIdentity(entry))
	if len(found) == 0 {
		return nil, ErrAbsenceUnproven
	}
	if len(found) != 1 || found[0].event != stringIdentity(hujson.String("stop")) {
		return nil, fmt.Errorf("owned command collision or event drift")
	}
	loc := &found[0]
	if valueDigest("stop-entry", loc.array.Elements[loc.index]) != r.EntryDigest {
		return nil, fmt.Errorf("owned full entry drift")
	}
	return loc, nil
}

// VerifyOwned checks grammar and one unique complete owned stop entry. It does
// not require an unchanged foreign remainder or assert client activation.
func VerifyOwned(body []byte, receipt *Receipt) error {
	entry, err := validateReceipt(receipt)
	if err == nil {
		var d *document
		d, err = parseDocument(body)
		if err == nil {
			_, err = ownedLocation(d, receipt, entry)
		}
	}
	if err != nil {
		return fmt.Errorf("%w: %w", ErrConflict, err)
	}
	return nil
}
