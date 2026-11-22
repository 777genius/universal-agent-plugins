package sourceacquisition

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const (
	testCommit      = "0123456789abcdef0123456789abcdef01234567"
	connectStderr   = "fatal: unable to access 'https://github.com/acme/repo.git/': Failed to connect to github.com port 443 after 75004 ms: Couldn't connect to server\n"
	authStderr      = "remote: Invalid username or password.\nfatal: Authentication failed for 'https://github.com/acme/repo.git/'\n"
	notFoundStderr  = "remote: Repository not found.\nfatal: repository 'https://github.com/acme/repo.git/' not found\n"
	recvStallStderr = "error: RPC failed; curl 56 Recv failure: Operation timed out\nfatal: expected flush after ref listing\n"
)

// scriptedRunner answers Git commands like OSRunner would, failing the first
// reads of a kind with scripted stderr.
type scriptedRunner struct {
	mu      sync.Mutex
	calls   map[string]int // by first remote verb, e.g. ls-remote or fetch
	fails   map[string][]string
	onCall  func(verb string, n int)
	objects map[string]int // fetch attempts by fetched object
}

func remoteVerb(args []string) string {
	for _, arg := range args {
		if arg == "ls-remote" || arg == "fetch" {
			return arg
		}
	}
	return ""
}

func (runner *scriptedRunner) Run(ctx context.Context, command Command) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	verb := remoteVerb(command.Args)
	runner.mu.Lock()
	if runner.calls == nil {
		runner.calls, runner.objects = map[string]int{}, map[string]int{}
	}
	runner.calls[verb]++
	n := runner.calls[verb]
	var stderr string
	if verb == "fetch" {
		object := command.Args[len(command.Args)-1]
		runner.objects[object]++
		if script := runner.fails[object]; runner.objects[object] <= len(script) {
			stderr = script[runner.objects[object]-1]
		}
	} else if script := runner.fails[verb]; n <= len(script) {
		stderr = script[n-1]
	}
	runner.mu.Unlock()
	if runner.onCall != nil {
		runner.onCall(verb, n)
	}
	if stderr != "" {
		return nil, gitExitResult(command, 128, nil, []byte(stderr))
	}
	switch {
	case verb == "ls-remote":
		return []byte(testCommit + "\trefs/heads/main\n"), nil
	case len(command.Args) > 2 && command.Args[2] == "rev-parse":
		return []byte(testCommit + "\n"), nil
	}
	return nil, nil
}

func (runner *scriptedRunner) count(verb string) int {
	runner.mu.Lock()
	defer runner.mu.Unlock()
	return runner.calls[verb]
}

func noRetryPause(t *testing.T, pause time.Duration) {
	t.Helper()
	original := connectRetryPause
	connectRetryPause = func() time.Duration { return pause }
	t.Cleanup(func() { connectRetryPause = original })
}

func resolveMain(ctx context.Context, runner Runner, t *testing.T) (RefResolution, error) {
	return Acquirer{Runner: runner, TempRoot: t.TempDir()}.ResolveGitHubRef(ctx, "acme/repo", "main")
}

