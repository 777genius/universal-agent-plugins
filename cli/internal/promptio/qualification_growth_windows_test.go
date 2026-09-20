//go:build windows

package promptio

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const qualificationThreadCap = 64

func qualificationThreadIDs(t *testing.T) map[uint32]bool {
	t.Helper()
	h, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		t.Fatal(err)
	}
	ids := make(map[uint32]bool)
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	err = windows.Thread32First(h, &entry)
	for err == nil {
		if entry.OwnerProcessID == windows.GetCurrentProcessId() {
			ids[entry.ThreadID] = true
		}
		err = windows.Thread32Next(h, &entry)
	}
	closeErr := windows.CloseHandle(h)
	if closeErr != nil {
		t.Fatalf("thread snapshot close: %v", closeErr)
	}
	if err != windows.ERROR_NO_MORE_FILES {
		t.Fatalf("thread snapshot enumeration: %v", err)
	}
	return ids
}

func qualificationRawResources(t *testing.T) [2]uint32 {
	t.Helper()
	var handles uint32
	ok, _, err := qualificationGetProcessHandleCount.Call(uintptr(windows.CurrentProcess()), uintptr(unsafe.Pointer(&handles)))
	if ok == 0 {
		t.Fatal(err)
	}
	return [2]uint32{handles, uint32(runtime.NumGoroutine())}
}

// A fixed cohort, preallocated before the baseline. No retries, rebasing,
// thread-to-handle subtraction, or skipped fixture conditions.
type qualificationGrowth struct {
	entered chan uint32
	release chan struct{}
	joined  chan struct{}
	once    sync.Once
}

func (g *qualificationGrowth) start(t *testing.T, before map[uint32]bool) {
	t.Helper()
	count := len(before) + 1
	if count > qualificationThreadCap {
		t.Fatalf("unmet fixture condition: cohort=%d cap=%d", count, qualificationThreadCap)
	}
	var wg sync.WaitGroup
	wg.Add(count)
	for i := 0; i < count; i++ {
		go func() {
			runtime.LockOSThread()
			g.entered <- windows.GetCurrentThreadId()
			<-g.release
			runtime.UnlockOSThread()
			wg.Done()
		}()
	}
	go func() { wg.Wait(); close(g.joined) }()
	t.Cleanup(func() { g.stop(t) })
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	observed := map[uint32]bool{}
	newThreads := 0
	for i := 0; i < count; i++ {
		select {
		case id := <-g.entered:
			if observed[id] {
				t.Fatal("duplicate locked native TID")
			}
			observed[id] = true
			if !before[id] {
				newThreads++
			}
		case <-deadline.C:
			t.Fatal("unmet fixture condition: locked cohort did not enter in two seconds")
		}
	}
	if newThreads == 0 {
		t.Fatal("unmet fixture condition: no observed native thread growth")
	}
	t.Logf("growth baseline_tids=%v cohort_tids=%v new=%d cap=%d", before, observed, newThreads, qualificationThreadCap)
}
func (g *qualificationGrowth) stop(t *testing.T) {
	t.Helper()
	g.once.Do(func() { close(g.release) })
	select {
	case <-g.joined:
	case <-time.After(2 * time.Second):
		t.Fatal("runtime helpers did not unlock and join")
	}
}

