package securityscan

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type Cache interface {
	Load(string) (domain.SecurityAssessment, bool)
	Store(string, domain.SecurityAssessment) error
}

type FileCache struct {
	Root string
}

func CacheKey(subject domain.SecuritySubject, requirement domain.SecurityRequirement) string {
	value := strings.Join([]string{subject.TreeDigest, subject.ManifestDigest, requirement.Scanner.ID, requirement.Scanner.Version, requirement.Policy.ID, fmt.Sprint(requirement.Policy.Version), requirement.Policy.Digest}, "\x00")
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func (cache FileCache) Load(key string) (domain.SecurityAssessment, bool) {
	body, err := os.ReadFile(filepath.Join(cache.Root, key+".json"))
	if err != nil || len(body) > 1<<20 {
		return domain.SecurityAssessment{}, false
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	var assessment domain.SecurityAssessment
	if decoder.Decode(&assessment) != nil {
		return domain.SecurityAssessment{}, false
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return domain.SecurityAssessment{}, false
	}
	return assessment, true
}

func (cache FileCache) Store(key string, assessment domain.SecurityAssessment) error {
	if len(key) != 64 || cache.Root == "" {
		return errors.New("invalid security cache target")
	}
	if err := os.MkdirAll(cache.Root, 0o700); err != nil {
		return fmt.Errorf("create security cache: %w", err)
	}
	body, err := json.Marshal(assessment)
	if err != nil {
		return fmt.Errorf("encode security cache: %w", err)
	}
	temporary, err := os.CreateTemp(cache.Root, ".assessment-*.tmp")
	if err != nil {
		return fmt.Errorf("create security cache entry: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(body); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	target := filepath.Join(cache.Root, key+".json")
	if runtime.GOOS == "windows" {
		_ = os.Remove(target)
	}
	if err := os.Rename(temporaryName, target); err != nil {
		return fmt.Errorf("commit security cache entry: %w", err)
	}
	return nil
}
