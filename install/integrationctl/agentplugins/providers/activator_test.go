package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type recordingRunner struct {
	commands      []legacyports.Command
	run           func(legacyports.Command) legacyports.CommandResult
	duplexOutput  string
	duplexErr     error
	duplexPostErr error
	duplexLive    bool
	capabilityErr error
}

type runOnlyRecordingRunner struct {
	commands []legacyports.Command
}

func (runner *runOnlyRecordingRunner) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	runner.commands = append(runner.commands, command)
	return legacyports.CommandResult{}, nil
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
		defer liveReader.Close()
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
	if strings.HasSuffix(strings.Join(command.Argv, " "), "plugin list") {
		for index := len(runner.commands) - 2; index >= 0; index-- {
			argv := runner.commands[index].Argv
			if len(argv) >= 4 && argv[1] == "plugin" && argv[2] == "install" {
				return legacyports.CommandResult{Stdout: []byte("Installed plugins:\n  • " + argv[3] + " (v1.0.0)")}, nil
			}
		}
	}
	return legacyports.CommandResult{}, nil
}

func TestActivatorInstallsCopilotAndVSCodeThroughManagedMarketplace(t *testing.T) {
	t.Parallel()
	for _, client := range []domain.ClientID{domain.ClientCopilot, domain.ClientVSCode} {
		client := client
		t.Run(string(client), func(t *testing.T) {
			runner := &recordingRunner{}
			request := activationRequest(t, client)
			request.BackendExecutable = "/test/bin/copilot"
			outcome, err := (Activator{Runner: runner}).Activate(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			if outcome.Activation != domain.ActivationActive || outcome.Verification != domain.VerificationInstalled || len(outcome.UserActions) != 0 {
				t.Fatalf("outcome = %+v", outcome)
			}
			marketplace := managedMarketplaceName(request.Plan.PhysicalArtifactID)
			want := [][]string{
				{"/test/bin/copilot", "plugin", "marketplace", "add", request.Delivery.ActivePath},
				{"/test/bin/copilot", "plugin", "install", "demo@" + marketplace},
				{"/test/bin/copilot", "plugin", "list"},
			}
			if got := commandArgv(runner.commands); !reflect.DeepEqual(got, want) {
				t.Fatalf("commands = %#v, want %#v", got, want)
			}
		})
	}
}

func TestActivatorUpdateRecoversMissingMarketplaceAndPlugin(t *testing.T) {
	t.Parallel()
	runner := &recordingRunner{run: func(command legacyports.Command) legacyports.CommandResult {
		joined := strings.Join(command.Argv, " ")
		if strings.Contains(joined, "marketplace update") || strings.Contains(joined, "plugin update") {
			return legacyports.CommandResult{ExitCode: 1, Stderr: []byte("missing")}
		}
		if strings.HasSuffix(joined, "plugin list") {
			return legacyports.CommandResult{Stdout: []byte("Installed plugins:\n  • demo@agentplugins-8f97b00da374 (v1.0.0)")}
		}
		return legacyports.CommandResult{}
	}}
	request := activationRequest(t, domain.ClientCopilot)
	request.BackendExecutable = "/test/bin/copilot"
	request.Replacing = true
	outcome, err := (Activator{Runner: runner}).Activate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Activation != domain.ActivationActive {
		t.Fatalf("outcome = %+v", outcome)
	}
	marketplace := managedMarketplaceName(request.Plan.PhysicalArtifactID)
	want := [][]string{
		{"/test/bin/copilot", "plugin", "marketplace", "update", marketplace},
		{"/test/bin/copilot", "plugin", "marketplace", "add", request.Delivery.ActivePath},
		{"/test/bin/copilot", "plugin", "update", "demo@" + marketplace},
		{"/test/bin/copilot", "plugin", "install", "demo@" + marketplace},
		{"/test/bin/copilot", "plugin", "list"},
	}
	if got := commandArgv(runner.commands); !reflect.DeepEqual(got, want) {
		t.Fatalf("commands = %#v, want %#v", got, want)
	}
}

func TestActivatorFallsBackToManualForUnknownCopilotListing(t *testing.T) {
	t.Parallel()
	runner := &recordingRunner{run: func(legacyports.Command) legacyports.CommandResult { return legacyports.CommandResult{} }}
	request := activationRequest(t, domain.ClientCopilot)
	request.BackendExecutable = "/test/bin/copilot"
	outcome, err := (Activator{Runner: runner}).Activate(context.Background(), request)
	if err != nil {
		t.Fatalf("verification error = %v", err)
	}
	if outcome.Activation != domain.ActivationManual || outcome.Verification != domain.VerificationPackageValid || len(outcome.LocalActions) != 1 {
		t.Fatalf("outcome = %+v", outcome)
	}
}

func TestActivatorUsesCodexCLIAndVerifiesJSONState(t *testing.T) {
	t.Parallel()
	runner := &recordingRunner{run: func(command legacyports.Command) legacyports.CommandResult {
		if strings.Contains(strings.Join(command.Argv, " "), "plugin list --json") {
			return legacyports.CommandResult{Stdout: []byte(`{"installed":[{"pluginId":"demo@agentplugins-8f97b00da374","name":"demo","marketplaceName":"agentplugins-8f97b00da374","installed":true,"enabled":true}],"available":[]}`)}
		}
		return legacyports.CommandResult{}
	}}
	request := activationRequest(t, domain.ClientCodex)
	request.BackendExecutable = "/test/bin/codex"
	outcome, err := (Activator{Runner: runner}).Activate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Activation != domain.ActivationActive || outcome.Verification != domain.VerificationInstalled {
		t.Fatalf("outcome = %+v", outcome)
	}
	marketplace := managedMarketplaceName(request.Plan.PhysicalArtifactID)
	want := [][]string{
		{"/test/bin/codex", "plugin", "marketplace", "add", request.Delivery.ActivePath, "--json"},
		{"/test/bin/codex", "plugin", "add", "demo@" + marketplace, "--json"},
		{"/test/bin/codex", "plugin", "list", "--json"},
	}
	if got := commandArgv(runner.commands); !reflect.DeepEqual(got, want) {
		t.Fatalf("commands = %#v, want %#v", got, want)
	}
}

func TestCodexVerificationRequiresExactEnabledManagedEntry(t *testing.T) {
	t.Parallel()
	marketplace := "managed"
	valid := func(pluginID, name, market string, installed, enabled bool) string {
		return fmt.Sprintf(`{"pluginId":%q,"name":%q,"marketplaceName":%q,"installed":%t,"enabled":%t}`, pluginID, name, market, installed, enabled)
	}
	cases := map[string]struct {
		body string
		want codexStatus
	}{
		"installed":               {`{"metadata":{"version":1},"installed":[` + valid("demo@managed", "demo", marketplace, true, true) + `]}`, codexStatusInstalled},
		"empty":                   {`{"installed":[]}`, codexStatusAbsent},
		"valid unrelated":         {`{"installed":[` + valid("other@managed", "other", marketplace, true, true) + `]}`, codexStatusAbsent},
		"expected disabled":       {`{"installed":[` + valid("demo@managed", "demo", marketplace, true, false) + `]}`, codexStatusAbsent},
		"expected not installed":  {`{"installed":[` + valid("demo@managed", "demo", marketplace, false, true) + `]}`, codexStatusAbsent},
		"malformed":               {`{`, codexStatusUnknown},
		"old shape":               {`{"plugins":[{"name":"demo"}]}`, codexStatusUnknown},
		"null array":              {`{"installed":null}`, codexStatusUnknown},
		"arbitrary entry":         {`{"installed":[{"message":"not authenticated"}]}`, codexStatusUnknown},
		"wrong field type":        {`{"installed":[{"pluginId":7,"name":"demo","marketplaceName":"managed","installed":true,"enabled":true}]}`, codexStatusUnknown},
		"wrong name type":         {`{"installed":[{"pluginId":"demo@managed","name":7,"marketplaceName":"managed","installed":true,"enabled":true}]}`, codexStatusUnknown},
		"wrong marketplace type":  {`{"installed":[{"pluginId":"demo@managed","name":"demo","marketplaceName":7,"installed":true,"enabled":true}]}`, codexStatusUnknown},
		"wrong installed type":    {`{"installed":[{"pluginId":"demo@managed","name":"demo","marketplaceName":"managed","installed":"true","enabled":true}]}`, codexStatusUnknown},
		"wrong enabled type":      {`{"installed":[{"pluginId":"demo@managed","name":"demo","marketplaceName":"managed","installed":true,"enabled":1}]}`, codexStatusUnknown},
		"missing plugin id":       {`{"installed":[{"name":"demo","marketplaceName":"managed","installed":true,"enabled":true}]}`, codexStatusUnknown},
		"missing enabled":         {`{"installed":[{"pluginId":"demo@managed","name":"demo","marketplaceName":"managed","installed":true}]}`, codexStatusUnknown},
		"additive entry fields":   {`{"installed":[{"pluginId":"demo@managed","name":"demo","marketplaceName":"managed","installed":true,"enabled":true,"source":{"source":"local"},"installPolicy":"AVAILABLE"}]}`, codexStatusInstalled},
		"enabled case variant":    {`{"installed":[{"pluginId":"demo@managed","name":"demo","marketplaceName":"managed","installed":true,"Enabled":true}]}`, codexStatusUnknown},
		"duplicate enabled":       {`{"installed":[{"pluginId":"demo@managed","name":"demo","marketplaceName":"managed","installed":true,"enabled":true,"enabled":false}]}`, codexStatusUnknown},
		"duplicate installed":     {`{"installed":[{"pluginId":"demo@managed","name":"demo","marketplaceName":"managed","installed":true,"installed":false,"enabled":true}]}`, codexStatusUnknown},
		"inconsistent identity":   {`{"installed":[` + valid("demo@other", "demo", marketplace, true, true) + `]}`, codexStatusUnknown},
		"duplicate identity":      {`{"installed":[` + valid("demo@managed", "demo", marketplace, true, true) + `,` + valid("demo@managed", "demo", marketplace, false, false) + `]}`, codexStatusUnknown},
		"duplicate top installed": {`{"installed":[],"installed":[` + valid("demo@managed", "demo", marketplace, true, true) + `]}`, codexStatusUnknown},
		"top installed variant":   {`{"Installed":[` + valid("demo@managed", "demo", marketplace, true, true) + `]}`, codexStatusUnknown},
		"ambiguous top installed": {`{"installed":[],"Installed":[]}`, codexStatusUnknown},
	}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			if got := codexPluginStatus([]byte(test.body), "demo", marketplace); got != test.want {
				t.Fatalf("status = %v, want %v", got, test.want)
			}
		})
	}
}

