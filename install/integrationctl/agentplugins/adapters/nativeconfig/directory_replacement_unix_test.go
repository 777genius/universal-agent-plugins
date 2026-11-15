//go:build !windows

package nativeconfig

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Actual pathname replacement at read/lock/staging/publication boundaries must
// never move the writer or its compensation into an identical foreign root.
func TestDirectoryKernelRejectsIdenticalReplacementAtMutationBoundaries(t *testing.T) {
	for _, action := range []Action{ActionUpdate, ActionRemove} {
		for _, at := range []int{2, 4, 8, 9, 11} {
			t.Run(fmt.Sprintf("%s/%d", action, at), func(t *testing.T) {
				directory := filepath.Join(t.TempDir(), "selected")
				if err := os.Mkdir(directory, 0700); err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(directory, "config.yaml")
				req := Request{Paths: Paths{JSON: path}, Codec: CodecHermes, Name: "owned", Action: ActionAdd, Server: Server{Type: "remote", URL: "https://example.test/one"}}
				receipt, err := New().Apply(req)
				if err != nil {
					t.Fatal(err)
				}
				root, err := os.OpenRoot(directory)
				if err != nil {
					t.Fatal(err)
				}
				defer func() {
					if err := root.Close(); err != nil {
						t.Error(err)
					}
				}()
				original, _ := root.Stat(".")
				calls := 0
				var foreign []byte
				verify := func() error {
					calls++
					if calls == at {
						foreign, err = os.ReadFile(path)
						if err != nil {
							t.Fatal(err)
						}
						if err := os.Rename(directory, directory+"-original"); err != nil {
							t.Fatal(err)
						}
						if err := os.Mkdir(directory, 0700); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(path, foreign, 0600); err != nil {
							t.Fatal(err)
						}
					}
					named, err := os.Stat(directory)
					if err != nil || !os.SameFile(original, named) {
						return ErrConcurrentChange
					}
					return nil
				}
				kernel, err := New().WithDirectory(root, directory, verify)
				if err == nil {
					req.Action, req.Owned, req.Server.URL = action, &receipt, "https://example.test/two"
					_, err = kernel.Apply(req)
				}
				if !errors.Is(err, ErrConcurrentChange) || foreign == nil {
					t.Fatal("original directory substitution was not refused", err, calls)
				}
				body, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(body, foreign) {
					t.Fatal("writer or compensation changed replacement bytes", err)
				}
			})
		}
	}
}