// The original ceiling and stability predicate, with its original three-second
// rule. Evaluate only after every growth helper has unlocked and joined.
func qualificationOldOracleRejects(t *testing.T, baseline [2]uint32) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	last := [2]uint32{}
	stable := 0
	samples, excess := 0, 0
	for {
		got := qualificationRawResources(t)
		samples++
		if got[0] > baseline[0] {
			excess++
		}
		if got == last && got[0] <= baseline[0] && got[1] <= baseline[1] {
			stable++
		} else {
			stable = 0
		}
		if stable == 3 {
			t.Fatalf("unmet fixture condition: old global ceiling accepted after join: baseline=%v actual=%v", baseline, got)
		}
		if time.Now().After(deadline) {
			if got[1] > baseline[1] || got[0] <= baseline[0] || excess != samples {
				t.Fatalf("unmet fixture condition: persistent handle excess with settled goroutines not demonstrated: baseline=%v final=%v excess=%d/%d", baseline, got, excess, samples)
			}
			t.Logf("old ceiling rejected after joined helpers: baseline=%v final=%v raw_delta=%d samples=%d", baseline, got, int64(got[0])-int64(baseline[0]), samples)
			return
		}
		last = got
		time.Sleep(20 * time.Millisecond)
	}
}

// Explicit native entry point: UAP_WINDOWS_RESOURCE_ORACLE=1 promptio.test.exe
// -test.run=^TestQualificationNativeResourceOracle$ -test.v -test.timeout=180s
// Run in a disposable attached Windows console. Each fixed case is a fresh
// process; logs go to disk, console output is bounded, and a failed fixture
// is fatal, never retried.
func TestQualificationNativeResourceOracle(t *testing.T) {
	if os.Getenv("UAP_WINDOWS_RESOURCE_ORACLE") != "1" {
		return
	} // Explicit opt-in, not a native success claim.
	if scenario := os.Getenv("UAP_WINDOWS_RESOURCE_ORACLE_CHILD"); scenario != "" {
		qualificationNativeOracleChild(t, scenario)
		return
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cases := []string{"runtime-only/growth", "balanced/plain", "balanced/growth", "console-retained/plain", "console-retained/growth", "thread-retained/plain", "thread-retained/growth", "close-failure/plain", "close-failure/growth", "branches/plain"}
	for _, scenario := range cases {
		t.Run(strings.ReplaceAll(scenario, "/", "-"), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, executable, "-test.run=^TestQualificationNativeResourceOracle$", "-test.v", "-test.timeout=14s")
			cmd.Env = append(os.Environ(), "UAP_WINDOWS_RESOURCE_ORACLE_CHILD="+scenario)
			cmd.Stdin = os.Stdin // Inherit console attachment; no controller or profile.
			logDir := os.Getenv("UAP_WINDOWS_RESOURCE_ORACLE_LOG_DIR")
			if logDir == "" {
				logDir = os.TempDir()
			}
			log, err := os.CreateTemp(logDir, "resource-oracle-"+strings.ReplaceAll(scenario, "/", "-")+"-*.log")
			if err != nil {
				t.Fatal(err)
			}
			cmd.Stdout, cmd.Stderr = log, log
			runErr := cmd.Run()
			closeErr := log.Close()
			t.Logf("native child %s log=%s", scenario, log.Name())
			if closeErr != nil {
				t.Fatalf("native log close: %v", closeErr)
			}
			if runErr != nil {
				// Long native census logs stay on disk; failure output is bounded to 6KiB.
				f, err := os.Open(log.Name())
				if err != nil {
					t.Fatal(err)
				}
				info, statErr := f.Stat()
				var tail []byte
				if statErr == nil {
					tail = make([]byte, min(info.Size(), 6<<10))
					_, err = f.ReadAt(tail, info.Size()-int64(len(tail)))
				}
				closeErr := f.Close()
				t.Fatalf("native child %s: %v\n%s\nlog read/close: %v %v %v", scenario, runErr, tail, statErr, err, closeErr)
			}
		})
		if t.Failed() {
			return
		} // Investigate the first failure before any additional run.
	}
	t.Log("QUALIFICATION_NATIVE_RESOURCE_ORACLE_OK")
}

