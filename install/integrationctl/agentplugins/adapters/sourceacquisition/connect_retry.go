package sourceacquisition

import (
	"bytes"
	"context"
	"errors"
	"math/rand/v2"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
)

// ErrGitConnect classifies a Git read whose remote connection could not be
// established or was lost while waiting for data. Only clearly connect-class
// failures carry it; authentication, missing repositories, bad refs, TLS and
// verification failures never do.
var ErrGitConnect = errors.New("git remote connection failed")

const (
	// connectRetryBudget bounds a read and its single retry together, the same
	// two minutes one native Git command may take.
	connectRetryBudget = 2 * time.Minute
	// connectRetryMinWindow is the least time a retry must have left in the
	// budget to be worth starting.
	connectRetryMinWindow = 15 * time.Second
)

// connectRetryPause is a jittered 1-2 s pause; tests replace it.
var connectRetryPause = func() time.Duration {
	return time.Second + rand.N(time.Second) //nolint:gosec // jitter, not security
}

// connectRetryNow is the budget clock; tests replace it.
var connectRetryNow = time.Now

// Lowercased git/curl/ssh stderr phrases of a connection that was never made
// or stalled while receiving.
var gitConnectMarkers = []string{
	"couldn't connect to server",
	"connection timed out",
	"recv failure:",
}

// Git's, curl's and ssh's own phrases for failures about credentials,
// existence, refs or trust. They are matched as whole phrases, so repository
// names (openssl/openssl, rustls) and millisecond counts never match. Such a
// failure is never retried even when it also mentions a timeout.
var gitNonConnectMarkers = []string{
	"authentication failed", "could not read username", "could not read password",
	"permission denied (", "repository not found", "remote: not found",
	"the requested url returned error: 4", "returned error: 4",
	"ssl certificate problem", "ssl: certificate", "certificate verify failed",
	"server certificate verification failed", "unknown revision", "couldn't find remote ref",
}

// git: "fatal: repository '<url>' not found".
var gitRepositoryNotFound = regexp.MustCompile(`fatal: repository '[^'\n]*' not found`)

// isGitConnectFailure reports a failed Git read (exit 128, nothing on stdout)
// whose stderr shows only a connect-class failure. Anything else is not
// connect-class (fail closed).
func isGitConnectFailure(code int, stdout, stderr []byte) bool {
	if code != 128 || len(bytes.TrimSpace(stdout)) != 0 {
		return false
	}
	text := strings.ToLower(string(stderr))
	for _, marker := range gitNonConnectMarkers {
		if strings.Contains(text, marker) {
			return false
		}
	}
	if gitRepositoryNotFound.MatchString(text) {
		return false
	}
	for _, marker := range gitConnectMarkers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	// curl: "Failed to connect to <host> port <n> after <ms> ms: ...".
	_, after, ok := strings.Cut(text, "failed to connect to ")
	return ok && strings.Contains(after, " after ") && strings.Contains(after, " ms")
}

// gitConnectError keeps an existing public failure message while marking it
// connect-class for callers and for the retry.
type gitConnectError struct{ message string }

func (err *gitConnectError) Error() string        { return err.message }
func (err *gitConnectError) Is(target error) bool { return target == ErrGitConnect }

// gitFailure returns message as an error that is connect-class exactly when
// cause is; the message itself is unchanged either way.
func gitFailure(cause error, message string) error {
	if errors.Is(cause, ErrGitConnect) {
		return &gitConnectError{message: message}
	}
	return errors.New(message)
}

type connectRetryCounterKey struct{}

// WithConnectRetryCounter returns ctx whose connect retries add to counter, so
// callers can trace them.
func WithConnectRetryCounter(ctx context.Context, counter *atomic.Int32) context.Context {
	return context.WithValue(ctx, connectRetryCounterKey{}, counter)
}

// withGitConnectRetry runs one pure remote read and, when it failed only to
// connect, repeats it once after a short jittered pause. attempt must be a
// complete read that starts from nothing (a fresh temporary workspace or a
// stateless ls-remote). Cancellation, deadlines and every other failure are
// returned as they are. The retry stays inside connectRetryBudget measured
// from the first attempt; when that budget, not the caller, ends the retry,
// the first failure is returned.
func withGitConnectRetry[T any](ctx context.Context, attempt func(context.Context) (T, error)) (T, error) {
	deadline := connectRetryNow().Add(connectRetryBudget)
	value, err := attempt(ctx)
	if err == nil || ctx.Err() != nil || !errors.Is(err, ErrGitConnect) {
		return value, err
	}
	pause := connectRetryPause()
	if deadline.Sub(connectRetryNow()) < pause+connectRetryMinWindow {
		return value, err
	}
	timer := time.NewTimer(pause)
	select {
	case <-ctx.Done():
		timer.Stop()
		var zero T
		return zero, ctx.Err()
	case <-timer.C:
	}
	if counter, ok := ctx.Value(connectRetryCounterKey{}).(*atomic.Int32); ok {
		counter.Add(1)
	}
	retryCtx, cancel := context.WithTimeout(ctx, deadline.Sub(connectRetryNow()))
	defer cancel()
	second, retryErr := attempt(retryCtx)
	if retryErr != nil && ctx.Err() == nil && retryCtx.Err() != nil {
		return value, err
	}
	return second, retryErr
}