func TestKiroVerificationScansUnknownBeforeRecognizedNegative(t *testing.T) {
	t.Parallel()
	for _, activationComplete := range []bool{false, true} {
		t.Run(fmt.Sprintf("activation_complete_%t", activationComplete), func(t *testing.T) {
			request := activationRequest(t, domain.ClientKiro)
			request.BackendExecutable = "/test/bin/kiro-cli"
			request.VerifyOnly = true
			request.ActivationComplete = activationComplete
			request.Plan.Components = []domain.ComponentDecision{
				{Kind: domain.ComponentMCPServer, Name: "unknown-first", Support: domain.SupportNative},
				{Kind: domain.ComponentMCPServer, Name: "negative-later", Support: domain.SupportNative},
			}
			runner := &recordingRunner{duplexOutput: acpResponse(0, `{"protocolVersion":1}`) + acpResponse(1, `{"sessionId":"s"}`) + acpStatus("s", "negative-later", "disconnected", "")}
			outcome, err := (Activator{Runner: runner}).Activate(context.Background(), request)
			if err == nil || outcome.Activation != domain.ActivationFailed || !outcome.AuthoritativeObservation {
				t.Fatalf("outcome=%+v err=%v", outcome, err)
			}
			if outcome.ActivationAttested {
				t.Fatalf("recognized negative evidence was bypassed by attestation: %+v", outcome)
			}
			if len(runner.commands) != 1 {
				t.Fatalf("verifier calls = %d, want one ACP session", len(runner.commands))
			}
		})
	}
}

func TestKiroVerificationFailsWhenAnyPlannedServerIsMissing(t *testing.T) {
	t.Parallel()
	request := activationRequest(t, domain.ClientKiro)
	request.BackendExecutable = "/test/bin/kiro-cli"
	request.VerifyOnly = true
	request.Plan.Components = []domain.ComponentDecision{
		{Kind: domain.ComponentMCPServer, Name: "unknown-first", Support: domain.SupportNative},
		{Kind: domain.ComponentMCPServer, Name: "healthy-later", Support: domain.SupportNative},
	}
	runner := &recordingRunner{duplexOutput: acpResponse(0, `{"protocolVersion":1}`) + acpResponse(1, `{"sessionId":"s"}`) + acpStatus("s", "healthy-later", "connected", `[{"name":"search","disabled":false}]`)}
	outcome, err := (Activator{Runner: runner}).Activate(context.Background(), request)
	if err == nil || outcome.Activation != domain.ActivationFailed || !outcome.AuthoritativeObservation {
		t.Fatalf("outcome=%+v err=%v", outcome, err)
	}
	if len(runner.commands) != 1 {
		t.Fatalf("verifier calls = %d, want one ACP session", len(runner.commands))
	}
}

func TestKiroMultiServerTimeoutCannotFallBackToActivationAttestation(t *testing.T) {
	t.Parallel()
	request := activationRequest(t, domain.ClientKiro)
	request.BackendExecutable = "/test/bin/kiro-cli"
	request.VerifyOnly = true
	request.ActivationComplete = true
	request.Plan.Components = []domain.ComponentDecision{
		{Kind: domain.ComponentMCPServer, Name: "alpha", Support: domain.SupportNative},
		{Kind: domain.ComponentMCPServer, Name: "beta", Support: domain.SupportNative},
	}
	runner := &recordingRunner{
		duplexOutput: acpResponse(0, `{"protocolVersion":1}`) + acpResponse(1, `{"sessionId":"s"}`) +
			acpStatus("s", "alpha", "connected", `[{"name":"search","disabled":false}]`),
		duplexPostErr: context.DeadlineExceeded,
	}
	outcome, err := (Activator{Runner: runner}).Activate(context.Background(), request)
	if err == nil || outcome.Activation != domain.ActivationFailed || !outcome.AuthoritativeObservation {
		t.Fatalf("outcome=%+v err=%v, want authoritative incomplete verification failure", outcome, err)
	}
	if outcome.ActivationAttested || errors.Is(err, errKiroACPContractUnknown) {
		t.Fatalf("timeout downgraded negative evidence to attestation/manual fallback: outcome=%+v err=%v", outcome, err)
	}
}

