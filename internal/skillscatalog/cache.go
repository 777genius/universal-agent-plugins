package skillscatalog

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func cacheFilename(name string) bool {
	if len(name) != 69 || !strings.HasSuffix(name, ".json") {
		return false
	}
	_, err := hex.DecodeString(name[:64])
	return err == nil
}
func (s *Service) load() error {
	files, err := os.ReadDir(s.dir)
	if err != nil {
		return err
	}
	// This directory contains only our hex-key records; other files are untouched.
	for _, file := range files {
		if !cacheFilename(file.Name()) {
			continue
		}
		if e, ok := s.loadEntry(file); ok {
			s.entries[e.Key] = e
		}
	}
	s.trimLocked()
	return nil
}
func (s *Service) loadEntry(file os.DirEntry) (entry, bool) {
	path := filepath.Join(s.dir, file.Name())
	info, err := file.Info()
	if err != nil || !info.Mode().IsRegular() {
		return entry{}, false
	}
	if info.Size() > maxBodyBytes {
		_ = os.Remove(path)
		return entry{}, false
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return entry{}, false
	}
	var e entry
	if !validStoredJSON(b) || json.Unmarshal(b, &e) != nil || !s.validEntry(e, file.Name()[:64]) {
		_ = os.Remove(path)
		return entry{}, false
	}
	return e, true
}
func (s *Service) validEntry(e entry, key string) bool {
	return e.Key == key && e.Key == e.Request.key(s.provider.ID()) && validRequest(e.Request) && validPage(e.Result, e.Request, s.provider.ID()) && s.usable(e.Result, s.now())
}
func (s *Service) persist(e entry) bool {
	b, err := json.Marshal(e)
	if err != nil || len(b) > maxBodyBytes {
		return false
	}
	f, err := os.CreateTemp(s.dir, ".catalog-*")
	if err != nil {
		return false
	}
	name := f.Name()
	// Cleanup errors cannot invalidate a successful rename; all records remain
	// independently bounded and validated when read.
	defer func() { _ = os.Remove(name) }()
	if _, err = f.Write(b); err != nil {
		_ = f.Close()
		return false
	}
	if err = f.Sync(); err != nil {
		_ = f.Close()
		return false
	}
	if f.Close() != nil {
		return false
	}
	return os.Rename(name, filepath.Join(s.dir, e.Key+".json")) == nil
}
func (s *Service) trimLocked() {
	if len(s.entries) <= maxEntries {
		return
	}
	keys := make([]string, 0, len(s.entries))
	for key := range s.entries {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return s.entries[keys[i]].Result.Cache.FetchedAt.Before(s.entries[keys[j]].Result.Cache.FetchedAt)
	})
	for _, key := range keys[:len(keys)-maxEntries] {
		delete(s.entries, key)
		_ = os.Remove(filepath.Join(s.dir, key+".json"))
	}
}
