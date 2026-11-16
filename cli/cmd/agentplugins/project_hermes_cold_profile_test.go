package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
)

type hermesColdProfileNode struct {
	info   os.FileInfo
	digest [sha256.Size]byte
}

type hermesColdProfile struct {
	nodes  map[string]hermesColdProfileNode
	config []byte
}

func readHermesColdProfile(root string) (hermesColdProfile, error) {
	profile := hermesColdProfile{nodes: map[string]hermesColdProfileNode{}}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode().Type() != os.ModeDir && !info.Mode().IsRegular() {
			return fmt.Errorf("selected profile contains an aliased or special node: %q", path)
		}
		var body []byte
		if info.Mode().IsRegular() {
			body, err = atomicfile.ReadBoundedRegularFile(path, info.Size())
			if err != nil {
				return err
			}
		}
		named, err := os.Lstat(path)
		if err != nil || !os.SameFile(info, named) || info.Mode() != named.Mode() {
			return fmt.Errorf("selected profile changed during capture: %q", path)
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		profile.nodes[relative] = hermesColdProfileNode{info: info, digest: sha256.Sum256(body)}
		if relative == "config.yaml" {
			profile.config = body
		}
		return nil
	})
	if err != nil {
		return hermesColdProfile{}, err
	}
	if node, found := profile.nodes["config.yaml"]; !found || !node.info.Mode().IsRegular() {
		return hermesColdProfile{}, fmt.Errorf("selected profile lacks its original regular configuration")
	}
	return profile, nil
}

func (profile hermesColdProfile) tree() map[string]string {
	tree := make(map[string]string, len(profile.nodes))
	for name, node := range profile.nodes {
		tree[name] = fmt.Sprintf("%s/%x", node.info.Mode(), node.digest)
	}
	return tree
}

func (input *hermesColdInputs) rememberProfile(t *testing.T, root string) {
	t.Helper()
	if _, found := input.protected[root]; found {
		t.Fatal("cannot renew an original selected profile")
	}
	profile, err := readHermesColdProfile(root)
	if err != nil {
		t.Fatal(err)
	}
	if input.dataIdentities == nil {
		input.dataIdentities = map[string]os.FileInfo{}
	}
	for relative, node := range profile.nodes {
		if relative == "config.yaml" {
			continue // Native trust may atomically replace this one file.
		}
		path := filepath.Join(root, relative)
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := file.Close(); err != nil {
				t.Error(err)
			}
		})
		held, err := file.Stat()
		named, statErr := os.Lstat(path)
		if err != nil || statErr != nil || named.Mode().Type() != os.ModeDir && !named.Mode().IsRegular() ||
			!os.SameFile(held, named) || !os.SameFile(node.info, held) || node.info.Mode() != held.Mode() {
			t.Fatal("selected profile changed during held capture", err, statErr)
		}
		node.info = held
		profile.nodes[relative] = node
		input.dataIdentities[path] = held
	}
	if input.trustProfiles == nil {
		input.trustProfiles = map[string]*hermesColdProfile{}
	}
	input.trustProfiles[root] = &profile
	input.protected[root] = profile.tree()
}

// Only the first confirmed add may adopt native initialization outside retained
// subtrees. Every original node is checked first; none of the other authorities
// or their snapshots is renewed. Held identities remain checked on later steps.
func (input *hermesColdInputs) adoptProjectTrust(root, project string) error {
	before := input.trustProfiles[root]
	if before == nil {
		return fmt.Errorf("selected profile lacks an unused original trust snapshot")
	}
	after, err := readHermesColdProfile(root)
	if err != nil {
		return err
	}
	if err := before.validate(after, project); err != nil {
		return err
	}
	input.protected[root] = after.tree()
	delete(input.trustProfiles, root)
	return nil
}

func (profile hermesColdProfile) validate(after hermesColdProfile, project string) error {
	for name, original := range profile.nodes {
		current, found := after.nodes[name]
		if !found || original.info.Mode() != current.info.Mode() {
			return fmt.Errorf("retained selected-profile node was removed or changed type/mode: %q", name)
		}
		if name != "config.yaml" && (!os.SameFile(original.info, current.info) || original.digest != current.digest) {
			return fmt.Errorf("retained selected-profile node changed identity or content: %q", name)
		}
	}
	foreign := filepath.Join("skills", "foreign")
	for name := range after.nodes {
		if name == foreign || strings.HasPrefix(name, foreign+string(filepath.Separator)) {
			if _, found := profile.nodes[name]; !found {
				return fmt.Errorf("native initialization entered the retained foreign Skill subtree: %q", name)
			}
		}
	}
	return hermesColdTrustConfiguration(profile.config, after.config, project)
}

func hermesColdTrustConfiguration(before, after []byte, project string) error {
	expected, err := hermesColdConfiguration(before)
	if err != nil {
		return err
	}
	current, err := hermesColdConfiguration(after)
	if err != nil {
		return err
	}
	skills, ok := current["skills"].(map[string]any)
	if !ok {
		return fmt.Errorf("selected profile lacks its recorded project trust")
	}
	trusted, ok := skills["trusted_project_dirs"].([]any)
	if !ok || len(trusted) != 1 {
		return fmt.Errorf("selected profile must record exactly one trusted project root")
	}
	path, ok := trusted[0].(string)
	if !ok || !filepath.IsAbs(path) {
		return fmt.Errorf("selected profile trust is not an absolute directory")
	}
	selected, err := os.Stat(project)
	recorded, statErr := os.Stat(path)
	if err != nil || statErr != nil || !selected.IsDir() || !recorded.IsDir() || !os.SameFile(selected, recorded) {
		return fmt.Errorf("selected profile trusted a different physical project root")
	}
	retained := map[string]any{}
	if value, found := expected["skills"]; found {
		retained, ok = value.(map[string]any)
		if !ok {
			return fmt.Errorf("original skills settings are not a mapping")
		}
	}
	if value, found := retained["trusted_project_dirs"]; found {
		prior, isList := value.([]any)
		if !isList || len(prior) != 0 {
			return fmt.Errorf("original profile already has a different trust authority")
		}
	}
	retained["trusted_project_dirs"] = []any{path}
	expected["skills"] = retained
	if !reflect.DeepEqual(expected, current) {
		return fmt.Errorf("project trust changed settings beyond the authorized trust list")
	}
	return nil
}

func hermesColdConfiguration(body []byte) (map[string]any, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(body))
	var config map[string]any
	if err := decoder.Decode(&config); err != nil || config == nil {
		return nil, fmt.Errorf("selected profile configuration is not one valid YAML mapping")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("selected profile configuration has trailing YAML")
	}
	return config, nil
}
