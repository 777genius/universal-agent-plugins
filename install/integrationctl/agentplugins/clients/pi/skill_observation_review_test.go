package pi

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Trust bytes must go through the injected bounded reader before parsing.
// A late read can complete with changed bytes or be interrupted without them.
type piReviewTrustFiles struct {
	piReadOnlySkillFiles
	after      *bool
	target     string
	cancel     context.CancelFunc
	interrupt  bool
	semantic   error
	finalReads int
	trustReads int
	unbounded  int
}

func (f *piReviewTrustFiles) ReadNoFollow(string) ([]byte, os.FileMode, bool, error) {
	f.unbounded++
	return nil, 0, false, errors.New("unexpected unbounded trust read")
}

func (f *piReviewTrustFiles) ReadNoFollowBounded(path string, limit int64) ([]byte, os.FileMode, bool, error) {
	if filepath.Base(path) == piTrustHookEntry || filepath.Base(path) == piTrustProjectsName {
		f.trustReads++
		if limit != piTrustDocumentLimit {
			return nil, 0, false, errors.New("wrong trust read bound")
		}
	}
	after := f.after != nil && *f.after
	if after && (filepath.Base(path) == "trust.json" || filepath.Base(path) == piTrustProjectsName) {
		f.finalReads++
	}
	if path == f.target && (f.after == nil || after) {
		if f.cancel != nil {
			f.cancel()
		}
		if f.interrupt {
			return nil, 0, false, context.Canceled
		}
		if f.semantic != nil {
			return nil, 0, false, f.semantic
		}
		return []byte("{}"), 0600, true, nil
	}
	v, err := nativeconfig.New().ReadExactFileBounded(path, limit)
	return v.Body, v.Mode, v.Exists, err
}

func TestPiSkillsInitialSemanticCaptureFailureIsNotSuccess(t *testing.T) {
	for _, scope := range []domain.InstallScope{domain.ScopeUser, domain.ScopeProject} {
		for _, canceled := range []bool{false, true} {
			t.Run(string(scope)+map[bool]string{false: "/live", true: "/canceled"}[canceled], func(t *testing.T) {
				request, env, runner := piSkillFixture(t, scope)
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				refusal := errors.New("unsafe selected trust file")
				files := &piReviewTrustFiles{target: filepath.Join(request.Client.ConfigRoot, "settings.json"), semantic: refusal}
				if canceled {
					files.cancel = cancel
				}
				env.NativeConfig = nativeconfig.NewWithFileIO(files)
				got, err := completePiSkills(ctx, env, request, piSkillOutcome(request))
				if !errors.Is(err, refusal) || got.Activation != domain.ActivationFailed || got.Verification != domain.VerificationFailed || got.NativeEffect != domain.NativeEffectUncertain || runner.calls != 0 {
					t.Fatalf("initial semantic failure lost: %+v %v calls%d", got, err, runner.calls)
				}
				if canceled && !errors.Is(err, context.Canceled) {
					t.Fatalf("cancellation lost: %v", err)
				}
			})
		}
	}
}

func TestPiSkillsCompletedMismatchAndLateCancellationRemainFailed(t *testing.T) {
	for _, scope := range []domain.InstallScope{domain.ScopeUser, domain.ScopeProject} {
		for _, interrupted := range []bool{false, true} {
			t.Run(string(scope)+map[bool]string{false: "/completed", true: "/interrupted"}[interrupted], func(t *testing.T) {
				request, env, runner := piSkillFixture(t, scope)
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				after := false
				target := filepath.Join(request.Client.ConfigRoot, "trust.json")
				if scope == domain.ScopeProject {
					_, _, target = piTrustHookPaths(request.Client.ConfigRoot)
				}
				files := &piReviewTrustFiles{after: &after, target: target, cancel: cancel, interrupt: interrupted}
				// Preserve valid Project trust bytes while changing their exact snapshot.
				if scope == domain.ScopeProject && !interrupted {
					files.target = filepath.Join(request.Client.ConfigRoot, "trust.json")
				}
				env.NativeConfig = nativeconfig.NewWithFileIO(files)
				runner.after = func() { after = true }
				out := piSkillOutcome(request)
				got, err := completePiSkills(ctx, env, request, out)
				if !errors.Is(err, context.Canceled) || runner.calls != 1 || files.finalReads == 0 {
					t.Fatalf("late cancellation not exercised: %+v %v reads%d", got, err, files.finalReads)
				}
				if interrupted {
					if !reflect.DeepEqual(got, out) {
						t.Fatalf("interrupted proof invented mutation: %+v", got)
					}
				} else if got.Activation != domain.ActivationFailed || got.NativeEffect != domain.NativeEffectUncertain {
					t.Fatalf("completed mismatch lost: %+v %v", got, err)
				}
			})
		}
	}
}

