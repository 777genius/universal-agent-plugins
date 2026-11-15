package shared

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestProjectTrustMutationPreservesRefusalAndConcurrentClientWrite(t *testing.T) {
	for _, scenario := range []string{"refused", "concurrent", "unchanged"} {
		t.Run(scenario, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "trust.json")
			original := []byte(`{"foreign":true}`)
			if err := os.WriteFile(path, original, 0600); err != nil {
				t.Fatal(err)
			}
			refusal := errors.New("explicit trust refusal")
			changed, err := UpdateProjectTrustFile(nativeconfig.New(), path, func(body []byte) ([]byte, error) {
				if !bytes.Equal(body, original) {
					t.Fatal("mutation did not receive the locked predecessor")
				}
				switch scenario {
				case "refused":
					return nil, refusal
				case "concurrent":
					if err := os.WriteFile(path, []byte(`{"foreign":"changed by client"}`), 0600); err != nil {
						t.Fatal(err)
					}
					return []byte(`{"foreign":true,"project":"trusted"}`), nil
				default:
					return body, nil
				}
			})
			if changed || scenario == "refused" && !errors.Is(err, refusal) || scenario == "concurrent" && !errors.Is(err, nativeconfig.ErrConcurrentChange) || scenario == "unchanged" && err != nil {
				t.Fatalf("changed=%v err=%v", changed, err)
			}
			got, readErr := os.ReadFile(path)
			want := original
			if scenario == "concurrent" {
				want = []byte(`{"foreign":"changed by client"}`)
			}
			if readErr != nil || !bytes.Equal(got, want) {
				t.Fatalf("trust update replaced unrelated client changes: %q, %v", got, readErr)
			}
		})
	}
}

type projectTrustRollbackFiles struct {
	effect     domain.NativeEffectState
	writes     int
	applyErr   error
	restoreErr error
	foreign    []byte
}

func (*projectTrustRollbackFiles) ReadNoFollow(path string) ([]byte, os.FileMode, bool, error) {
	snapshot, err := nativeconfig.New().ReadExactFile(path)
	return snapshot.Body, snapshot.Mode, snapshot.Exists, err
}

func (files *projectTrustRollbackFiles) WriteAtomic(path string, body []byte, mode os.FileMode) error {
	files.writes++
	if files.writes == 1 {
		return errors.Join(os.WriteFile(path, body, mode), files.applyErr)
	}
	switch files.effect {
	case domain.NativeEffectUnchanged:
		return os.WriteFile(path, body, mode)
	case domain.NativeEffectCommitted:
		return files.restoreErr
	default:
		return errors.Join(os.WriteFile(path, files.foreign, mode), files.restoreErr)
	}
}

func (*projectTrustRollbackFiles) RemoveNoFollow(string) error {
	return errors.New("existing trust document must not be removed")
}

func TestProjectTrustMutationReportsEffectAfterRollback(t *testing.T) {
	original := []byte(`{"foreign":true}`)
	next := []byte(`{"foreign":true,"project":"trusted"}`)
	foreign := []byte(`{"foreign":"late native preference"}`)
	for _, test := range []struct {
		effect domain.NativeEffectState
		want   []byte
	}{
		{domain.NativeEffectUnchanged, original},
		{domain.NativeEffectCommitted, next},
		{domain.NativeEffectUncertain, foreign},
	} {
		t.Run(string(test.effect), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "trust.json")
			if err := os.WriteFile(path, original, 0600); err != nil {
				t.Fatal(err)
			}
			files := &projectTrustRollbackFiles{effect: test.effect, applyErr: errors.New("write became visible before failure"), restoreErr: errors.New("restore failed"), foreign: foreign}
			changed, err := UpdateProjectTrustFile(nativeconfig.NewWithFileIO(files), path, func([]byte) ([]byte, error) { return next, nil })
			var observed *NativeEffectError
			if !errors.As(err, &observed) || observed.Effect != test.effect || changed != (test.effect == domain.NativeEffectCommitted) || files.writes != 2 {
				t.Fatalf("lost post-rollback effect: changed=%v error=%v writes=%d", changed, err, files.writes)
			}
			if !errors.Is(err, files.applyErr) || errors.Is(err, files.restoreErr) != (test.effect != domain.NativeEffectUnchanged) {
				t.Fatalf("lost apply or rollback cause: %v", err)
			}
			if test.effect == domain.NativeEffectUncertain && !errors.Is(err, nativeconfig.ErrConcurrentChange) {
				t.Fatalf("late foreign write was not reported: %v", err)
			}
			body, readErr := os.ReadFile(path)
			if readErr != nil || !bytes.Equal(body, test.want) {
				t.Fatalf("wrong retained trust document: %q, %v", body, readErr)
			}
		})
	}
}
