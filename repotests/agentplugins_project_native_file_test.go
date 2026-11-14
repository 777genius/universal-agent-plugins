package pluginkitairepo_test

import (
	"errors"
	"fmt"
	"io"
	"os"
)

func readProjectNativeSnapshot(path string, before os.FileInfo, limit int64) (result []byte, err error) {
	if limit < 0 || !before.Mode().IsRegular() || before.Size() < 0 || before.Size() > limit {
		return nil, fmt.Errorf("snapshot pre-read type/size refused")
	}
	file, err := openProjectNativeSnapshot(path)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	same := func(info os.FileInfo) bool {
		return info != nil && info.Mode().IsRegular() && os.SameFile(before, info) && info.Mode() == before.Mode() && info.Size() == before.Size() && info.ModTime().Equal(before.ModTime())
	}
	opened, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("snapshot opened identity: %w", err)
	}
	if !same(opened) {
		return nil, fmt.Errorf("snapshot opened identity differs")
	}
	body, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	after, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("snapshot held identity: %w", err)
	}
	if len(body) > int(limit) || int64(len(body)) != before.Size() || !same(after) {
		return nil, fmt.Errorf("snapshot bytes/held identity changed")
	}
	current, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("snapshot path identity: %w", err)
	}
	if !same(current) {
		return nil, fmt.Errorf("snapshot path identity changed")
	}
	return body, nil
}