func qualificationNativeOracleChild(t *testing.T, scenario string) {
	oldGC := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(oldGC)
	g := &qualificationGrowth{entered: make(chan uint32, qualificationThreadCap), release: make(chan struct{}), joined: make(chan struct{})}
	q := new(qualificationHandleTrace) // Allocate trace/barriers before resource baseline.
	readEntered, readRelease := make(chan struct{}), make(chan struct{})
	requestJoined := make(chan struct{})
	requestDone := make(chan struct {
		line string
		err  error
	}, 1)
	var releaseOnce sync.Once
	releaseRead := func() { releaseOnce.Do(func() { close(readRelease) }) }
	defer releaseRead()
	control, growth, ok := strings.Cut(scenario, "/")
	if !ok {
		t.Fatal("invalid child scenario")
	}
	var f *os.File
	var originalMode uint32
	if control != "runtime-only" {
		f = qualificationConsoleFixture(t)
		if err := windows.GetConsoleMode(windows.Handle(f.Fd()), &originalMode); err != nil {
			t.Fatal(err)
		}
	}
	if control == "branches" {
		qualificationConsoleBranches(t, f)
		return
	}
	// Warm diagnostics and close snapshot handles before measuring resources.
	for _, proc := range []*windows.LazyProc{qualificationWriteConsoleInput, qualificationHandleInformation, cancelSynchronousIO, qualificationPeekOracleInput, qualificationReadOracleInput} {
		if err := proc.Find(); err != nil {
			t.Fatal(err)
		}
	}
	qualificationNativeCensus(t)
	var warmFlags uint32
	if f != nil {
		if err := qualificationGetHandleInformation(windows.Handle(f.Fd()), &warmFlags); err != nil {
			t.Fatal(err)
		}
	}
	before := qualificationThreadIDs(t)
	baseline := qualificationConsoleResources(t, nil)
	if growth == "growth" && control == "runtime-only" {
		g.start(t, before)
	}
	if control != "runtime-only" {
		ctx, cancel := context.WithTimeout(q.context(context.Background()), 2*time.Second)
		defer cancel()
		ops := ctx.Value(cancelWindowsKey{}).(*cancelWindowsOps)
		// The matrix uses an actual ReadConsole request with a queued record.
		qualificationQueueOracleLine(t, windows.Handle(f.Fd()), "balanced")
		var console, thread, retained windows.Handle
		protected := false
		observe := ops.observe
		ops.observe = func(e cancelWindowsEvent) {
			if e.operation == "console-acquire" && e.err == nil {
				console = e.handle
			}
			if e.operation == "thread-acquire" && e.err == nil {
				thread = e.handle
			}
			// A retained close is recorded with its real failure result, never success.
			observe(e)
			if e.operation == "read-enter" && growth == "growth" {
				close(readEntered)
				<-readRelease
			}
		}
		// Registered before requesting ownership so every acquired retained handle
		// is cleaned even when a later assertion fails. This runs AFTER verdict.
		t.Cleanup(func() {
			if retained == 0 {
				return
			}
			if protected {
				if err := windows.SetHandleInformation(retained, qualificationProtectFromClose, 0); err != nil {
					t.Errorf("clear close protection: %v", err)
				}
			}
			if err := windows.CloseHandle(retained); err != nil {
				t.Errorf("retained handle cleanup: %v", err)
			}
		})
		ops.close = func(h windows.Handle) error {
			retain := (control == "console-retained" && h == console) || (control == "thread-retained" && h == thread)
			if retain {
				retained = h
				return windows.ERROR_BUSY
			}
			if control == "close-failure" && h == thread {
				retained = h
				if err := windows.SetHandleInformation(h, qualificationProtectFromClose, qualificationProtectFromClose); err != nil {
					return err
				}
				protected = true
				err := windows.CloseHandle(h) // Genuine native close failure, not a fabricated result.
				if err == nil {
					protected = false
					retained = 0
				}
				return err
			}
			return windows.CloseHandle(h)
		}
		go func() {
			defer close(requestJoined)
			line, err := ReadLine(ctx, f)
			requestDone <- struct {
				line string
				err  error
			}{line, err}
		}()
		defer func() {
			releaseRead()
			cancel()
			select {
			case <-requestJoined:
			case <-time.After(2 * time.Second):
				panic("oracle request failed guaranteed join")
			}
		}()
		if growth == "growth" {
			select {
			case <-readEntered:
			case <-time.After(2 * time.Second):
				t.Fatal("prompt did not enter before runtime growth")
			}
			g.start(t, before)
			releaseRead()
		}
		var line string
		var err error
		select {
		case result := <-requestDone:
			line, err = result.line, result.err
		case <-time.After(2 * time.Second):
			t.Fatal("matrix request did not join in two seconds")
		}
		if err != nil || line != "balanced" {
			t.Fatalf("balanced request: %q %v", line, err)
		}
		qualificationAccountOracleEnter(t, windows.Handle(f.Fd()))
		cancel()
		if growth == "growth" {
			g.stop(t)
		}
		// Retention remains real through settling, verdict, validity check and census.
		qualificationConsoleResources(t, &baseline)
		verdict := q.verdict()
		if control == "balanced" {
			if verdict != nil {
				t.Fatal(verdict)
			}
			qualificationRequireOps(t, q, "console-acquire", "worker-start", "thread-acquire", "read-complete", "thread-close", "join", "console-close", "request-end")
			qualificationMutations(t, q)
		} else {
			want := "thread-close failed"
			if control == "console-retained" {
				want = "console-close failed"
			}
			if verdict == nil || !strings.Contains(verdict.Error(), want) {
				t.Fatalf("negative verdict=%v want=%s", verdict, want)
			}
			if retained == 0 {
				t.Fatal("negative control did not retain a real handle")
			}
			var flags uint32
			if err := qualificationGetHandleInformation(retained, &flags); err != nil {
				t.Fatalf("retained handle invalid through verdict: %v", err)
			}
			if control == "close-failure" && (!protected || flags&qualificationProtectFromClose == 0) {
				t.Fatal("genuine protected-close fixture not established")
			}
			t.Logf("negative control=%s retained=%#x valid through verdict=%v", control, retained, verdict)
		}
		var mode uint32
		if err := windows.GetConsoleMode(windows.Handle(f.Fd()), &mode); err != nil || mode != originalMode {
			t.Fatalf("borrowed input invalid or changed: mode=%#x want=%#x err=%v", mode, originalMode, err)
		}
	} else {
		g.stop(t)
		if q.count != 0 || q.generation != 0 {
			t.Fatal("runtime-only control acquired prompt resources")
		}
		// Empty traces intentionally FAIL the prompt verdict. This control only
		// proves that the process-wide oracle rejects without any prompt request.
		if q.verdict() == nil {
			t.Fatal("empty trace accepted")
		}
		qualificationConsoleResources(t, &baseline)
	}
	if growth == "growth" && (control == "balanced" || control == "runtime-only") {
		qualificationOldOracleRejects(t, baseline)
	}
	after := qualificationThreadIDs(t)
	final := qualificationRawResources(t)
	t.Logf("scenario=%s raw baseline=%v final=%v delta=%d native_before=%v native_after=%v", scenario, baseline, final, int64(final[0])-int64(baseline[0]), before, after)
}