func TestPiSkillsProjectTrustUsesOnlyBoundedCapturedBytes(t *testing.T) {
	request, env, runner := piSkillFixture(t, domain.ScopeProject)
	files := &piReviewTrustFiles{}
	env.NativeConfig = nativeconfig.NewWithFileIO(files)
	got, err := completePiSkills(t.Context(), env, request, piSkillOutcome(request))
	if err != nil || got.Activation != domain.ActivationActive || runner.calls != 1 || files.unbounded != 0 || files.trustReads != 4 {
		t.Fatalf("trust snapshots not bounded or retained: %+v %v unbounded%d trust%d", got, err, files.unbounded, files.trustReads)
	}
}

func TestPiSkillsInitialReadUncertaintyPreservesVerifiedBinding(t *testing.T) {
	for _, scope := range []domain.InstallScope{domain.ScopeUser, domain.ScopeProject} {
		t.Run(string(scope), func(t *testing.T) {
			request, env, runner := piSkillFixture(t, scope)
			target := filepath.Join(request.Client.ConfigRoot, "settings.json")
			env.NativeConfig = nativeconfig.NewWithFileIO(&piReviewTrustFiles{target: target, semantic: &os.PathError{Op: "read", Path: target, Err: syscall.EIO}})
			out := piSkillOutcome(request)
			got, err := completePiSkills(t.Context(), env, request, out)
			if err != nil || !reflect.DeepEqual(got, out) || runner.calls != 0 {
				t.Fatalf("read uncertainty changed binding: %+v %v", got, err)
			}
		})
	}
}

func TestPiSkillsProjectTrustOversizedFilesFailBeforeCatalog(t *testing.T) {
	for _, name := range []string{piTrustHookEntry, piTrustProjectsName} {
		t.Run(name, func(t *testing.T) {
			request, env, runner := piSkillFixture(t, domain.ScopeProject)
			directory, _, _ := piTrustHookPaths(request.Client.ConfigRoot)
			if err := os.WriteFile(filepath.Join(directory, name), bytes.Repeat([]byte("x"), piTrustDocumentLimit+1), 0600); err != nil {
				t.Fatal(err)
			}
			files := &piReviewTrustFiles{}
			env.NativeConfig = nativeconfig.NewWithFileIO(files)
			got, err := completePiSkills(t.Context(), env, request, piSkillOutcome(request))
			if err == nil || got.Activation != domain.ActivationFailed || got.NativeEffect != domain.NativeEffectUncertain || runner.calls != 0 || files.unbounded != 0 || files.trustReads == 0 {
				t.Fatalf("oversized trust reached catalog: %+v %v", got, err)
			}
		})
	}
}

func TestPiSkillsFinalSemanticCaptureFailureRetainsCancellation(t *testing.T) {
	for _, scope := range []domain.InstallScope{domain.ScopeUser, domain.ScopeProject} {
		t.Run(string(scope), func(t *testing.T) {
			request, env, runner := piSkillFixture(t, scope)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			after := false
			refusal := errors.New("unsafe selected trust file")
			files := &piReviewTrustFiles{after: &after, target: filepath.Join(request.Client.ConfigRoot, "settings.json"), cancel: cancel, semantic: refusal}
			env.NativeConfig = nativeconfig.NewWithFileIO(files)
			runner.after = func() { after = true }
			got, err := completePiSkills(ctx, env, request, piSkillOutcome(request))
			if !errors.Is(err, refusal) || !errors.Is(err, context.Canceled) || got.Activation != domain.ActivationFailed || got.Verification != domain.VerificationFailed || got.NativeEffect != domain.NativeEffectUncertain || runner.calls != 1 {
				t.Fatalf("final semantic failure or cancellation lost: %+v %v calls%d", got, err, runner.calls)
			}
		})
	}
}