func TestIsGitConnectFailureFailsClosed(t *testing.T) {
	for _, test := range []struct {
		name    string
		code    int
		stdout  string
		stderr  string
		connect bool
	}{
		{"connect timeout", 128, "", connectStderr, true},
		{"refused", 128, "", "fatal: unable to access 'x': Couldn't connect to server\n", true},
		{"ssh timeout", 128, "", "ssh: connect to host github.com port 22: Connection timed out\nfatal: Could not read from remote repository.\n", true},
		{"receive stall", 128, "", recvStallStderr, true},
		{"auth", 128, "", authStderr, false},
		{"not found", 128, "", notFoundStderr, false},
		{"auth beside a timeout", 128, "", "fatal: Authentication failed\nConnection timed out\n", false},
		{"tls", 128, "", "fatal: unable to access 'x': SSL certificate problem: unable to get local issuer certificate\n", false},
		{"bad ref", 128, "", "fatal: couldn't find remote ref refs/heads/nope\n", false},
		{"other exit code", 1, "", connectStderr, false},
		{"partial stdout", 128, "abc\trefs/heads/main\n", connectStderr, false},
		{"unknown", 128, "", "fatal: something unexpected\n", false},
		{"failed to connect without timing", 128, "", "Failed to connect to proxy\n", false},
		// Repository names and millisecond counts never look like a refusal.
		{"openssl repository", 128, "", "fatal: unable to access 'https://github.com/openssl/openssl.git/': Failed to connect to github.com port 443 after 14031 ms: Couldn't connect to server\n", true},
		{"rustls repository", 128, "", "fatal: unable to access 'https://github.com/rustls/rustls.git/': Recv failure: Operation timed out\n", true},
		{"tls-named repository", 128, "", "fatal: unable to access 'https://github.com/acme/tls-invalid-notfound.git/': Couldn't connect to server\n", true},
		{"404 milliseconds", 128, "", "fatal: unable to access 'https://github.com/acme/repo.git/': Failed to connect to github.com port 443 after 40404 ms: Couldn't connect to server\n", true},
		{"401 and 403 milliseconds", 128, "", "fatal: unable to access 'x': Failed to connect to github.com port 443 after 14031 ms: Couldn't connect to server\nafter 1403 ms\n", true},
		// Git's real refusals stay refusals.
		{"http 403", 128, "", "fatal: unable to access 'https://github.com/acme/repo.git/': The requested URL returned error: 403\n", false},
		{"http 404", 128, "", "fatal: unable to access 'https://github.com/acme/repo.git/': The requested URL returned error: 404\n", false},
		{"remote not found", 128, "", "remote: Not Found\nfatal: repository 'https://github.com/acme/repo.git/' not found\n", false},
		{"repository not found beside a stall", 128, "", "fatal: repository 'https://github.com/acme/repo.git/' not found\nRecv failure: Operation timed out\n", false},
		{"ssh permission", 128, "", "git@github.com: Permission denied (publickey).\nfatal: Could not read from remote repository.\n", false},
		{"tls verify", 128, "", "fatal: unable to access 'https://github.com/acme/repo.git/': SSL: certificate subject name 'x' does not match target host name 'y'\n", false},
		{"tls with a timeout", 128, "", "fatal: unable to access 'x': server certificate verification failed. CAfile: none CRLfile: none\nConnection timed out\n", false},
	} {
		if got := isGitConnectFailure(test.code, []byte(test.stdout), []byte(test.stderr)); got != test.connect {
			t.Errorf("%s: connect = %v, want %v", test.name, got, test.connect)
		}
	}
}

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (clock *fakeClock) Now() time.Time {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	return clock.now
}

func (clock *fakeClock) advance(by time.Duration) {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	clock.now = clock.now.Add(by)
}

func useFakeClock(t *testing.T) *fakeClock {
	t.Helper()
	clock := &fakeClock{now: time.Unix(1_000_000, 0)}
	original := connectRetryNow
	connectRetryNow = clock.Now
	t.Cleanup(func() { connectRetryNow = original })
	return clock
}

// The budget runs from the first attempt: a retry is skipped when the first
// attempt used most of it, and otherwise gets only what is left.
func TestConnectRetryBudgetCountsFromTheFirstAttempt(t *testing.T) {
	noRetryPause(t, time.Millisecond)
	for _, test := range []struct {
		firstTook time.Duration
		attempts  int
	}{
		{firstTook: connectRetryBudget - 10*time.Second, attempts: 1},
		{firstTook: connectRetryBudget - 30*time.Second, attempts: 2},
	} {
		clock := useFakeClock(t)
		start := clock.Now()
		attempts := 0
		var retryDeadline time.Time
		_, err := withGitConnectRetry(t.Context(), func(ctx context.Context) (int, error) {
			attempts++
			if attempts == 1 {
				clock.advance(test.firstTook)
				return 0, &gitConnectError{message: "stalled"}
			}
			retryDeadline, _ = ctx.Deadline()
			return 1, nil
		})
		if attempts != test.attempts {
			t.Fatalf("first attempt took %v: %d attempts, want %d (err %v)", test.firstTook, attempts, test.attempts, err)
		}
		if attempts == 2 {
			// The retry may run only for what is left of the budget (real time).
			left := connectRetryBudget - clock.Now().Sub(start)
			if until := time.Until(retryDeadline); until > left || until <= 0 {
				t.Fatalf("retry deadline in %v, want within the %v left of the budget", until, left)
			}
		}
	}
}

// Cancellation during the pause returns context.Canceled without a second
// attempt.
func TestConnectRetryCancelDuringPauseRunsNoSecondAttempt(t *testing.T) {
	noRetryPause(t, 10*time.Second)
	ctx, cancel := context.WithCancel(t.Context())
	var canceled atomic.Bool
	var afterCancel atomic.Int32
	attempts := 0
	_, err := withGitConnectRetry(ctx, func(context.Context) (int, error) {
		attempts++
		if canceled.Load() {
			afterCancel.Add(1)
		}
		time.AfterFunc(20*time.Millisecond, func() { canceled.Store(true); cancel() })
		return 0, &gitConnectError{message: "stalled"}
	})
	if !errors.Is(err, context.Canceled) || attempts != 1 || afterCancel.Load() != 0 {
		t.Fatalf("err=%v attempts=%d after cancel=%d; want context.Canceled and one attempt", err, attempts, afterCancel.Load())
	}
}

