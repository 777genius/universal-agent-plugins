package providers

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers/nativeconfig"
)

type countingNativeConfigFileIO struct{ calls int }

func (files *countingNativeConfigFileIO) ReadNoFollow(string) ([]byte, os.FileMode, bool, error) {
	files.calls++
	return nil, 0, false, nil
}

func (files *countingNativeConfigFileIO) WriteAtomic(string, []byte, os.FileMode) error {
	files.calls++
	return nil
}

func (files *countingNativeConfigFileIO) RemoveNoFollow(string) error {
	files.calls++
	return nil
}

func TestCheckedCombinedCapacity(t *testing.T) {
	tests := []struct {
		name    string
		left    int
		right   int
		want    int
		wantErr bool
	}{
		{name: "ordinary", left: 2, right: 3, want: 5},
		{name: "exact maximum", left: math.MaxInt - 1, right: 1, want: math.MaxInt},
		{name: "overflow", left: math.MaxInt, right: 1, wantErr: true},
		{name: "negative left", left: -1, right: 0, wantErr: true},
		{name: "negative right", left: 0, right: -1, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := checkedCombinedCapacity(test.left, test.right)
			if test.wantErr {
				if !errors.Is(err, errCombinedCapacityOverflow) {
					t.Fatalf("error = %v, want %v", err, errCombinedCapacityOverflow)
				}
				return
			}
			if err != nil {
				t.Fatalf("checked capacity: %v", err)
			}
			if got != test.want {
				t.Fatalf("capacity = %d, want %d", got, test.want)
			}
		})
	}
}

func TestClineCapacityFailurePrecedesFilesystemEffects(t *testing.T) {
	root := t.TempDir()
	configRoot := filepath.Join(root, ".cline")
	activePath := filepath.Join(root, "active")
	writeTestFile(t, filepath.Join(activePath, "skills", "docs", "SKILL.md"), "managed\n")
	server := nativeconfig.Server{Type: "stdio", Command: "node", Args: []string{filepath.Join(activePath, "server.js")}}
	writeClineProjectionFixture(t, activePath, map[string]nativeconfig.Server{"docs": server})
	desired := clineFixtureObjects(t, configRoot, activePath, "docs", "docs", server)
	files := &countingNativeConfigFileIO{}
	renameCalls := 0
	capacityCalls := 0
	err := applyClineNativeMutationWithKernelRenameAndCapacity(configRoot, activePath, nil, desired, nativeconfig.NewWithFileIO(files), func(oldPath, newPath string) error {
		renameCalls++
		return os.Rename(oldPath, newPath)
	}, func(left, right int) (int, error) {
		capacityCalls++
		if left != 0 || right != 2 {
			t.Fatalf("capacity inputs = (%d, %d), want (0, 2)", left, right)
		}
		return 0, errCombinedCapacityOverflow
	})
	if !errors.Is(err, errCombinedCapacityOverflow) {
		t.Fatalf("error = %v, want %v", err, errCombinedCapacityOverflow)
	}
	if renameCalls != 0 {
		t.Fatalf("rename calls = %d, want 0", renameCalls)
	}
	if capacityCalls != 1 {
		t.Fatalf("capacity calls = %d, want 1", capacityCalls)
	}
	if files.calls != 0 {
		t.Fatalf("native config file calls = %d, want 0", files.calls)
	}
	if _, statErr := os.Lstat(configRoot); !os.IsNotExist(statErr) {
		t.Fatalf("Cline config root was touched before capacity rejection: %v", statErr)
	}
}

func TestGeminiCapacityFailurePrecedesFilesystemEffects(t *testing.T) {
	root := t.TempDir()
	configRoot := filepath.Join(root, ".gemini")
	activePath, desired := geminiNativeFixture(t, configRoot, "managed", "https://docs.test")
	files := &countingNativeConfigFileIO{}
	renameCalls := 0
	capacityCalls := 0
	err := applyGeminiNativeMutationWithKernelRenameAndCapacity(configRoot, activePath, nil, desired, nativeconfig.NewWithFileIO(files), func(oldPath, newPath string) error {
		renameCalls++
		return os.Rename(oldPath, newPath)
	}, func(left, right int) (int, error) {
		capacityCalls++
		if left != 0 || right != 2 {
			t.Fatalf("capacity inputs = (%d, %d), want (0, 2)", left, right)
		}
		return 0, errCombinedCapacityOverflow
	})
	if !errors.Is(err, errCombinedCapacityOverflow) {
		t.Fatalf("error = %v, want %v", err, errCombinedCapacityOverflow)
	}
	if renameCalls != 0 {
		t.Fatalf("rename calls = %d, want 0", renameCalls)
	}
	if capacityCalls != 1 {
		t.Fatalf("capacity calls = %d, want 1", capacityCalls)
	}
	if files.calls != 0 {
		t.Fatalf("native config file calls = %d, want 0", files.calls)
	}
	if _, statErr := os.Lstat(configRoot); !os.IsNotExist(statErr) {
		t.Fatalf("Gemini config root was touched before capacity rejection: %v", statErr)
	}
}
