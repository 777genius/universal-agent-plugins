//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/securityscan"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func windCycleCold(label string) bool {
	return label == "add-user" || label == "add-a" || label == "add-b" || label == "update-a"
}

func windCycleSecurity(value any, found *[]any) {
	switch row := value.(type) {
	case map[string]any:
		for k, v := range row {
			if k == "security" {
				*found = append(*found, v)
			} else {
				windCycleSecurity(v, found)
			}
		}
	case []any:
		for _, v := range row {
			windCycleSecurity(v, found)
		}
	}
}

func (c *windProjectCycle) cacheProof(label string, r *windComponentRecipe, frames map[string]map[string]windProfileRecord, stdout []byte, start, end time.Time) (map[string]any, error) {
	refused := fmt.Errorf("Windsurf exact stage assessment proof refused")
	global := c.w.recipes["user"].StateRoot
	before, after := c.w.trees[global], frames[global]
	subject := domain.SecuritySubject{TreeDigest: r.Digest, ManifestDigest: "sha256:" + clineAttemptSHA(r.Authored["plugin.json"])}
	key := securityscan.CacheKey(subject, securityscan.DefaultRequirement())
	name := windAssessmentDirectory + "/" + key + ".json"
	raw, err := projectNativeRuntimeRead(filepath.Join(global, name), 1<<20)
	if err != nil {
		return nil, refused
	}
	if _, err := projectNativeRuntimeJSON(raw); err != nil {
		return nil, refused
	}
	var assessment domain.SecurityAssessment
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&assessment) != nil {
		return nil, refused
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, refused
	}
	canonical, err := json.Marshal(assessment)
	if err != nil || !bytes.Equal(raw, canonical) || assessment.Validate(securityscan.DefaultRequirement(), subject) != nil || assessment.Evidence != "" || assessment.Counts != (domain.SecurityCounts{}) || assessment.ScannedFiles != 3 || len(assessment.Findings) != 0 {
		return nil, refused
	}
	file := after[name]
	if file.Kind != "file" || file.Mode != 0100600 || file.UID != uint32(os.Getuid()) || file.GID != uint32(os.Getgid()) || file.Links != 1 || file.Inode == 0 || file.Device != after[windAssessmentDirectory].Device || file.Size != int64(len(raw)) || file.SHA256 != clineAttemptSHA(raw) {
		return nil, refused
	}
	cold := windCycleCold(label)
	prior := []string{}
	for path, old := range before {
		if strings.HasPrefix(path, windAssessmentDirectory+"/") {
			prior = append(prior, path)
			if !c.cacheKeys[path] || old != after[path] {
				return nil, refused
			}
		}
	}
	sort.Strings(prior)
	count := 0
	for path := range after {
		if strings.HasPrefix(path, windAssessmentDirectory+"/") {
			count++
			if !c.cacheKeys[path] && (!cold || path != name) {
				return nil, refused
			}
		}
	}
	if cold {
		if _, exists := before[name]; exists || count != len(prior)+1 || len(prior) >= 4 || !windRuntimeCommandTimes(file, start, end) || !windAssessmentParent(before[windAssessmentDirectory], after[windAssessmentDirectory], start, end) {
			return nil, refused
		}
	} else if !c.cacheKeys[name] || count != len(prior) || before[windAssessmentDirectory] != after[windAssessmentDirectory] {
		return nil, refused
	}
	security := []any{}
	if label != "repeat-remove-a" {
		value, err := projectNativeRuntimeJSON(stdout)
		if err != nil {
			return nil, refused
		}
		windCycleSecurity(value, &security)
		if strings.HasPrefix(label, "add-") || label == "repeat-add-a" {
			outer, ok := value.(map[string]any)
			if !ok {
				return nil, refused
			}
			data, ok := outer["data"].(map[string]any)
			if !ok || data["security"] == nil {
				return nil, refused
			}
		}
	}
	present := strings.HasPrefix(label, "add-") || label == "repeat-add-a"
	if present {
		if len(security) != 1 {
			return nil, refused
		}
		body, _ := json.Marshal(security[0])
		var public domain.SecurityAssessment
		d := json.NewDecoder(bytes.NewReader(body))
		d.DisallowUnknownFields()
		if d.Decode(&public) != nil {
			return nil, refused
		}
		want := assessment
		want.Evidence = domain.SecurityEvidenceLocalScan
		if label == "repeat-add-a" {
			want.Evidence = domain.SecurityEvidenceCache
		}
		if !reflect.DeepEqual(public, want) {
			return nil, refused
		}
	} else if len(security) != 0 {
		return nil, refused
	}
	phase := "existing"
	if cold {
		phase = "cold"
		c.cacheKeys[name] = true
	}
	return map[string]any{"role": "assessment", "scope": r.Scope, "version": windCycleVersion(r), "source_digest": r.Digest, "manifest_digest": subject.ManifestDigest, "path": filepath.Join(global, name), "key": key, "phase": phase, "prior": prior, "assessment": windCycleStream(raw), "before_file": before[name], "after_file": file, "before_parent": before[windAssessmentDirectory], "after_parent": after[windAssessmentDirectory]}, nil
}