func TestKiroEOFDuringSettlementFailsClosed(t *testing.T) {
	t.Parallel()
	request := activationRequest(t, domain.ClientKiro)
	request.BackendExecutable = "/test/bin/kiro-cli"
	request.VerifyOnly = true
	request.ActivationComplete = true
	request.Plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "alpha", Support: domain.SupportNative}}
	runner := &recordingRunner{duplexOutput: connectedACP("alpha")}
	outcome, err := (Activator{Runner: runner}).Activate(context.Background(), request)
	if err == nil || outcome.Activation != domain.ActivationFailed {
		t.Fatalf("outcome=%+v err=%v, want settlement EOF failure", outcome, err)
	}
	if outcome.ActivationAttested {
		t.Fatalf("settlement EOF was converted to attested active: %+v", outcome)
	}
}

func TestKiroPartialRecordAfterCompletionCannotBeAttestedActive(t *testing.T) {
	t.Parallel()
	request := activationRequest(t, domain.ClientKiro)
	request.BackendExecutable = "/test/bin/kiro-cli"
	request.VerifyOnly = true
	request.ActivationComplete = true
	request.Plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "alpha", Support: domain.SupportNative}}
	runner := &recordingRunner{duplexOutput: connectedACP("alpha") + `{"jsonrpc":"2.0"`}
	outcome, err := (Activator{Runner: runner}).Activate(context.Background(), request)
	if err == nil || !errors.Is(err, errKiroACPPartialExit) || !errors.Is(err, errRecognizedNegativeEvidence) || outcome.ActivationAttested {
		t.Fatalf("partial EOF outcome=%+v err=%v, want non-attestable negative evidence", outcome, err)
	}
}

