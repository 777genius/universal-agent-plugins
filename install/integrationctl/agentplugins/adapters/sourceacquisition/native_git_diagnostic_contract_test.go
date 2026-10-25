package sourceacquisition

import (
	"context"
	"errors"
	"os/exec"
	"reflect"
	"testing"

	processadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
)

type nativeDiagnosticDelegate func(context.Context, Command) ([]byte, error)

func (delegate nativeDiagnosticDelegate) Run(ctx context.Context, command Command) ([]byte, error) {
	return delegate(ctx, command)
}

type nativeDiagnosticSecretError struct{}
type nativeDiagnosticContextKey struct{}

func (*nativeDiagnosticSecretError) Error() string {
	panic("native diagnostic must not read error payload")
}

func TestNativeTransportDiagnosticPreservesIdentityAndFailureBound(t *testing.T) {
	ctx := context.WithValue(t.Context(), nativeDiagnosticContextKey{}, "secret context")
	command := Command{Dir: "/secret/path", Args: []string{"secret command", "secret argument"}, Stdin: []byte("secret input"), NativeCredentials: true, MaxOutputBytes: 73}
	body, original := []byte("secret output"), &nativeDiagnosticSecretError{}
	calls, reports := 0, 0
	runner := &nativeTransportDiagnosticRunner{operation: "acquire", delegate: nativeDiagnosticDelegate(func(got context.Context, input Command) ([]byte, error) {
		calls++
		if got != ctx || !reflect.DeepEqual(input, command) || &input.Args[0] != &command.Args[0] || &input.Stdin[0] != &command.Stdin[0] {
			t.Fatal("native diagnostic changed context/command authority")
		}
		return body, original
	}), report: func(operation, class, status string) {
		reports++
		if operation != "acquire" || class != "other_git_or_process" || status != "unavailable" {
			t.Fatal("native diagnostic emitted nonfixed payload")
		}
	}}
	for range 6 {
		got, err := runner.Run(ctx, command)
		if reflect.ValueOf(err) != reflect.ValueOf(original) || &got[0] != &body[0] {
			t.Fatal("native diagnostic changed original bytes/error")
		}
	}
	if calls != 6 || reports != 4 || runner.failures.Load() != 6 {
		t.Fatal("native diagnostic changed call/failure/log bounds")
	}
}

func TestNativeTransportDiagnosticSuccessAndClosedClasses(t *testing.T) {
	runner := &nativeTransportDiagnosticRunner{delegate: nativeDiagnosticDelegate(func(context.Context, Command) ([]byte, error) {
		return []byte("success"), nil
	}), report: func(string, string, string) { t.Fatal("successful native command logged") }}
	if _, err := runner.Run(t.Context(), Command{}); err != nil || runner.failures.Load() != 0 {
		t.Fatal("successful native command changed")
	}
	for _, tc := range []struct {
		err  error
		want string
	}{{context.Canceled, "context_canceled"}, {context.DeadlineExceeded, "context_deadline"}, {processadapter.ErrOrdinaryContainmentUnsupported, "ordinary_unsupported"}, {processadapter.ErrStdoutLimitExceeded, "stdout_limit"}, {processadapter.ErrOrdinaryForcedCleanup, "ordinary_forced_cleanup"}, {errors.Join(context.Canceled, processadapter.ErrOrdinaryForcedCleanup), "context_canceled"}, {&exec.ExitError{}, "other_git_or_process"}, {errors.New("opaque native policy failure"), "other_git_or_process"}} {
		if class, status := nativeTransportDiagnosticClass(tc.err); class != tc.want || status != "unavailable" {
			t.Fatalf("native diagnostic class=%q want=%q", class, tc.want)
		}
	}
}

func TestNativeTransportDiagnosticPositiveExitDoesNotReadError(t *testing.T) {
	original := errors.Join(&gitTransportExitError{cause: &nativeDiagnosticSecretError{}, code: 128})
	body := []byte("secret output")
	reports := 0
	runner := &nativeTransportDiagnosticRunner{operation: "secret operation", delegate: nativeDiagnosticDelegate(func(context.Context, Command) ([]byte, error) {
		return body, original
	}), report: func(operation, class, status string) {
		reports++
		if operation != "unknown" || class != "git_exit" || status != "128" {
			t.Fatal("positive Git exit or closed operation classification changed")
		}
	}}
	got, err := runner.Run(t.Context(), Command{Args: []string{"secret command"}})
	if reflect.ValueOf(err) != reflect.ValueOf(original) || &got[0] != &body[0] || reports != 1 {
		t.Fatal("positive native exit diagnostic changed delegation")
	}
}
