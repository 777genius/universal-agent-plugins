package kiro

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"

	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// recordingRunner is the duplex fake the ACP tests embed. It used to live in
// providers because these tests did; the production runner stays there.
type recordingRunner struct {
	commands      []legacyports.Command
	run           func(legacyports.Command) legacyports.CommandResult
	duplexOutput  string
	duplexErr     error
	duplexPostErr error
	duplexLive    bool
	capabilityErr error
}

func (runner *recordingRunner) RunDuplexWithPlannedShutdown(_ context.Context, command legacyports.Command, exchange func(io.Writer, io.Reader) error) error {
	runner.commands = append(runner.commands, command)
	if runner.duplexErr != nil {
		return runner.duplexErr
	}
	output := io.Reader(strings.NewReader(runner.duplexOutput))
	var liveWriter *os.File
	var outputWritten chan struct{}
	if runner.duplexLive {
		liveReader, writer, err := os.Pipe()
		if err != nil {
			return err
		}
		defer func() { _ = liveReader.Close() }()
		liveWriter = writer
		output = liveReader
		outputWritten = make(chan struct{})
		go func() {
			_, _ = io.WriteString(writer, runner.duplexOutput)
			close(outputWritten)
		}()
	}
	stdin := &fixtureACPStdin{close: func() error {
		if liveWriter != nil {
			<-outputWritten
		}
		return nil
	}}
	exchangeErr := exchange(stdin, output)
	_ = stdin.Close()
	if liveWriter != nil {
		_ = liveWriter.Close()
	}
	if runner.duplexPostErr != nil {
		return errors.Join(exchangeErr, runner.duplexPostErr)
	}
	return exchangeErr
}

func (runner *recordingRunner) DuplexCapability() error { return runner.capabilityErr }

func (runner *recordingRunner) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	runner.commands = append(runner.commands, command)
	if runner.run != nil {
		return runner.run(command), nil
	}
	return legacyports.CommandResult{}, nil
}
