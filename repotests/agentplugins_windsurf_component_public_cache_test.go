//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/securityscan"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const windAssessmentDirectory = "security/assessments"

type windPublicCache struct {
	assessment domain.SecurityAssessment
	start, end time.Time
}

func windPublicAssessment(stdout []byte, r *windComponentRecipe, start, end time.Time) (windPublicCache, error) {
	refused := fmt.Errorf("component public assessment identity refused")
	value, err := projectNativeRuntimeJSON(stdout)
	outer, ok := value.(map[string]any)
	if err != nil || !ok || projectNativeRuntimeKeys(outer, "schema_version command result data", "") != nil || outer["schema_version"] != json.Number("1") || outer["command"] != "add" || outer["result"] != "success" {
		return windPublicCache{}, refused
	}
	data, ok := outer["data"].(map[string]any)
	if !ok || data["batch"] != true || data["succeeded"] != json.Number("1") || data["failed"] != json.Number("0") {
		return windPublicCache{}, refused
	}
	raw, err := json.Marshal(data["security"])
	if err != nil {
		return windPublicCache{}, refused
	}
	var assessment domain.SecurityAssessment
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&assessment) != nil {
		return windPublicCache{}, refused
	}
	if _, err := decoder.Token(); err != io.EOF {
		return windPublicCache{}, refused
	}
	cache := windPublicCache{assessment: assessment, start: start, end: end}
	if cache.validate(r) != nil {
		return windPublicCache{}, refused
	}
	return cache, nil
}

func (cache windPublicCache) validate(r *windComponentRecipe) error {
	subject := domain.SecuritySubject{TreeDigest: r.Digest, ManifestDigest: "sha256:" + windsurfConfigDigest(r.Authored["plugin.json"])}
	if (r.Scope != "user" && r.Scope != "a" && r.Scope != "b") || cache.end.Before(cache.start) || cache.end.Sub(cache.start) > 35*time.Second || cache.assessment.Validate(securityscan.DefaultRequirement(), subject) != nil || cache.assessment.Evidence != domain.SecurityEvidenceLocalScan || cache.assessment.Counts.Blocking != 0 {
		return fmt.Errorf("component exact cold public assessment required")
	}
	return nil
}

func windAssessmentParent(old, now windProfileRecord, start, end time.Time) bool {
	return old.Kind == "directory" && old.Mode == 040700 && old.Inode > 0 && old.Links > 0 && old.UID == uint32(os.Getuid()) && old.GID == uint32(os.Getgid()) && windRuntimeAuthority(old, now) && old.SHA256 == "" && now.SHA256 == "" && old.Size >= 0 && old.Size <= 1<<20 && now.Size >= 0 && now.Size <= 1<<20 && windRuntimeCommandTimes(now, start, end)
}

func windSourceScratchRoot(old, now windProfileRecord, start, end time.Time) bool {
	return old.Kind == "directory" && old.Mode == 040700 && old.Inode > 0 && old.Links > 0 && old.UID == uint32(os.Getuid()) && old.GID == uint32(os.Getgid()) && windRuntimeAuthority(old, now) && old.Size >= 0 && old.Size == now.Size && old.SHA256 == "" && now.SHA256 == "" && windRuntimeCommandTimes(now, start, end)
}

func (w *windComponentFixture) assessmentFrame(r *windComponentRecipe, frame map[string]windProfileRecord, cache windPublicCache) (string, error) {
	refused := fmt.Errorf("component exact assessment frame refused")
	if cache.validate(r) != nil {
		return "", refused
	}
	root := w.recipes["user"].StateRoot
	before := w.trees[root]
	if !windAssessmentParent(before[windAssessmentDirectory], frame[windAssessmentDirectory], cache.start, cache.end) {
		return "", refused
	}
	key := securityscan.CacheKey(cache.assessment.Subject, securityscan.DefaultRequirement())
	name := windAssessmentDirectory + "/" + key + ".json"
	if _, exists := before[name]; exists {
		return "", refused // This fixed first-capture recipe has three unique cold adds.
	}
	known := map[string]bool{}
	for _, recipe := range w.recipes {
		subject := domain.SecuritySubject{TreeDigest: recipe.Digest, ManifestDigest: "sha256:" + windsurfConfigDigest(recipe.Authored["plugin.json"])}
		known[windAssessmentDirectory+"/"+securityscan.CacheKey(subject, securityscan.DefaultRequirement())+".json"] = true
	}
	prior, current := 0, 0
	for path, record := range before {
		if strings.HasPrefix(path, windAssessmentDirectory+"/") {
			prior++
			if !known[path] || record != frame[path] {
				return "", refused
			}
		}
	}
	for path := range frame {
		if strings.HasPrefix(path, windAssessmentDirectory+"/") {
			current++
			if _, exists := before[path]; !exists && path != name {
				return "", refused
			}
		}
	}
	if prior >= 3 || current != prior+1 {
		return "", refused
	}
	assessment := cache.assessment
	assessment.Evidence = ""
	expected, err := json.Marshal(assessment)
	if err != nil || len(expected) > 1<<20 {
		return "", refused
	}
	raw, err := projectNativeRuntimeRead(filepath.Join(root, name), 1<<20)
	// Linux overlay filesystems may expose different st_dev values for a
	// directory and its regular children. The held no-follow inventory, not
	// a parent/child device-number assumption, proves the leaf's identity.
	actual, actualErr := windOwnedTree(w.f.ctx, root)
	file := frame[name]
	if err != nil || actualErr != nil || actual[name] != file || !bytes.Equal(raw, expected) || file.Kind != "file" || file.Mode != 0100600 || file.UID != uint32(os.Getuid()) || file.GID != uint32(os.Getgid()) || file.Links != 1 || file.Inode == 0 || file.Device == 0 || file.Size != int64(len(expected)) || file.SHA256 != windsurfConfigDigest(expected) || !windRuntimeCommandTimes(file, cache.start, cache.end) {
		return "", refused
	}
	return name, nil
}

func (w *windComponentFixture) publicStateFrames(r *windComponentRecipe, frames map[string]map[string]windProfileRecord, cache *windPublicCache) error {
	global := w.recipes["user"].StateRoot
	before, after := w.trees[global], frames[global]
	leaf := ""
	if cache != nil {
		var err error
		leaf, err = w.assessmentFrame(r, after, *cache)
		if err != nil {
			return err
		}
	}
	if r.Scope != "user" {
		added := 0
		if cache != nil {
			added = 1
		}
		if len(after) != len(before)+added {
			return fmt.Errorf("component global state membership differs")
		}
		for name, old := range before {
			now, exists := after[name]
			if exists && old == now {
				continue
			}
			if cache != nil && (name == windAssessmentDirectory || name == "." && windSourceScratchRoot(old, now, cache.start, cache.end)) {
				continue
			}
			return fmt.Errorf("component public add changed unrelated global state")
		}
		for name := range after {
			if _, existed := before[name]; !existed && name != leaf {
				return fmt.Errorf("component public add added unrelated global state")
			}
		}
	}
	if r.Scope != "user" {
		leaf = ""
	}
	return w.scopeFrameClosure(r, frames[r.StateRoot], leaf)
}
