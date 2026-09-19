package providers

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/cline"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/gemini"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
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
	err := cline.ApplyClineNativeMutationWithKernelRenameAndCapacity(configRoot, activePath, nil, desired, nativeconfig.NewWithFileIO(files), func(oldPath, newPath string) error {
		renameCalls++
		return os.Rename(oldPath, newPath)
	}, func(left, right int) (int, error) {
		capacityCalls++
		if left != 0 || right != 2 {
			t.Fatalf("capacity inputs = (%d, %d), want (0, 2)", left, right)
		}
		return 0, shared.ErrCombinedCapacityOverflow
	})
	if !errors.Is(err, shared.ErrCombinedCapacityOverflow) {
		t.Fatalf("error = %v, want %v", err, shared.ErrCombinedCapacityOverflow)
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
	err := gemini.ApplyGeminiNativeMutationWithKernelRenameAndCapacity(configRoot, activePath, nil, desired, nativeconfig.NewWithFileIO(files), func(oldPath, newPath string) error {
		renameCalls++
		return os.Rename(oldPath, newPath)
	}, func(left, right int) (int, error) {
		capacityCalls++
		if left != 0 || right != 2 {
			t.Fatalf("capacity inputs = (%d, %d), want (0, 2)", left, right)
		}
		return 0, shared.ErrCombinedCapacityOverflow
	})
	if !errors.Is(err, shared.ErrCombinedCapacityOverflow) {
		t.Fatalf("error = %v, want %v", err, shared.ErrCombinedCapacityOverflow)
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
