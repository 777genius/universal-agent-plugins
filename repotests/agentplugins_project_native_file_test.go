package pluginkitairepo_test

import (
	"fmt"
	"io"
	"os"
)

func readProjectNativeSnapshot(path string, before os.FileInfo, limit int64) ([]byte, error) {
	if limit < 0 || !before.Mode().IsRegular() || before.Size() < 0 || before.Size() > limit {
		return nil, fmt.Errorf("snapshot pre-read type/size refused")
	}
	file, err := openProjectNativeSnapshot(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	same := func(info os.FileInfo) bool {
		return info != nil && info.Mode().IsRegular() && os.SameFile(before, info) && info.Mode() == before.Mode() && info.Size() == before.Size() && info.ModTime() == before.ModTime()
	}
	opened, err := file.Stat()
	if err != nil || !same(opened) {
		return nil, fmt.Errorf("snapshot opened identity differs: %v", err)
	}
	body, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	after, err := file.Stat()
	if err != nil || len(body) > int(limit) || int64(len(body)) != before.Size() || !same(after) {
		return nil, fmt.Errorf("snapshot bytes/held identity changed: %v", err)
	}
	current, err := os.Lstat(path)
	if err != nil || !same(current) {
		return nil, fmt.Errorf("snapshot path identity changed: %v", err)
	}
	return body, nil
}
