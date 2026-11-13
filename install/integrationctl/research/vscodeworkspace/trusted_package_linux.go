//go:build linux

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
)

func trustedPackageDigest(ctx context.Context, root string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	captured, err := trustedCapturePackage(ctx, root)
	if err != nil {
		return "", err
	}
	return packagedigest.DigestCaptured(ctx, captured)
}

// Installed managed receipts use the existing provider Stager's framing. The
// acquired source's v1 digest above remains a separate immutable identity.
func trustedInstalledPackageDigest(ctx context.Context, root string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	captured, err := trustedCapturePackage(ctx, root)
	if err != nil {
		return "", err
	}
	if _, err := packagedigest.DigestCaptured(ctx, captured); err != nil {
		return "", err
	}
	sort.Slice(captured, func(i, j int) bool { return captured[i].Path < captured[j].Path })
	hash := sha256.New()
	for _, item := range captured {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		kind := "file"
		if item.Kind == "directory" {
			kind = "dir"
		}
		_, _ = fmt.Fprintf(hash, "%s\x00%s\x00%t\x00%d\x00", kind, item.Path, item.Executable, len(item.Content))
		for body := item.Content; len(body) > 0; {
			if err := ctx.Err(); err != nil {
				return "", err
			}
			n := min(len(body), 32*1024)
			_, _ = hash.Write(body[:n])
			body = body[n:]
		}
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

func trustedCapturePackage(ctx context.Context, root string) ([]packagedigest.CapturedEntry, error) {
	parent, err := trustedParent(filepath.Join(root, "leaf"))
	if err != nil {
		return nil, err
	}
	_ = parent.Close()
	captured := []packagedigest.CapturedEntry{}
	count, total := 0, int64(0)
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		return captureTrustedPackageEntry(ctx, root, path, walkErr, &captured, &count, &total)
	})
	if err != nil {
		return nil, err
	}
	return captured, nil
}

func captureTrustedPackageEntry(ctx context.Context, root, path string, walkErr error, captured *[]packagedigest.CapturedEntry, count *int, total *int64) error {
	if walkErr != nil {
		return walkErr
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	*count++
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	if *count > 4096 || len(strings.Split(rel, string(filepath.Separator))) > 32 {
		return fmt.Errorf("package capture entry/depth bound")
	}
	info, err := anchoredInfo(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		parent, err := trustedParent(filepath.Join(path, "leaf"))
		if err != nil {
			return err
		}
		_ = parent.Close()
		if rel != "." {
			*captured = append(*captured, packagedigest.CapturedEntry{Path: filepath.ToSlash(rel), Kind: "directory"})
		}
		return nil
	}
	if !regular(info, 128<<20) {
		return fmt.Errorf("package capture nonordinary file")
	}
	*total += info.Size()
	if *total > 256<<20 {
		return fmt.Errorf("package capture byte bound")
	}
	body, err := readOrdinary(path, 128<<20)
	if err != nil {
		return err
	}
	*captured = append(*captured, packagedigest.CapturedEntry{Path: filepath.ToSlash(rel), Kind: "file", Executable: info.Mode().Perm()&0111 != 0, Content: body})
	return nil
}
