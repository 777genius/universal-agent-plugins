package shared

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
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
