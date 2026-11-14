package usecase

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

var operationSequence atomic.Uint64

func operationID(prefix, integrationID string, t time.Time) string {
	prefix = sanitizeID(prefix)
	if len(prefix) > 12 {
		prefix = prefix[:12]
	}
	sum := sha256.Sum256([]byte(integrationID))
	return fmt.Sprintf("%s_%x_%d_%d", prefix, sum[:6], t.UnixNano(), operationSequence.Add(1))
}

func cleanupResolvedSource(source ports.ResolvedSource) {
	if source.Cleanup == nil {
		return
	}
	_ = source.Cleanup()
}

func cleanupPlannedExisting(items []plannedExistingTarget) {
	for _, item := range items {
		if item.Resolved != nil {
			cleanupResolvedSource(*item.Resolved)
		}
	}
}

func syncNeedsReplace(record domain.InstallationRecord, source string, desiredPolicy domain.InstallPolicy, desiredTargets []domain.TargetID, desiredVersion string) bool {
	if strings.TrimSpace(record.RequestedSourceRef.Value) != strings.TrimSpace(source) {
		return true
	}
	if record.Policy.Scope != desiredPolicy.Scope {
		return true
	}
	if len(record.Targets) != len(desiredTargets) {
		return true
	}
	currentTargets := sortedTargets(record.Targets)
	sort.Slice(desiredTargets, func(i, j int) bool { return desiredTargets[i] < desiredTargets[j] })
	for i := range currentTargets {
		if currentTargets[i] != desiredTargets[i] {
			return true
		}
	}
	if desiredVersion != "" && strings.TrimSpace(record.ResolvedVersion) != strings.TrimSpace(desiredVersion) {
		return false
	}
	return false
}

func syncNeedsUpdate(record domain.InstallationRecord, source, desiredVersion string) bool {
	if strings.TrimSpace(record.RequestedSourceRef.Value) != strings.TrimSpace(source) {
		return false
	}
	if strings.TrimSpace(desiredVersion) == "" {
		return false
	}
	return strings.TrimSpace(record.ResolvedVersion) != strings.TrimSpace(desiredVersion)
}

func sanitizeID(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "integration"
	}
	return b.String()
}