func TestCodexProviderDistinguishesNegativeAndUnknownListings(t *testing.T) {
	t.Parallel()
	request := activationRequest(t, domain.ClientCodex)
	request.BackendExecutable = "/test/bin/codex"
	request.VerifyOnly = true
	for name, body := range map[string]string{
		"negative": `{"installed":[]}`,
		"unknown":  `{"installed":[{"name":"demo"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			runner := &recordingRunner{run: func(legacyports.Command) legacyports.CommandResult {
				return legacyports.CommandResult{Stdout: []byte(body)}
			}}
			outcome, err := (Activator{Runner: runner}).Activate(context.Background(), request)
			if name == "negative" {
				if err == nil || outcome.Activation != domain.ActivationFailed || !outcome.AuthoritativeObservation {
					t.Fatalf("outcome=%+v err=%v", outcome, err)
				}
				return
			}
			if err != nil || outcome.Activation != domain.ActivationManual || outcome.AuthoritativeObservation {
				t.Fatalf("outcome=%+v err=%v", outcome, err)
			}
		})
	}
}

func TestCopilotVerificationRejectsRecognizedNegativeEvidence(t *testing.T) {
	t.Parallel()
	request := activationRequest(t, domain.ClientCopilot)
	request.BackendExecutable = "/test/bin/copilot"
	request.VerifyOnly = true
	spec := "demo@" + managedMarketplaceName(request.Plan.PhysicalArtifactID)
	cases := map[string]legacyports.CommandResult{
		"pending":          {Stdout: []byte("Installed plugins:\n  • " + spec + " pending")},
		"disconnected":     {Stdout: []byte("Installed plugins:\n  • " + spec + " disconnected")},
		"disabled":         {Stdout: []byte("Installed plugins:\n  • " + spec + " disabled")},
		"auth required":    {Stdout: []byte("Installed plugins:\n  • " + spec + " auth-required")},
		"error":            {Stdout: []byte("Installed plugins:\n  error: " + spec)},
		"unrelated":        {Stdout: []byte("Installed plugins:\n  • demo-extra@" + managedMarketplaceName(request.Plan.PhysicalArtifactID) + " (v1.0.0)")},
		"explicitly empty": {Stdout: []byte("Installed plugins:\n  No plugins installed.")},
	}
	for name, listed := range cases {
		t.Run(name, func(t *testing.T) {
			runner := &recordingRunner{run: func(legacyports.Command) legacyports.CommandResult { return listed }}
			if _, err := (Activator{Runner: runner}).Activate(context.Background(), request); err == nil {
				t.Fatalf("untrusted Copilot listing was accepted: %+v", listed)
			}
		})
	}
}

func TestCopilotUnknownOutputContractRemainsManual(t *testing.T) {
	t.Parallel()
	request := activationRequest(t, domain.ClientCopilot)
	request.BackendExecutable = "/test/bin/copilot"
	request.VerifyOnly = true
	spec := "demo@" + managedMarketplaceName(request.Plan.PhysicalArtifactID)
	for name, listed := range map[string]legacyports.CommandResult{
		"empty":           {},
		"bare name":       {Stdout: []byte("Installed plugins:\n  • demo (v1.0.0)")},
		"stderr only":     {Stderr: []byte("Installed plugins:\n  • " + spec + " (v1.0.0)")},
		"outside section": {Stdout: []byte("• " + spec + " (v1.0.0)")},
		"bad version":     {Stdout: []byte("Installed plugins:\n  • " + spec + " (latest)")},
		"wrong version":   {Stdout: []byte("Installed plugins:\n  • " + spec + " (v1.0.1)")},
		"duplicate":       {Stdout: []byte("Installed plugins:\n  • " + spec + " (v1.0.0)\n  • " + spec + " (v1.0.0)")},
	} {
		t.Run(name, func(t *testing.T) {
			runner := &recordingRunner{run: func(legacyports.Command) legacyports.CommandResult { return listed }}
			outcome, err := (Activator{Runner: runner}).Activate(context.Background(), request)
			if err != nil || outcome.Activation != domain.ActivationManual || outcome.Verification != domain.VerificationPackageValid {
				t.Fatalf("outcome=%+v err=%v", outcome, err)
			}
			if outcome.ActivationAttested || len(runner.commands) != 1 || len(outcome.LocalActions) == 0 {
				t.Fatalf("unknown output path is not actionable: outcome=%+v commands=%+v", outcome, runner.commands)
			}
		})
	}
}

func TestCopilotExactVersion1078OutputRemainsSupported(t *testing.T) {
	t.Parallel()
	request := activationRequest(t, domain.ClientCopilot)
	request.Plan.DeclaredVersion = "1.0.78"
	request.BackendExecutable = "/test/bin/copilot"
	request.VerifyOnly = true
	spec := "demo@" + managedMarketplaceName(request.Plan.PhysicalArtifactID)
	runner := &recordingRunner{run: func(legacyports.Command) legacyports.CommandResult {
		return legacyports.CommandResult{Stdout: []byte("Installed plugins:\n  • " + spec + " (v1.0.78)")}
	}}
	outcome, err := (Activator{Runner: runner}).Activate(context.Background(), request)
	if err != nil || outcome.Activation != domain.ActivationActive {
		t.Fatalf("outcome=%+v err=%v", outcome, err)
	}
}

func TestCopilotLivePluginListingBindsEnabledIdentityAndManagedPath(t *testing.T) {
	t.Parallel()
	request := activationRequest(t, domain.ClientCopilot)
	request.Plan.DeclaredVersion = "1.7.0-uap.1"
	request.BackendExecutable = "/test/bin/copilot"
	request.VerifyOnly = true
	spec := "demo@" + managedMarketplaceName(request.Plan.PhysicalArtifactID)
	live := func(version, status, path string) []byte {
		return []byte(copilotLiveHeader + "\n  • " + spec + " (v" + version + ") (" + status + ")\n      from " + path + "\n")
	}

	runner := &recordingRunner{run: func(legacyports.Command) legacyports.CommandResult {
		return legacyports.CommandResult{Stdout: live("1.7.0-uap.1", "enabled", request.Delivery.ActivePath)}
	}}
	outcome, err := (Activator{Runner: runner}).Activate(context.Background(), request)
	if err != nil || outcome.Activation != domain.ActivationActive || outcome.Verification != domain.VerificationInstalled {
		t.Fatalf("live outcome=%+v err=%v", outcome, err)
	}

	for name, body := range map[string][]byte{
		"disabled":      live("1.7.0-uap.1", "disabled", request.Delivery.ActivePath),
		"wrong version": live("1.7.0-uap.0", "enabled", request.Delivery.ActivePath),
		"wrong path":    live("1.7.0-uap.1", "enabled", filepath.Join(t.TempDir(), "other")),
		"dot path":      live("1.7.0-uap.1", "enabled", filepath.Dir(request.Delivery.ActivePath)+string(filepath.Separator)+"alias"+string(filepath.Separator)+".."+string(filepath.Separator)+filepath.Base(request.Delivery.ActivePath)),
		"missing path":  []byte(copilotLiveHeader + "\n  • " + spec + " (v1.7.0-uap.1) (enabled)\n"),
		"extra suffix":  append(live("1.7.0-uap.1", "enabled", request.Delivery.ActivePath), []byte("unexpected\n")...),
		"ansi prefix":   append([]byte("\x1b[32m"), live("1.7.0-uap.1", "enabled", request.Delivery.ActivePath)...),
	} {
		t.Run(name, func(t *testing.T) {
			runner := &recordingRunner{run: func(legacyports.Command) legacyports.CommandResult {
				return legacyports.CommandResult{Stdout: body}
			}}
			outcome, err := (Activator{Runner: runner}).Activate(context.Background(), request)
			if err != nil || outcome.Activation != domain.ActivationManual || outcome.AuthoritativeObservation {
				t.Fatalf("unknown outcome=%+v err=%v", outcome, err)
			}
		})
	}
}

func TestActivationAttestationCannotBypassObservableVerifier(t *testing.T) {
	t.Parallel()
	for _, client := range []domain.ClientID{domain.ClientCodex, domain.ClientCopilot, domain.ClientVSCode, domain.ClientKiro} {
		t.Run(string(client), func(t *testing.T) {
			runner := &recordingRunner{run: func(command legacyports.Command) legacyports.CommandResult {
				switch client {
				case domain.ClientCopilot, domain.ClientVSCode:
					return legacyports.CommandResult{Stdout: []byte("Installed plugins:\n  No plugins installed.")}
				case domain.ClientKiro:
					return legacyports.CommandResult{Stdout: []byte("demo: disconnected")}
				default:
					return legacyports.CommandResult{Stdout: []byte(`{"installed":[]}`)}
				}
			}}
			request := activationRequest(t, client)
			request.ActivationComplete = true
			request.VerifyOnly = true
			request.BackendExecutable = "/test/bin/" + string(client)
			if client == domain.ClientVSCode {
				request.BackendExecutable = "/test/bin/copilot"
			}
			if client == domain.ClientKiro {
				request.BackendExecutable = "/test/bin/kiro-cli"
				request.Plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "demo", Support: domain.SupportNative}}
				runner.duplexOutput = acpResponse(0, `{"protocolVersion":1}`) + acpResponse(1, `{"sessionId":"s"}`) + acpStatus("s", "demo", "disconnected", "")
			}
			outcome, err := (Activator{Runner: runner}).Activate(context.Background(), request)
			if err == nil || outcome.Activation != domain.ActivationFailed || outcome.Verification != domain.VerificationFailed || !outcome.AuthoritativeObservation {
				t.Fatalf("recognized negative evidence did not fail closed: outcome=%+v err=%v", outcome, err)
			}
			if outcome.ActivationAttested {
				t.Fatalf("observable activation was marked attested: %+v", outcome)
			}
			if len(runner.commands) == 0 {
				t.Fatal("observable activation completion did not run the verifier")
			}
		})
	}
}

func TestKiroHumanStatusOutputNeverProvesActivation(t *testing.T) {
	t.Parallel()
	request := activationRequest(t, domain.ClientKiro)
	request.BackendExecutable = "/test/bin/kiro-cli"
	request.VerifyOnly = true
	request.Plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "demo-server", Support: domain.SupportNative}}
	for name, listed := range map[string]legacyports.CommandResult{
		"legacy connected": {Stdout: []byte("demo-server: connected")},
		"current table":    {Stdout: []byte("Name         Scope  Agent  Command  Timeout  Disabled  Env Vars\ndemo-server global -      http     60       false     -")},
	} {
		t.Run(name, func(t *testing.T) {
			runner := &recordingRunner{run: func(legacyports.Command) legacyports.CommandResult { return listed }, duplexErr: errors.New("ACP unsupported")}
			outcome, err := (Activator{Runner: runner}).Activate(context.Background(), request)
			if err != nil || outcome.Activation != domain.ActivationManual || len(runner.commands) != 1 || runner.commands[0].Argv[1] != "acp" {
				t.Fatalf("human Kiro status influenced activation: outcome=%+v err=%v commands=%v", outcome, err, runner.commands)
			}
		})
	}
}

func TestKiroUnknownStatusContractRemainsManual(t *testing.T) {
	t.Parallel()
	request := activationRequest(t, domain.ClientKiro)
	request.BackendExecutable = "/test/bin/kiro-cli"
	request.VerifyOnly = true
	request.Plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "demo-server", Support: domain.SupportNative}}
	for name, listed := range map[string]legacyports.CommandResult{
		"stderr only":              {Stderr: []byte("demo-server: connected")},
		"unrelated positive":       {Stdout: []byte("demo-server-other: connected")},
		"unrelated negative":       {Stdout: []byte("demo-server-other: disconnected")},
		"warning plus negative":    {Stdout: []byte("warning: stale cache\ndemo-server: disconnected")},
		"negative plus extra text": {Stdout: []byte("demo-server: disconnected\nretrying")},
		"unknown":                  {Stdout: []byte("demo-server: enabled")},
	} {
		t.Run(name, func(t *testing.T) {
			runner := &recordingRunner{run: func(legacyports.Command) legacyports.CommandResult { return listed }, duplexErr: errors.New("ACP output contract unknown")}
			outcome, err := (Activator{Runner: runner}).Activate(context.Background(), request)
			if err != nil || outcome.Activation != domain.ActivationManual || outcome.Verification != domain.VerificationPackageValid {
				t.Fatalf("outcome=%+v err=%v", outcome, err)
			}
			if outcome.ActivationAttested || len(runner.commands) != 1 || len(outcome.LocalActions) == 0 {
				t.Fatalf("unknown output path is not actionable: outcome=%+v commands=%+v", outcome, runner.commands)
			}
		})
	}
}

func TestUnknownObservableVerificationRequiresAndRecordsExplicitAttestation(t *testing.T) {
	t.Parallel()
	for _, client := range []domain.ClientID{domain.ClientCodex, domain.ClientCopilot, domain.ClientVSCode, domain.ClientKiro} {
		t.Run(string(client), func(t *testing.T) {
			request := activationRequest(t, client)
			request.VerifyOnly = true
			request.ActivationComplete = true
			request.BackendExecutable = "/test/bin/" + string(client)
			if client == domain.ClientVSCode {
				request.BackendExecutable = "/test/bin/copilot"
			}
			if client == domain.ClientKiro {
				request.BackendExecutable = "/test/bin/kiro-cli"
				request.Plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "demo", Support: domain.SupportNative}}
			}
			runner := &recordingRunner{run: func(legacyports.Command) legacyports.CommandResult { return legacyports.CommandResult{} }}
			if client == domain.ClientKiro {
				runner.duplexErr = errors.New("ACP unsupported")
			}
			outcome, err := (Activator{Runner: runner}).Activate(context.Background(), request)
			if err != nil || outcome.Activation != domain.ActivationActive || outcome.Verification != domain.VerificationInstalled || !outcome.ActivationAttested {
				t.Fatalf("outcome=%+v err=%v", outcome, err)
			}
			if len(runner.commands) != 1 {
				t.Fatalf("verifier calls = %d", len(runner.commands))
			}
		})
	}
}

func TestVerifyOnlyRunsOnlyReadOnlyClientCommands(t *testing.T) {
	t.Parallel()
	for _, client := range []domain.ClientID{domain.ClientCodex, domain.ClientCopilot, domain.ClientKiro} {
		client := client
		t.Run(string(client), func(t *testing.T) {
			request := activationRequest(t, client)
			request.VerifyOnly = true
			runner := &recordingRunner{}
			var want [][]string
			switch client {
			case domain.ClientCodex:
				request.BackendExecutable = "/test/bin/codex"
				marketplace := managedMarketplaceName(request.Plan.PhysicalArtifactID)
				runner.run = func(legacyports.Command) legacyports.CommandResult {
					return legacyports.CommandResult{Stdout: []byte(fmt.Sprintf(`{"installed":[{"pluginId":"demo@%s","name":"demo","marketplaceName":%q,"installed":true,"enabled":true}]}`, marketplace, marketplace))}
				}
				want = [][]string{{"/test/bin/codex", "plugin", "list", "--json"}}
			case domain.ClientCopilot:
				request.BackendExecutable = "/test/bin/copilot"
				spec := "demo@" + managedMarketplaceName(request.Plan.PhysicalArtifactID)
				runner.run = func(legacyports.Command) legacyports.CommandResult {
					return legacyports.CommandResult{Stdout: []byte("Installed plugins:\n  • " + spec + " (v1.0.0)")}
				}
				want = [][]string{{"/test/bin/copilot", "plugin", "list"}}
			case domain.ClientKiro:
				request.BackendExecutable = "/test/bin/kiro-cli"
				request.Plan.Components = []domain.ComponentDecision{
					{Kind: domain.ComponentMCPServer, Name: "alpha", Support: domain.SupportNative},
					{Kind: domain.ComponentMCPServer, Name: "beta", Support: domain.SupportNative},
				}
				runner.duplexOutput = acpResponse(0, `{"protocolVersion":1}`) + acpResponse(1, `{"sessionId":"s"}`) +
					acpStatus("s", "alpha", "connected", `[{"name":"a","disabled":false}]`) + acpStatus("s", "beta", "connected", `[{"name":"b","disabled":false}]`)
				runner.duplexLive = true
				want = [][]string{
					{"/test/bin/kiro-cli", "acp", "--agent-engine", "v3", "--auth-method", "cli"},
				}
			}

			outcome, err := (Activator{Runner: runner}).Activate(context.Background(), request)
			if err != nil || outcome.Activation != domain.ActivationActive {
				t.Fatalf("outcome=%+v err=%v", outcome, err)
			}
			if got := commandArgv(runner.commands); !reflect.DeepEqual(got, want) {
				t.Fatalf("commands = %#v, want %#v", got, want)
			}
		})
	}
}

func TestClientListingDoesNotConvertUnknownAuthentication(t *testing.T) {
	t.Parallel()
	request := activationRequest(t, domain.ClientCopilot)
	request.BackendExecutable = "/test/bin/copilot"
	outcome, err := (Activator{Runner: &recordingRunner{}}).Activate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Authentication != domain.AuthenticationNotChecked {
		t.Fatalf("authentication = %s", outcome.Authentication)
	}
}

func TestActivatorPinsDetectedKiroExecutableForACPVerification(t *testing.T) {
	t.Parallel()
	runner := &recordingRunner{duplexOutput: connectedACP("demo-server"), duplexLive: true}
	request := activationRequest(t, domain.ClientKiro)
	request.BackendExecutable = "/test/bin/kiro-cli"
	request.Plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "demo-server", Support: domain.SupportNative}}
	outcome, err := (Activator{Runner: runner}).Activate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Activation != domain.ActivationActive || outcome.Verification != domain.VerificationInstalled {
		t.Fatalf("outcome = %+v", outcome)
	}
	want := [][]string{{"/test/bin/kiro-cli", "acp", "--agent-engine", "v3", "--auth-method", "cli"}}
	if got := commandArgv(runner.commands); !reflect.DeepEqual(got, want) {
		t.Fatalf("commands = %#v, want %#v", got, want)
	}
	request.VerifyOnly = true
	retry, err := (Activator{Runner: runner}).Activate(context.Background(), request)
	if err != nil || retry.Activation != domain.ActivationActive || retry.Verification != domain.VerificationInstalled {
		t.Fatalf("verify-only retry = %+v, %v", retry, err)
	}
	want = append(want, []string{"/test/bin/kiro-cli", "acp", "--agent-engine", "v3", "--auth-method", "cli"})
	if got := commandArgv(runner.commands); !reflect.DeepEqual(got, want) {
		t.Fatalf("idempotent commands = %#v, want %#v", got, want)
	}
}

func TestActivatorRunOnlyRunnerCannotMutateKiro(t *testing.T) {
	t.Parallel()
	runner := &runOnlyRecordingRunner{}
	request := activationRequest(t, domain.ClientKiro)
	request.BackendExecutable = "/test/bin/kiro-cli"
	request.Plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "demo-server", Support: domain.SupportNative}}
	outcome, err := (Activator{Runner: runner}).Activate(context.Background(), request)
	if err == nil || !strings.Contains(err.Error(), "requires an ACP duplex process runner") || len(runner.commands) != 0 {
		t.Fatalf("run-only Kiro activation mutated before duplex preflight: outcome=%+v commands=%+v", outcome, runner.commands)
	}
}

func TestActivatorContainmentPreflightFailureCannotMutateKiro(t *testing.T) {
	t.Parallel()
	runner := &recordingRunner{capabilityErr: errors.New("delegated cgroup unavailable")}
	request := activationRequest(t, domain.ClientKiro)
	request.BackendExecutable = "/test/bin/kiro-cli"
	request.Plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "demo-server", Support: domain.SupportNative}}
	outcome, err := (Activator{Runner: runner}).Activate(context.Background(), request)
	if err == nil || !strings.Contains(err.Error(), "manual_activation_required") || len(runner.commands) != 0 {
		t.Fatalf("failed containment preflight mutated Kiro: outcome=%+v error=%v commands=%+v", outcome, err, runner.commands)
	}
}

func TestActivatorInstallsKiroSkillWithoutManualPowerImport(t *testing.T) {
	t.Parallel()
	request := activationRequest(t, domain.ClientKiro)
	request.BackendExecutable = "/test/bin/kiro-cli"
	request.Plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "guide", Support: domain.SupportNative}}
	source := filepath.Join(request.Delivery.ActivePath, "skills", "guide")
	writeTestFile(t, filepath.Join(source, "SKILL.md"), "---\nname: guide\ndescription: Guide\n---\n")
	digest, err := digestKiroSkillDirectory(source)
	if err != nil {
		t.Fatal(err)
	}
	request.Delivery.NativeObjects = []domain.NativeObjectOwnership{{
		ObjectID: "kiro-skill:guide", Kind: kiroSkillObjectKind, LogicalName: "guide",
		Path: filepath.Join(request.Client.ConfigRoot, "skills", "guide"), SourceRelative: "skills/guide",
		ManagedDigest: digest, ProtectionClass: "managed",
	}}
	outcome, err := (Activator{Runner: &recordingRunner{}}).Activate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Activation != domain.ActivationActive || outcome.Verification != domain.VerificationInstalled {
		t.Fatalf("outcome = %+v", outcome)
	}
	if _, err := os.Stat(filepath.Join(request.Client.ConfigRoot, "skills", "guide", "SKILL.md")); err != nil {
		t.Fatalf("installed Kiro skill: %v", err)
	}
}

func TestActivatorCleansNewMarketplaceWhenPluginInstallFails(t *testing.T) {
	t.Parallel()
	runner := &recordingRunner{run: func(command legacyports.Command) legacyports.CommandResult {
		if strings.Contains(strings.Join(command.Argv, " "), "plugin install") {
			return legacyports.CommandResult{ExitCode: 1, Stderr: []byte("install failed")}
		}
		return legacyports.CommandResult{}
	}}
	request := activationRequest(t, domain.ClientCopilot)
	request.BackendExecutable = "/test/bin/copilot"
	if _, err := (Activator{Runner: runner}).Activate(context.Background(), request); err == nil {
		t.Fatal("failed install unexpectedly succeeded")
	}
	marketplace := managedMarketplaceName(request.Plan.PhysicalArtifactID)
	want := [][]string{
		{"/test/bin/copilot", "plugin", "marketplace", "add", request.Delivery.ActivePath},
		{"/test/bin/copilot", "plugin", "install", "demo@" + marketplace},
		{"/test/bin/copilot", "plugin", "marketplace", "remove", marketplace},
	}
	if got := commandArgv(runner.commands); !reflect.DeepEqual(got, want) {
		t.Fatalf("commands = %#v, want %#v", got, want)
	}
}

func TestActivatorUsesPathSpecificManualHintsWithoutLeakingThemToJSON(t *testing.T) {
	t.Parallel()
	for _, client := range []domain.ClientID{domain.ClientCodex, domain.ClientKiro, domain.ClientVSCode} {
		client := client
		t.Run(string(client), func(t *testing.T) {
			request := activationRequest(t, client)
			outcome, err := (Activator{}).Activate(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			if outcome.Activation != domain.ActivationManual || len(outcome.UserActions) != 1 || len(outcome.LocalActions) != 1 {
				t.Fatalf("outcome = %+v", outcome)
			}
			if !strings.Contains(outcome.LocalActions[0], request.Delivery.ActivePath) {
				t.Fatalf("local action = %q", outcome.LocalActions[0])
			}
			body, err := json.Marshal(outcome)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(body), request.Delivery.ActivePath) {
				t.Fatalf("public JSON leaked local path: %s", body)
			}
		})
	}
}

func TestActivatorKeepsCursorManualUntilClientDiscoveryIsVerified(t *testing.T) {
	t.Parallel()
	request := activationRequest(t, domain.ClientCursor)
	outcome, err := (Activator{}).Activate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Activation != domain.ActivationManual || outcome.Verification != domain.VerificationPackageValid || len(outcome.UserActions) != 1 {
		t.Fatalf("outcome = %+v", outcome)
	}
}

func TestDeactivatorPreviewsThenRemovesNativeCopilotLifecycle(t *testing.T) {
	t.Parallel()
	runner := &recordingRunner{}
	request := domain.DeactivationRequest{
		Client: domain.DetectedClient{ClientID: domain.ClientCopilot}, DeclaredName: "demo",
		CurrentActivation: domain.ActivationActive, BackendExecutable: "/test/bin/copilot",
		PhysicalArtifactID: "demo-0123456789ab",
	}
	preview, err := (Activator{Runner: runner}).Deactivate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.ArtifactRemovalAllowed || preview.ExternalRemovalComplete || len(runner.commands) != 0 {
		t.Fatalf("preview = %+v, commands = %+v", preview, runner.commands)
	}
	request.Confirmed = true
	outcome, err := (Activator{Runner: runner}).Deactivate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !outcome.ArtifactRemovalAllowed || !outcome.ExternalRemovalComplete {
		t.Fatalf("outcome = %+v", outcome)
	}
	want := [][]string{
		{"/test/bin/copilot", "plugin", "uninstall", "demo@" + managedMarketplaceName(request.PhysicalArtifactID)},
		{"/test/bin/copilot", "plugin", "marketplace", "remove", managedMarketplaceName(request.PhysicalArtifactID)},
	}
	if got := commandArgv(runner.commands); !reflect.DeepEqual(got, want) {
		t.Fatalf("commands = %#v, want %#v", got, want)
	}
}

func TestDeactivatorPreviewsThenCleansManagedCodexMarketplace(t *testing.T) {
	t.Parallel()
	runner := &recordingRunner{}
	request := codexDeactivationRequest(t)
	preview, err := (Activator{Runner: runner}).Deactivate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.ArtifactRemovalAllowed || !preview.ExternalRemovalComplete || len(runner.commands) != 0 {
		t.Fatalf("preview = %+v, commands = %+v", preview, runner.commands)
	}
	request.Confirmed = true
	outcome, err := (Activator{Runner: runner}).Deactivate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !outcome.ArtifactRemovalAllowed || !outcome.ExternalRemovalComplete {
		t.Fatalf("outcome = %+v", outcome)
	}
	marketplace := managedMarketplaceName(request.PhysicalArtifactID)
	want := [][]string{
		{"/test/bin/codex", "plugin", "remove", request.DeclaredName + "@" + marketplace, "--json"},
		{"/test/bin/codex", "plugin", "marketplace", "remove", marketplace, "--json"},
	}
	if got := commandArgv(runner.commands); !reflect.DeepEqual(got, want) {
		t.Fatalf("commands = %#v, want %#v", got, want)
	}
}

// TestDeactivatorCleansStalePluginEntryWhenMarketplaceAlreadyGone covers the
// gap a live-native run found: if config.toml's marketplace source is
// already gone (e.g. a user manually ran only `codex plugin marketplace
// remove` off this code's own earlier guidance) but the separate
// `[plugins."id"] enabled = true` entry survives, managedCodexMarketplaceRegistered
// reports not-registered and the old code skipped cleanup entirely,
// reproducing the self-heal bug on every subsequent remove. The plugin's own
// registration must still be cleared whenever a live CLI is available,
// independent of marketplace registration.
func TestDeactivatorCleansStalePluginEntryWhenMarketplaceAlreadyGone(t *testing.T) {
	t.Parallel()
	runner := &recordingRunner{}
	request := codexDeactivationRequest(t)
	request.Confirmed = true
	marketplace := managedMarketplaceName(request.PhysicalArtifactID)
	writeTestFile(t, filepath.Join(request.Client.ConfigRoot, "config.toml"), fmt.Sprintf(`
[plugins."%s@%s"]
enabled = true
`, request.DeclaredName, marketplace))
	outcome, err := (Activator{Runner: runner}).Deactivate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !outcome.ArtifactRemovalAllowed || !outcome.ExternalRemovalComplete {
		t.Fatalf("outcome = %+v", outcome)
	}
	want := [][]string{{"/test/bin/codex", "plugin", "remove", request.DeclaredName + "@" + marketplace, "--json"}}
	if got := commandArgv(runner.commands); !reflect.DeepEqual(got, want) {
		t.Fatalf("commands = %#v, want %#v (marketplace remove must NOT run -- there is no marketplace source left to clean)", got, want)
	}
}

// TestDeactivatorBlocksRemovalWhenPluginEntryStaleAndCLIUnavailable covers a
// second gap from the same live-native finding: without a live CLI, a stale
// `[plugins."id"]` entry must block removal exactly like a registered
// marketplace does, not just fall through to ExternalRemovalComplete=true
// with nothing actually cleaned.
func TestDeactivatorBlocksRemovalWhenPluginEntryStaleAndCLIUnavailable(t *testing.T) {
	t.Parallel()
	request := codexDeactivationRequest(t)
	request.Confirmed = true
	request.BackendExecutable = ""
	marketplace := managedMarketplaceName(request.PhysicalArtifactID)
	writeTestFile(t, filepath.Join(request.Client.ConfigRoot, "config.toml"), fmt.Sprintf(`
[plugins."%s@%s"]
enabled = true
`, request.DeclaredName, marketplace))
	outcome, err := (Activator{}).Deactivate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	wantPluginRemove := fmt.Sprintf("codex plugin remove %s@%s --json", request.DeclaredName, marketplace)
	if outcome.ArtifactRemovalAllowed || outcome.ExternalRemovalComplete || outcome.Activation != domain.ActivationManual ||
		len(outcome.UserActions) != 1 || !strings.Contains(outcome.UserActions[0], wantPluginRemove) {
		t.Fatalf("outcome = %+v, want blocked with guidance containing %q", outcome, wantPluginRemove)
	}
}

func TestDeactivatorTreatsAbsentManagedCodexMarketplaceAsClean(t *testing.T) {
	t.Parallel()
	runner := &recordingRunner{run: func(command legacyports.Command) legacyports.CommandResult {
		return legacyports.CommandResult{ExitCode: 1, Stderr: []byte("marketplace `agentplugins-demo` is not configured or installed")}
	}}
	request := codexDeactivationRequest(t)
	request.Confirmed = true
	outcome, err := (Activator{Runner: runner}).Deactivate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !outcome.ArtifactRemovalAllowed || !outcome.ExternalRemovalComplete {
		t.Fatalf("outcome = %+v", outcome)
	}
}

func TestDeactivatorRetainsManagedCodexArtifactWhenPluginCleanupFails(t *testing.T) {
	t.Parallel()
	runner := &recordingRunner{run: func(command legacyports.Command) legacyports.CommandResult {
		return legacyports.CommandResult{ExitCode: 1, Stderr: []byte("config write failed")}
	}}
	request := codexDeactivationRequest(t)
	request.Confirmed = true
	outcome, err := (Activator{Runner: runner}).Deactivate(context.Background(), request)
	if err == nil || !strings.Contains(err.Error(), "remove managed Codex plugin") {
		t.Fatalf("outcome = %+v, error = %v", outcome, err)
	}
	if outcome.ExternalRemovalComplete {
		t.Fatalf("failed cleanup claimed external completion: %+v", outcome)
	}
}

func TestDeactivatorRetainsManagedCodexArtifactWhenMarketplaceCleanupFails(t *testing.T) {
	t.Parallel()
	runner := &recordingRunner{run: func(command legacyports.Command) legacyports.CommandResult {
		if len(command.Argv) >= 3 && command.Argv[1] == "plugin" && command.Argv[2] == "remove" {
			return legacyports.CommandResult{}
		}
		return legacyports.CommandResult{ExitCode: 1, Stderr: []byte("config write failed")}
	}}
	request := codexDeactivationRequest(t)
	request.Confirmed = true
	outcome, err := (Activator{Runner: runner}).Deactivate(context.Background(), request)
	if err == nil || !strings.Contains(err.Error(), "remove managed Codex marketplace") {
		t.Fatalf("outcome = %+v, error = %v", outcome, err)
	}
	if outcome.ExternalRemovalComplete {
		t.Fatalf("failed cleanup claimed external completion: %+v", outcome)
	}
}

func TestDeactivatorNeverRemovesSameNameUserCodexMarketplace(t *testing.T) {
	t.Parallel()
	runner := &recordingRunner{}
	request := codexDeactivationRequest(t)
	request.Confirmed = true
	marketplace := managedMarketplaceName(request.PhysicalArtifactID)
	writeTestFile(t, filepath.Join(request.Client.ConfigRoot, "config.toml"), fmt.Sprintf(`
[marketplaces.%s]
source_type = "local"
source = "/user/replaced-marketplace"
`, marketplace))
	outcome, err := (Activator{Runner: runner}).Deactivate(context.Background(), request)
	if err == nil || !strings.Contains(err.Error(), "no longer points at the managed artifact") {
		t.Fatalf("outcome = %+v, error = %v", outcome, err)
	}
	if len(runner.commands) != 0 || outcome.ExternalRemovalComplete {
		t.Fatalf("user marketplace was targeted: outcome=%+v commands=%+v", outcome, runner.commands)
	}
}

func TestDeactivatorRetainsManagedCodexArtifactWhenCLIIsUnavailable(t *testing.T) {
	t.Parallel()
	request := codexDeactivationRequest(t)
	request.Confirmed = true
	request.BackendExecutable = ""
	outcome, err := (Activator{}).Deactivate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	marketplace := managedMarketplaceName(request.PhysicalArtifactID)
	wantPluginRemove := fmt.Sprintf("codex plugin remove %s@%s --json", request.DeclaredName, marketplace)
	wantMarketplaceRemove := fmt.Sprintf("codex plugin marketplace remove %s --json", marketplace)
	if outcome.ArtifactRemovalAllowed || outcome.ExternalRemovalComplete || outcome.Activation != domain.ActivationManual ||
		len(outcome.UserActions) != 1 ||
		!strings.Contains(outcome.UserActions[0], wantPluginRemove) ||
		!strings.Contains(outcome.UserActions[0], wantMarketplaceRemove) {
		t.Fatalf("outcome = %+v, want guidance containing both %q and %q", outcome, wantPluginRemove, wantMarketplaceRemove)
	}
}

func TestDeactivatorAllowsIdempotentCodexRemovalWhenRegistryIsAlreadyAbsent(t *testing.T) {
	t.Parallel()
	request := codexDeactivationRequest(t)
	request.Confirmed = true
	request.BackendExecutable = ""
	if err := os.Remove(filepath.Join(request.Client.ConfigRoot, "config.toml")); err != nil {
		t.Fatal(err)
	}
	outcome, err := (Activator{}).Deactivate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !outcome.ArtifactRemovalAllowed || !outcome.ExternalRemovalComplete || outcome.Activation != domain.ActivationNotRequired {
		t.Fatalf("outcome = %+v", outcome)
	}
}

func TestDeactivatorTreatsAlreadyAbsentNativeObjectsAsRemoved(t *testing.T) {
	t.Parallel()
	runner := &recordingRunner{run: func(command legacyports.Command) legacyports.CommandResult {
		if strings.Contains(strings.Join(command.Argv, " "), "marketplace remove") {
			return legacyports.CommandResult{ExitCode: 1, Stderr: []byte(`Marketplace "demo" is not registered`)}
		}
		return legacyports.CommandResult{ExitCode: 1, Stderr: []byte(`Plugin "demo" is not installed`)}
	}}
	outcome, err := (Activator{Runner: runner}).Deactivate(context.Background(), domain.DeactivationRequest{
		Client: domain.DetectedClient{ClientID: domain.ClientVSCode}, DeclaredName: "demo",
		CurrentActivation: domain.ActivationActive, BackendExecutable: "/test/bin/copilot",
		PhysicalArtifactID: "demo-0123456789ab", Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !outcome.ArtifactRemovalAllowed || !outcome.ExternalRemovalComplete {
		t.Fatalf("outcome = %+v", outcome)
	}
}

func TestDeactivatorDoesNotClaimManualCopilotLifecycle(t *testing.T) {
	t.Parallel()
	runner := &recordingRunner{}
	outcome, err := (Activator{Runner: runner}).Deactivate(context.Background(), domain.DeactivationRequest{
		Client: domain.DetectedClient{ClientID: domain.ClientCopilot}, DeclaredName: "demo",
		CurrentActivation: domain.ActivationManual, BackendExecutable: "/test/bin/copilot",
		PhysicalArtifactID: "demo-0123456789ab", Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ArtifactRemovalAllowed || len(outcome.UserActions) != 1 || len(runner.commands) != 0 {
		t.Fatalf("outcome = %+v, commands = %+v", outcome, runner.commands)
	}
}

func TestDeactivatorPreservesManualClientArtifactsUntilAcknowledged(t *testing.T) {
	t.Parallel()
	for _, client := range []domain.ClientID{domain.ClientCodex, domain.ClientKiro, domain.ClientVSCode} {
		outcome, err := (Activator{}).Deactivate(context.Background(), domain.DeactivationRequest{
			Client: domain.DetectedClient{ClientID: client}, DeclaredName: "demo",
			CurrentActivation: domain.ActivationManual,
		})
		if err != nil {
			t.Fatal(err)
		}
		if outcome.ArtifactRemovalAllowed || outcome.Activation != domain.ActivationManual || outcome.ExternalRemovalComplete || len(outcome.UserActions) != 1 {
			t.Fatalf("client %s outcome = %+v", client, outcome)
		}
		if !strings.Contains(outcome.UserActions[0], "--external-uninstalled") {
			t.Fatalf("client %s actions = %+v", client, outcome.UserActions)
		}
	}
}

func TestChatGPTActivationIsManualAndNeverUsesCodexRunner(t *testing.T) {
	t.Parallel()
	request := activationRequest(t, domain.ClientChatGPT)
	request.Plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentApp, Name: "demo", Support: domain.SupportProjected}}
	request.BackendExecutable = "/test/bin/codex"
	runner := &recordingRunner{}
	outcome, err := (Activator{Runner: runner}).Activate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Activation != domain.ActivationManual || outcome.Verification != domain.VerificationPackageValid || len(runner.commands) != 0 {
		t.Fatalf("ChatGPT activation = %+v commands=%+v", outcome, runner.commands)
	}
	if len(outcome.UserActions) != 1 || !strings.Contains(outcome.UserActions[0], "Developer Mode") || !strings.Contains(outcome.UserActions[0], ".app.json") {
		t.Fatalf("ChatGPT actions = %+v", outcome.UserActions)
	}
}

func TestChatGPTSkillsOnlyActivationDoesNotMentionAppRegistration(t *testing.T) {
	t.Parallel()
	request := activationRequest(t, domain.ClientChatGPT)
	request.Plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "docs", Support: domain.SupportProjected}}
	outcome, err := (Activator{}).Activate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Activation != domain.ActivationManual || len(outcome.UserActions) != 1 || !strings.Contains(outcome.UserActions[0], "skills-only") || strings.Contains(outcome.UserActions[0], ".app.json") || strings.Contains(outcome.UserActions[0], "Developer Mode") {
		t.Fatalf("ChatGPT skills-only activation = %+v", outcome)
	}
}

func TestChatGPTActivationCanOnlyCompleteByExplicitAttestation(t *testing.T) {
	t.Parallel()
	request := activationRequest(t, domain.ClientChatGPT)
	request.ActivationComplete = true
	outcome, err := (Activator{}).Activate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Activation != domain.ActivationActive || outcome.Verification != domain.VerificationInstalled || !outcome.ActivationAttested {
		t.Fatalf("ChatGPT attestation = %+v", outcome)
	}
}

func codexDeactivationRequest(t *testing.T) domain.DeactivationRequest {
	t.Helper()
	configRoot := t.TempDir()
	managedArtifact := filepath.Join(t.TempDir(), "managed", "demo")
	physicalArtifactID := "demo-0123456789ab"
	marketplace := managedMarketplaceName(physicalArtifactID)
	writeTestFile(t, filepath.Join(configRoot, "config.toml"), fmt.Sprintf(`
[marketplaces.%s]
source_type = "local"
source = %q

[marketplaces.user-marketplace]
source_type = "local"
source = "/user/marketplace"
`, marketplace, managedArtifact))
	return domain.DeactivationRequest{
		Client: domain.DetectedClient{ClientID: domain.ClientCodex, ConfigRoot: configRoot}, DeclaredName: "demo",
		CurrentActivation: domain.ActivationActive, BackendExecutable: "/test/bin/codex",
		PhysicalArtifactID: physicalArtifactID, ManagedArtifactPath: managedArtifact,
		ExternalUninstalled: true,
	}
}

func activationRequest(t *testing.T, client domain.ClientID) domain.ActivationRequest {
	t.Helper()
	root := t.TempDir()
	base := filepath.Join(root, "managed")
	active := filepath.Join(base, "demo")
	if err := os.MkdirAll(active, 0o755); err != nil {
		t.Fatal(err)
	}
	return domain.ActivationRequest{
		Client:       domain.DetectedClient{ClientID: client, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(root, ".kiro")},
		DeclaredName: "demo",
		Plan: domain.DeliveryPlan{
			ClientID: client, ActivePath: active, PhysicalArtifactID: "demo-0123456789ab",
			DeclaredVersion: "1.0.0", Authentication: domain.AuthenticationNotChecked,
		},
		Delivery: domain.StagedDelivery{ClientID: client, OwnedBase: base, ActivePath: active},
	}
}

func commandArgv(commands []legacyports.Command) [][]string {
	result := make([][]string, 0, len(commands))
	for _, command := range commands {
		result = append(result, command.Argv)
	}
	return result
}
