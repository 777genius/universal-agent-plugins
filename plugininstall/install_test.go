package plugininstall

import (
	"context"
	"errors"
	"runtime"
	"testing"

	"github.com/777genius/plugin-kit-ai/plugininstall/domain"
)

func TestExitCodeFromErrMapsDomainAndContextErrors(t *testing.T) {
	t.Parallel()

	if got := ExitCodeFromErr(nil); got != 0 {
		t.Fatalf("nil exit code = %d", got)
	}
	if got := ExitCodeFromErr(domain.NewError(domain.ExitChecksum, "bad")); got != int(domain.ExitChecksum) {
		t.Fatalf("domain exit code = %d", got)
	}
	if got := ExitCodeFromErr(context.Canceled); got != 1 {
		t.Fatalf("canceled exit code = %d", got)
	}
	if got := ExitCodeFromErr(context.DeadlineExceeded); got != 3 {
		t.Fatalf("deadline exit code = %d", got)
	}
	if got := ExitCodeFromErr(errors.New("x")); got != 1 {
		t.Fatalf("generic exit code = %d", got)
	}
}

func TestHostTargetDefaultsAndTrims(t *testing.T) {
	t.Parallel()

	got := hostTarget("  ", " \t ")
	if got.GOOS != runtime.GOOS || got.GOARCH != runtime.GOARCH {
		t.Fatalf("default target = %#v", got)
	}
	got = hostTarget(" linux ", " amd64 ")
	if got.GOOS != "linux" || got.GOARCH != "amd64" {
		t.Fatalf("trimmed target = %#v", got)
	}
}
