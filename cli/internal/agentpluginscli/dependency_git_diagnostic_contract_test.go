package agentpluginscli

import (
	"context"
	"errors"
	"reflect"
	"testing"

	processadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
)

type dependencyDiagnosticDelegate func(context.Context, sourceacquisition.Command) ([]byte, error)

func (delegate dependencyDiagnosticDelegate) Run(ctx context.Context, command sourceacquisition.Command) ([]byte, error) {
	return delegate(ctx, command)
}

type dependencyDiagnosticSecretError struct{}
type dependencyDiagnosticContextKey struct{}

func (*dependencyDiagnosticSecretError) Error() string {
	panic("Git argv/stream/credential error text must not be read")
}

func TestDependencyGitDiagnosticPreservesCallAndOmitsAllPayloads(t *testing.T) {
	ctx := context.WithValue(t.Context(), dependencyDiagnosticContextKey{}, "secret context")
	command := sourceacquisition.Command{Dir: "/secret/path", Args: []string{"-C", "/secret/repository", "fetch", "secret argument"}, Stdin: []byte("secret input")}
	body, original := []byte("secret output"), &dependencyDiagnosticSecretError{}
	calls, reports := 0, 0
	runner := &dependencyGitDiagnosticRunner{delegate: dependencyDiagnosticDelegate(func(got context.Context, input sourceacquisition.Command) ([]byte, error) {
		calls++
		if got != ctx || !reflect.DeepEqual(input, command) || &input.Args[0] != &command.Args[0] || &input.Stdin[0] != &command.Stdin[0] {
			t.Fatal("diagnostic changed context/command authority")
		}
		return body, original
	}), report: func(phase, class string) {
		reports++
		if phase != "fetch" || class != "other_git_or_process" {
			t.Fatalf("diagnostic emitted nonfixed payload: phase=%q class=%q", phase, class)
		}
	}}
	for range 6 {
		got, err := runner.Run(ctx, command)
		if reflect.ValueOf(err) != reflect.ValueOf(original) || &got[0] != &body[0] {
			t.Fatal("diagnostic changed original bytes/error")
		}
	}
	if calls != 6 || reports != 4 {
		t.Fatalf("diagnostic call/log bounds changed: calls=%d reports=%d", calls, reports)
	}
}

func TestDependencyGitDiagnosticSuccessIsSilentAndClassesAreFixed(t *testing.T) {
	runner := &dependencyGitDiagnosticRunner{delegate: dependencyDiagnosticDelegate(func(context.Context, sourceacquisition.Command) ([]byte, error) {
		return []byte("success output"), nil
	}), report: func(string, string) { t.Fatal("successful Git invocation logged") }}
	if _, err := runner.Run(t.Context(), sourceacquisition.Command{}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		err  error
		want string
	}{{context.Canceled, "context_canceled"}, {context.DeadlineExceeded, "context_deadline"}, {processadapter.ErrOrdinaryContainmentUnsupported, "ordinary_unsupported"}, {processadapter.ErrStdoutLimitExceeded, "stdout_limit"}, {errors.New("secret error"), "other_git_or_process"}} {
		if got := dependencyGitDiagnosticClass(tc.err); got != tc.want {
			t.Fatalf("diagnostic class=%q want=%q", got, tc.want)
		}
	}
	if got := dependencyGitDiagnosticPhase([]string{"secret command"}); got != "unknown" {
		t.Fatalf("unknown argv became diagnostic payload: %q", got)
	}
}
