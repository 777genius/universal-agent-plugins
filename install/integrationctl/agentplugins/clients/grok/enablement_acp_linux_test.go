//go:build linux

package grok

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type enablementDuplexFixture struct {
	frames, trailing string
	cleanupError     error
	requests         bytes.Buffer
	calls            int
}

func (*enablementDuplexFixture) Run(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
	return legacyports.CommandResult{}, errors.New("unexpected ordinary command")
}
func (*enablementDuplexFixture) DuplexCapability() error { return nil }
func (fixture *enablementDuplexFixture) RunReadonlyDuplexWithPlannedShutdown(_ context.Context, command legacyports.Command, inputs legacyports.ReadonlyDuplexInputs, exchange func(io.Writer, io.Reader) error) error {
	if err := validateEnablementFixtureInputs(command, inputs); err != nil {
		return err
	}
	fixture.calls++
	reader, writer, err := os.Pipe()
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close(); _ = writer.Close() }()
	if _, err := io.WriteString(writer, fixture.frames); err != nil {
		return err
	}
	err = exchange(&fixture.requests, reader)
	if err == nil && fixture.trailing != "" {
		_, err = io.WriteString(writer, fixture.trailing)
	}
	return errors.Join(err, fixture.cleanupError)
}

func TestEnablementACPConsumerRequiresWholePipeAndFamilySuccess(t *testing.T) {
	for _, fixture := range []*enablementDuplexFixture{
		{frames: enablementACPFrames(t, true)},
		{frames: enablementACPFrames(t, true), trailing: `{"jsonrpc":`},
		{frames: enablementACPFrames(t, true) + `{"jsonrpc":`},
		{frames: enablementACPFrames(t, true), cleanupError: errors.New("family remains unproved")},
	} {
		wantSuccess := fixture.trailing == "" && fixture.cleanupError == nil && fixture.frames == enablementACPFrames(t, true)
		inputs, _ := inventoryInputsFixture(t)
		fixture.frames = relocateEnablementFrames(t, fixture.frames, inputs.inputs.Project.Path)
		plugins, err := readEnablementInventory(t.Context(), fixture, inputs, inputs.profile.config.Original())
		if fixture.calls != 1 {
			t.Fatal("consumer did not reach the targeted pipe fixture", err, fixture.calls)
		}
		if (err == nil) != wantSuccess || wantSuccess && (len(plugins) != 1 || !plugins[0].enabled) {
			t.Fatalf("complete native consumer: %+v %v, want success %v", plugins, err, wantSuccess)
		}
	}
}

func validateEnablementFixtureInputs(command legacyports.Command, inputs legacyports.ReadonlyDuplexInputs) error {
	if !slices.Equal(command.Argv, []string{inputs.Executable.Path, "agent", "--no-leader", "stdio"}) ||
		command.Dir != inputs.Project.Path || inputs.Worktree.Path != inputs.Project.Path ||
		!slices.Equal(command.Env, grokMetadataEnvironment(inputs.Home.Path)) {
		return errors.New("inert readonly command and original roles differ")
	}
	for _, input := range []legacyports.ReadonlyOriginal{inputs.Home, inputs.Worktree, inputs.Project, inputs.Executable} {
		if input.File == nil {
			return errors.New("missing original fixture handle")
		}
		info, err := input.File.Stat()
		if err != nil {
			return err
		}
		if !inventoryInputSame(input.Info, info) {
			return errors.New("original fixture role changed")
		}
	}
	return nil
}

func relocateEnablementFrames(t *testing.T, frames, project string) string {
	t.Helper()
	oldRoot := "/project/.grok/plugins/physical"
	newRoot := filepath.Join(project, ".grok", "plugins", "physical")
	for old, next := range map[string]string{"/project": project, oldRoot: newRoot} {
		before, err := json.Marshal(old)
		if err != nil {
			t.Fatal(err)
		}
		after, err := json.Marshal(next)
		if err != nil {
			t.Fatal(err)
		}
		frames = strings.ReplaceAll(frames, string(before), string(after))
	}
	return strings.ReplaceAll(frames, enablementDigest([]byte(oldRoot))[7:15], enablementDigest([]byte(newRoot))[7:15])
}
