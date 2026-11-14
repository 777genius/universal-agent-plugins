package targetcontracts

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/sdk/platformmeta"
)

func All() []Entry {
	profiles := platformmeta.All()
	out := make([]Entry, 0, len(profiles))
	for _, profile := range profiles {
		out = append(out, fromProfile(profile))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Target < out[j].Target
	})
	return out
}

func ByTarget(name string) []Entry {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return All()
	}
	out := make([]Entry, 0, 1)
	for _, entry := range All() {
		if entry.Target == name {
			out = append(out, entry)
		}
	}
	return out
}

func Lookup(name string) (Entry, bool) {
	profile, ok := platformmeta.Lookup(name)
	if !ok {
		return Entry{}, false
	}
	return fromProfile(profile), true
}

func JSON(entries []Entry) ([]byte, error) {
	return json.MarshalIndent(entries, "", "  ")
}
