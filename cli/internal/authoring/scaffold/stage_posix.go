//go:build !windows

package scaffold

import (
	"io"
	"os"
)

func makePrivateStage(parent *os.Root, name string) error {
	return parent.Mkdir(name, 0700)
}

func makePrivateSkillStage(parent *os.Root, name string) (io.Closer, error) {
	if err := makePrivateStage(parent, name); err != nil {
		return nil, err
	}
	return skillStageGuard{}, nil
}

type skillStageGuard struct{}

func (skillStageGuard) Close() error { return nil }
