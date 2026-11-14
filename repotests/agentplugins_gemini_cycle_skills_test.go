//go:build linux

package pluginkitairepo_test

import (
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
)

type geminiCycleSkillRow struct {
	Description, Path string
	Enabled, Builtin  bool
}

var geminiCycleSGR = regexp.MustCompile(`\x1b\[[0-9;]*m`)
var geminiCycleSkillHeader = regexp.MustCompile(`^(\S.*?) \[(Enabled|Disabled)\]( \[Built-in\])?$`)
var geminiCycleBuiltinSHA = map[string]string{"antigravity-support": "2b866bce962f642be8c0dd74ffc2548fa29f2100bc945518b2f738b7d7d6fd38", "skill-creator": "aab93935325978cb5a54b4907ae1ca11d7b4a903925ed0cc1739345392280db9"}
var geminiCycleBuiltinDescription = map[string]string{
	"antigravity-support": "Use when the user asks questions, seeks help, or requests instructions related to installing, setting up, or migrating to Antigravity CLI. This skill provides the latest up to date details, requirements, and commands sourced from the official Antigravity CLI documentation.",
	"skill-creator":       "Guide for creating effective skills. This skill should be used when users want to create a new skill (or update an existing skill) that extends Gemini CLI's capabilities with specialized knowledge, workflows, or tool integrations.",
}

func geminiCycleSkills(raw []byte, want map[string]geminiCycleSkillRow) error {
	if len(raw) > 1<<20 {
		return fmt.Errorf("Gemini skill catalog bound")
	}
	text := geminiCycleSGR.ReplaceAllString(string(raw), "")
	if strings.ContainsAny(text, "\x00\x1b") {
		return fmt.Errorf("Gemini skill catalog control")
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := []string{}
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 || lines[0] != "Discovered Agent Skills:" || (len(lines)-1)%3 != 0 || (len(lines)-1)/3 > 256 {
		return fmt.Errorf("Gemini exact skill catalog framing")
	}
	got := map[string]geminiCycleSkillRow{}
	for index := 1; index < len(lines); index += 3 {
		match := geminiCycleSkillHeader.FindStringSubmatch(lines[index])
		if match == nil || !strings.HasPrefix(lines[index+1], "  Description: ") || !strings.HasPrefix(lines[index+2], "  Location:    ") {
			return fmt.Errorf("Gemini exact skill catalog row")
		}
		if _, exists := got[match[1]]; exists {
			return fmt.Errorf("Gemini duplicate skill row")
		}
		got[match[1]] = geminiCycleSkillRow{Description: strings.TrimPrefix(lines[index+1], "  Description: "), Path: strings.TrimPrefix(lines[index+2], "  Location:    "), Enabled: match[2] == "Enabled", Builtin: match[3] != ""}
	}
	if !reflect.DeepEqual(got, want) {
		return fmt.Errorf("Gemini complete pinned/foreign/owned skill selection differs")
	}
	return nil
}
func (c *geminiProjectCycle) skills(raw []byte, scope string) error {
	want := map[string]geminiCycleSkillRow{}
	for name, sha := range geminiCycleBuiltinSHA {
		path := filepath.Join(filepath.Dir(c.f.client), "builtin", name, "SKILL.md")
		actual, err := projectNativeRuntimeDigest(path)
		if err != nil || actual != sha || c.f.immutable[path] != sha {
			return fmt.Errorf("Gemini pinned builtin whole-file identity differs")
		}
		want[name] = geminiCycleSkillRow{Description: geminiCycleBuiltinDescription[name], Path: path, Enabled: true, Builtin: true}
	}
	seen := map[string]bool{}
	for _, key := range []string{"user", scope} {
		if seen[key] {
			continue
		}
		seen[key] = true
		r := c.recipes[key]
		want["foreign-"+key] = geminiCycleSkillRow{Description: "Foreign inert marker", Path: filepath.Join(r.Profile, "skills", "foreign-"+key, "SKILL.md"), Enabled: false, Builtin: false}
		if r.Binding.ClientBindingID != "" && !r.Removed {
			if err := c.skillProof(r, true); err != nil {
				return err
			}
			want[r.Name] = geminiCycleSkillRow{Description: r.Revision, Path: r.SkillPath, Enabled: true, Builtin: false}
		}
	}
	return geminiCycleSkills(raw, want)
}