func TestConnectRetryRescuesOneStalledRead(t *testing.T) {
	noRetryPause(t, time.Millisecond)
	runner := &scriptedRunner{fails: map[string][]string{"ls-remote": {connectStderr}}}
	var retries atomic.Int32
	resolution, err := resolveMain(WithConnectRetryCounter(t.Context(), &retries), runner, t)
	if err != nil || resolution.Commit != testCommit {
		t.Fatalf("resolution = %+v, %v", resolution, err)
	}
	if got := runner.count("ls-remote"); got != 2 || retries.Load() != 1 {
		t.Fatalf("ls-remote calls = %d, retries = %d; want 2 and 1", got, retries.Load())
	}
}

func TestConnectRetryHappensOnceAndKeepsTheClass(t *testing.T) {
	noRetryPause(t, time.Millisecond)
	runner := &scriptedRunner{fails: map[string][]string{"ls-remote": {connectStderr, recvStallStderr, connectStderr}}}
	_, err := resolveMain(t.Context(), runner, t)
	if !errors.Is(err, ErrGitConnect) || err.Error() != gitAcquisitionFailure("acme/repo", "main", "resolve Git ref").Error() {
		t.Fatalf("error = %v; want the unchanged message, connect-class", err)
	}
	if got := runner.count("ls-remote"); got != 2 {
		t.Fatalf("ls-remote calls = %d, want exactly 2", got)
	}
}

func TestConnectRetryNeverRetriesOtherFailures(t *testing.T) {
	noRetryPause(t, time.Millisecond)
	for name, stderr := range map[string]string{"auth": authStderr, "not found": notFoundStderr, "unknown": "fatal: something unexpected\n"} {
		runner := &scriptedRunner{fails: map[string][]string{"ls-remote": {stderr}}}
		_, err := resolveMain(t.Context(), runner, t)
		if err == nil || errors.Is(err, ErrGitConnect) || runner.count("ls-remote") != 1 {
			t.Fatalf("%s: err=%v calls=%d; want one attempt and no connect class", name, err, runner.count("ls-remote"))
		}
	}
}

func TestConnectRetryStopsOnCancelDuringPause(t *testing.T) {
	noRetryPause(t, 10*time.Second)
	ctx, cancel := context.WithCancel(t.Context())
	runner := &scriptedRunner{fails: map[string][]string{"ls-remote": {connectStderr}}}
	runner.onCall = func(verb string, n int) {
		if verb == "ls-remote" && n == 1 {
			time.AfterFunc(20*time.Millisecond, cancel)
		}
	}
	begin := time.Now()
	_, err := resolveMain(ctx, runner, t)
	// This ref path reports cancellation through its existing flattened
	// message; what matters is a prompt return without a second read.
	if err == nil || time.Since(begin) > 5*time.Second || runner.count("ls-remote") != 1 {
		t.Fatalf("err=%v after %v with %d calls; want prompt cancellation and no retry", err, time.Since(begin), runner.count("ls-remote"))
	}
}

func TestConnectRetryStaysInsideTheBudget(t *testing.T) {
	noRetryPause(t, connectRetryBudget)
	runner := &scriptedRunner{fails: map[string][]string{"ls-remote": {connectStderr}}}
	begin := time.Now()
	if _, err := resolveMain(t.Context(), runner, t); !errors.Is(err, ErrGitConnect) || runner.count("ls-remote") != 1 || time.Since(begin) > 10*time.Second {
		t.Fatalf("err=%v calls=%d after %v; a retry that cannot fit the budget must not even wait", err, runner.count("ls-remote"), time.Since(begin))
	}
}

// Concurrent exact-object fetches each retry their own stalled read at most
// once, in a fresh workspace.
func TestConnectRetryConcurrentFetchesRetryOnceEach(t *testing.T) {
	noRetryPause(t, time.Millisecond)
	runner := &scriptedRunner{fails: map[string][]string{}}
	objects := make([]string, 6)
	for i := range objects {
		objects[i] = fmt.Sprintf("%040x", i+1)
		runner.fails[objects[i]] = []string{connectStderr}
	}
	acquirer := Acquirer{Runner: runner, TempRoot: t.TempDir()}
	var wg sync.WaitGroup
	errs := make([]error, len(objects))
	for i, object := range objects {
		wg.Go(func() {
			_, errs[i] = acquirer.resolveCommit(t.Context(), "https://github.com/acme/repo.git", "acme/repo", object)
		})
	}
	wg.Wait()
	for i, object := range objects {
		if errs[i] != nil {
			t.Fatalf("object %d: %v", i, errs[i])
		}
		runner.mu.Lock()
		attempts := runner.objects[object]
		runner.mu.Unlock()
		if attempts != 2 {
			t.Fatalf("object %s fetched %d times, want 2", object, attempts)
		}
	}
}
