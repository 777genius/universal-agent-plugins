package commands

import (
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
)

const (
	devDebounce    = 250 * time.Millisecond
	devMaxCoalesce = time.Second
)

// devWindow keeps one quiet-period deadline and one hard coalescing deadline.
// Tree changes restart the quiet period; repeated identical failure codes do not.
type devWindow struct {
	set   bool
	first time.Time
	last  time.Time
}

func (w *devWindow) observe(now time.Time, restartQuietPeriod bool) {
	if !w.set {
		w.set = true
		w.first = now
		w.last = now
	} else if restartQuietPeriod {
		w.last = now
	}
}

func (w devWindow) ready(now time.Time) bool {
	return w.set && (now.Sub(w.last) >= devDebounce || now.Sub(w.first) >= devMaxCoalesce)
}

type devPending struct {
	devWindow
	project project.Result
}

func (pending *devPending) observe(next project.Result, now time.Time) {
	pending.project = next
	pending.devWindow.observe(now, true)
}

func (pending *devPending) take() project.Result {
	next := pending.project
	*pending = devPending{}
	return next
}

type devFailurePending struct {
	devWindow
	project project.Result
	err     error
	code    string
}

func (pending *devFailurePending) observe(next project.Result, err error, code string, now time.Time) {
	pending.project = next
	pending.err = err
	pending.devWindow.observe(now, pending.code != code)
	pending.code = code
}

func (pending *devFailurePending) take() (project.Result, string, error) {
	next, err, code := pending.project, pending.err, pending.code
	*pending = devFailurePending{}
	return next, code, err
}
