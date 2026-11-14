package fs

import (
	"context"
	"os"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type OS struct{}

func (OS) ReadFile(_ context.Context, path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (OS) WriteFileAtomic(_ context.Context, path string, data []byte, mode uint32) error {
	return atomicfile.Write(path, data, os.FileMode(mode))
}

func (OS) MkdirAll(_ context.Context, path string, mode uint32) error {
	return os.MkdirAll(path, os.FileMode(mode))
}

func (OS) Stat(_ context.Context, path string) (info ports.PathInfo, err error) {
	st, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return info, nil
		}
		return info, err
	}
	info.Exists = true
	info.IsDir = st.IsDir()
	return info, nil
}

func (OS) Remove(_ context.Context, path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
