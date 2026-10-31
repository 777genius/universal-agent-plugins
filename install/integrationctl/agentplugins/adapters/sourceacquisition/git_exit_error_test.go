package sourceacquisition

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func TestGitTransportExitFailureRetainsExactExistingMessages(t *testing.T) {
	for _, tc := range []struct {
		native bool
		want   string
	}{
		{false, "git fetch secret-argument failed with exit code 128: secret stream"},
		{true, "native Git authentication or transport failed (exit 128); configure Git credentials or SSH access and retry"},
	} {
		err := gitTransportExitFailure(Command{Args: []string{"fetch", "secret-argument"}, NativeCredentials: tc.native}, 128, []byte(" \nsecret stream\n "))
		if err.Error() != tc.want {
			t.Fatal("clean Git exit changed its existing message")
		}
		var status interface{ GitTransportExitCode() int }
		if !errors.As(err, &status) || status.GitTransportExitCode() != 128 {
			t.Fatal("clean Git exit lost its structural status")
		}
	}
}

func TestGitTransportExitCarrierPreservesCauseAndSignedStatus(t *testing.T) {
	original := errors.New("inert original cause")
	carrier := &gitTransportExitError{cause: original, code: -1}
	wrapped := fmt.Errorf("existing outer context: %w", carrier)
	if !errors.Is(wrapped, original) || reflect.ValueOf(carrier.Unwrap()) != reflect.ValueOf(original) || carrier.Error() != original.Error() {
		t.Fatal("Git exit carrier changed its original cause")
	}
	var status interface{ GitTransportExitCode() int }
	if !errors.As(wrapped, &status) || status.GitTransportExitCode() != -1 {
		t.Fatal("wrapped Git status was truncated or changed")
	}
}