func qualificationQueueOracleLine(t *testing.T, h windows.Handle, line string) {
	t.Helper()
	var mode, queued uint32
	if err := windows.GetConsoleMode(h, &mode); err != nil || mode&windows.ENABLE_LINE_INPUT == 0 {
		t.Fatalf("unmet fixture condition: cooked console required: mode=%#x err=%v", mode, err)
	}
	if err := windows.GetNumberOfConsoleInputEvents(h, &queued); err != nil || queued != 0 {
		t.Fatalf("unmet fixture condition: empty disposable input required: queued=%d err=%v", queued, err)
	}
	var records []qualificationKeyEvent
	for _, char := range line + "\r" {
		e := qualificationKeyEvent{eventType: 1, keyDown: 1, repeat: 1, char: uint16(char)}
		if char == '\r' {
			e.virtualKey = 0x0d
		}
		records = append(records, e)
		e.keyDown = 0
		records = append(records, e)
	}
	var written uint32
	ok, _, err := qualificationWriteConsoleInput.Call(uintptr(h), uintptr(unsafe.Pointer(&records[0])), uintptr(len(records)), uintptr(unsafe.Pointer(&written)))
	if ok == 0 || written != uint32(len(records)) {
		t.Fatalf("queue oracle line: %d/%d %v", written, len(records), err)
	}
}

