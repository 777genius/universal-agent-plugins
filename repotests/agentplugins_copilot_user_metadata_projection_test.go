//go:build linux

package pluginkitairepo_test

import (
	"encoding/json"
	"sort"
)

const copilotUserMetadataLimit = 2048

type copilotUserMetadataChange struct {
	PathSHA256        string `json:"path_sha256"`
	BeforeKind        string `json:"before_kind"`
	AfterKind         string `json:"after_kind"`
	BeforePermissions uint32 `json:"before_permissions"`
	AfterPermissions  uint32 `json:"after_permissions"`
	BeforePresent     bool   `json:"before_present"`
	AfterPresent      bool   `json:"after_present"`
	OwnerEqual        bool   `json:"owner_equal"`
	GroupEqual        bool   `json:"group_equal"`
	IdentityEqual     bool   `json:"identity_equal"`
	ModeEqual         bool   `json:"mode_equal"`
}
type copilotUserMetadataProjection struct {
	Stage          string                      `json:"stage"`
	Status         string                      `json:"status"`
	BeforeEntries  int                         `json:"before_entries"`
	AfterEntries   int                         `json:"after_entries"`
	ChangedEntries int                         `json:"changed_entries"`
	OmittedCount   int                         `json:"omitted_count"`
	Entries        []copilotUserMetadataChange `json:"entries"`
}

func copilotUserMetadataRefusal() []byte {
	return []byte(`{"stage":"add-user","status":"refused"}`)
}

func copilotUserMetadataDelta(before, after *copilotUserMetadata) []byte {
	return copilotMetadataDeltaAtStage(before, after, "add-user")
}

func copilotMetadataRefusalAtStage(stage string) []byte {
	switch stage {
	case "add-user":
		return copilotUserMetadataRefusal()
	case "native-list":
		return []byte(`{"stage":"native-list","status":"refused"}`)
	default:
		return nil
	}
}

func copilotMetadataDeltaAtStage(before, after *copilotUserMetadata, stage string) []byte {
	refused := copilotMetadataRefusalAtStage(stage)
	if refused == nil {
		return nil
	}
	if before == nil || after == nil || len(before.nodes) == 0 || len(after.nodes) == 0 || len(before.nodes) > copilotUserMetadataEntries || len(after.nodes) > copilotUserMetadataEntries || len(before.parents) != len(after.parents) {
		return refused
	}
	for i, parent := range before.parents {
		if !copilotUserMetadataIdentity(parent, after.parents[i]) {
			return refused
		}
	}
	if !copilotUserMetadataIdentity(before.nodes["."].stat, after.nodes["."].stat) {
		return refused
	}
	keys := map[string]bool{}
	for name := range before.nodes {
		keys[name] = true
	}
	for name := range after.nodes {
		keys[name] = true
	}
	changes := make([]copilotUserMetadataChange, 0, len(keys))
	for name := range keys {
		a, had := before.nodes[name]
		b, has := after.nodes[name]
		if had && has && copilotUserMetadataStable(a.stat, b.stat) {
			continue
		}
		change := copilotUserMetadataChange{PathSHA256: copilotProjectConfigHash([]byte(name)), BeforeKind: "absent", AfterKind: "absent", BeforePresent: had, AfterPresent: has}
		if had {
			change.BeforeKind, change.BeforePermissions = a.kind, a.stat.Mode&07777
		}
		if has {
			change.AfterKind, change.AfterPermissions = b.kind, b.stat.Mode&07777
		}
		if had && has {
			change.OwnerEqual, change.GroupEqual = a.stat.Uid == b.stat.Uid, a.stat.Gid == b.stat.Gid
			change.IdentityEqual, change.ModeEqual = a.stat.Dev == b.stat.Dev && a.stat.Ino == b.stat.Ino, a.stat.Mode == b.stat.Mode
		}
		changes = append(changes, change)
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].PathSHA256 < changes[j].PathSHA256 })
	count := min(len(changes), 16)
	for {
		projection := copilotUserMetadataProjection{stage, "observed", len(before.nodes), len(after.nodes), len(changes), len(changes) - count, append([]copilotUserMetadataChange{}, changes[:count]...)}
		body, err := json.Marshal(projection)
		if err == nil && len(body) <= copilotUserMetadataLimit {
			return body
		}
		if count == 0 {
			return refused
		}
		count--
	}
}
