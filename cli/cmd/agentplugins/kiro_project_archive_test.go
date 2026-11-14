package main

import (
	"archive/zip"
	"bytes"
	"io"
	"path/filepath"
	"testing"
)

func TestKiroUserArchivePreservesAuthoredSkillWithAutocrlf(t *testing.T) {
	gitHome := projectJourneyTemp(t)
	for _, name := range []string{"HOME", "USERPROFILE", "XDG_CONFIG_HOME"} {
		t.Setenv(name, gitHome)
	}
	transport := &projectJourneyTransport{repo: projectJourneyTemp(t), remote: filepath.Join(projectJourneyTemp(t), "remote.git")}
	projectJourneyGit(t, transport.repo, "init", "-b", "main")
	projectJourneyGit(t, transport.repo, "init", "--bare", transport.remote)
	projectJourneyGit(t, transport.repo, "config", "core.autocrlf", "true")
	want := kiroUserCommitSource(t, transport, "exact authored LF bytes")
	archive, err := zip.NewReader(bytes.NewReader(transport.archive), int64(len(transport.archive)))
	if err != nil {
		t.Fatal(err)
	}
	skill, err := archive.Open("provider-wrapper/skills/uap-user-docs/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(skill)
	closeErr := skill.Close()
	if readErr != nil || closeErr != nil {
		t.Fatal(readErr, closeErr)
	}
	if string(body) != want {
		t.Fatal("Kiro User archive changed the exact authored Skill bytes")
	}
}
