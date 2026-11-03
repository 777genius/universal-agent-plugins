package management

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"path/filepath"
	"sync"
	"time"
)

type Options struct {
	Backend                                             Backend
	Store                                               ObservationStore
	Lock                                                MutationLock
	ContextID, EnvironmentID, IntentRoot, EngineVersion string
	PlanLifetime                                        time.Duration
}

type planEntry struct {
	localSkill      *LocalSkillIdentity
	review          Plan
	prepared        Prepared
	operationID     string
	request         ChangeRequest
	acceptedRequest string
}

type runningOperation struct {
	record Observation
	ctx    context.Context
	cancel context.CancelFunc
}

// Application owns one captured environment and its retained authority. Durable
// observations correlate existing work; they are never executable jobs.
type Application struct {
	opts              Options
	ctx               context.Context
	cancel            context.CancelFunc
	mu                sync.Mutex
	closed, preparing bool
	plans             map[string]*planEntry
	running           map[string]*runningOperation
	workers           sync.WaitGroup
	closeOnce         sync.Once
	done              chan struct{}
}

// NewApplication takes backend ownership only on success. Construction starts
// no installation, persistence or native work.
func NewApplication(ctx context.Context, opts Options) (*Application, error) {
	if opts.Backend == nil || opts.Store == nil || opts.Lock == nil || opts.ContextID == "" || opts.EnvironmentID == "" || !filepath.IsAbs(opts.IntentRoot) {
		return nil, errors.New("management requires a backend, observation store, mutation lock and captured absolute intent root")
	}
	if opts.PlanLifetime <= 0 {
		opts.PlanLifetime = 5 * time.Minute
	}
	appCtx, cancel := context.WithCancel(ctx)
	app := &Application{opts: opts, ctx: appCtx, cancel: cancel, plans: map[string]*planEntry{}, running: map[string]*runningOperation{}, done: make(chan struct{})}
	go app.expiryLoop()
	return app, nil
}

// Bootstrap supplies captured identifiers; the transport supplies its origin
// and private capability token.
func (a *Application) Bootstrap() Bootstrap {
	return Bootstrap{ApiVersion: N1, ContextId: a.opts.ContextID, EngineVersion: a.opts.EngineVersion}
}
func (a *Application) Stopping() <-chan struct{} { return a.ctx.Done() }
func (a *Application) Done() <-chan struct{}     { return a.done }

// Close closes admission before waiting, so calls and execution can safely share
// a wait group. Accepted work and unaccepted prepared objects have one owner.
func (a *Application) Close() {
	a.closeOnce.Do(func() {
		a.mu.Lock()
		a.closed = true
		a.mu.Unlock()
		a.cancel()
		a.workers.Wait()
		a.mu.Lock()
		var pending []Prepared
		for _, plan := range a.plans {
			if plan.acceptedRequest == "" {
				pending = append(pending, plan.prepared)
			}
		}
		a.plans = map[string]*planEntry{}
		a.mu.Unlock()
		for _, prepared := range pending {
			prepared.Close()
		}
		if closer, ok := a.opts.Backend.(interface{ Close() }); ok {
			closer.Close()
		}
		close(a.done)
	})
}

func (a *Application) admit(ctx context.Context) (context.Context, func(), error) {
	a.mu.Lock()
	if a.closed || a.ctx.Err() != nil {
		a.mu.Unlock()
		return nil, nil, refusal(410, "SESSION_EXPIRED", "This management process is shutting down", false)
	}
	a.workers.Add(1)
	a.mu.Unlock()
	callCtx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(a.ctx, cancel)
	return callCtx, func() { stop(); cancel(); a.workers.Done() }, nil
}

func (a *Application) expiryLoop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-a.ctx.Done():
			go a.Close()
			return
		case now := <-ticker.C:
			a.mu.Lock()
			if !a.closed && !a.preparing {
				for id, plan := range a.plans {
					if plan.acceptedRequest == "" && !now.Before(plan.review.ExpiresAt) {
						plan.prepared.Close()
						delete(a.plans, id)
					}
				}
			}
			a.mu.Unlock()
		}
	}
}

func operationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, 2*time.Minute)
}
func randomID() string {
	var value [32]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(value[:])
}
func refusal(status int, code, message string, retryable bool) error {
	return &Refusal{Status: status, Code: code, Message: message, Retryable: retryable}
}