// Only the exact key-up emitted after this fixture's terminating Enter may
// survive ReadConsole. Never flush the shared queue: inherited owners reuse it.
func qualificationOracleResidual(records []qualificationKeyEvent) error {
	if len(records) == 0 {
		return nil
	}
	want := qualificationKeyEvent{eventType: 1, repeat: 1, virtualKey: 0x0d, char: '\r'}
	if len(records) != 1 || records[0] != want {
		return fmt.Errorf("unexpected residual console input: %+v", records)
	}
	return nil
}

var (
	qualificationPeekOracleInput = windows.NewLazySystemDLL("kernel32.dll").NewProc("PeekConsoleInputW")
	qualificationReadOracleInput = windows.NewLazySystemDLL("kernel32.dll").NewProc("ReadConsoleInputW")
)

func qualificationAccountOracleEnter(t *testing.T, h windows.Handle) {
	t.Helper()
	var records [2]qualificationKeyEvent
	var count uint32
	ok, _, err := qualificationPeekOracleInput.Call(uintptr(h), uintptr(unsafe.Pointer(&records[0])), uintptr(len(records)), uintptr(unsafe.Pointer(&count)))
	if ok == 0 {
		t.Fatalf("peek residual input: %v", err)
	}
	if err := qualificationOracleResidual(records[:count]); err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		return
	}
	ok, _, err = qualificationReadOracleInput.Call(uintptr(h), uintptr(unsafe.Pointer(&records[0])), 1, uintptr(unsafe.Pointer(&count)))
	if ok == 0 || count != 1 {
		t.Fatalf("account fixture Enter key-up: count=%d err=%v", count, err)
	}
	if err := qualificationOracleResidual(records[:count]); err != nil {
		t.Fatal(err)
	}
	var queued uint32
	if err := windows.GetNumberOfConsoleInputEvents(h, &queued); err != nil || queued != 0 {
		t.Fatalf("unexpected input after fixture Enter key-up: queued=%d err=%v", queued, err)
	}
}

func TestQualificationOracleResidualInput(t *testing.T) {
	enter := qualificationKeyEvent{eventType: 1, repeat: 1, virtualKey: 0x0d, char: '\r'}
	for _, records := range [][]qualificationKeyEvent{nil, {enter}} {
		if err := qualificationOracleResidual(records); err != nil {
			t.Fatal(err)
		}
	}
	for _, records := range [][]qualificationKeyEvent{{enter, enter}, {{eventType: 2}}, {{eventType: 1, keyDown: 1, repeat: 1, virtualKey: 0x0d, char: '\r'}}} {
		if qualificationOracleResidual(records) == nil {
			t.Fatalf("accepted unexpected residual: %+v", records)
		}
	}
	for field := 0; field < 8; field++ {
		unexpected := enter
		switch field {
		case 0:
			unexpected.eventType++
		case 1:
			unexpected.padding++
		case 2:
			unexpected.keyDown++
		case 3:
			unexpected.repeat++
		case 4:
			unexpected.virtualKey++
		case 5:
			unexpected.scan++
		case 6:
			unexpected.char++
		case 7:
			unexpected.control++
		}
		if qualificationOracleResidual([]qualificationKeyEvent{unexpected}) == nil {
			t.Fatalf("accepted changed fixture field %d", field)
		}
	}
}
