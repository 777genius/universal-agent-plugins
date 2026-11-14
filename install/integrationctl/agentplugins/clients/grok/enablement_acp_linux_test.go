//go:build linux

package grok

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"testing"

	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type enablementDuplexFixture struct {
	frames, trailing string
	cleanupError     error
	requests         bytes.Buffer
}

func (*enablementDuplexFixture) Run(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
	return legacyports.CommandResult{}, errors.New("unexpected ordinary command")
}
func (*enablementDuplexFixture) DuplexCapability() error { return nil }
func (fixture *enablementDuplexFixture) RunDuplexWithPlannedShutdown(_ context.Context, _ legacyports.Command, exchange func(io.Writer, io.Reader) error) error {
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
		plugins, err := readEnablementInventory(context.Background(), fixture, "/native/grok", "/project", "/home/user")
		wantSuccess := fixture.trailing == "" && fixture.cleanupError == nil && fixture.frames == enablementACPFrames(t, true)
		if (err == nil) != wantSuccess || wantSuccess && (len(plugins) != 1 || !plugins[0].enabled) {
			t.Fatalf("complete native consumer: %+v %v, want success %v", plugins, err, wantSuccess)
		}
	}
}
