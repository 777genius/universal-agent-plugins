//go:build linux

package pluginkitairepo_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
)

func geminiCycleInstalledDigest(ctx context.Context, captured []packagedigest.CapturedEntry) (string, error) {
	if _, err := packagedigest.DigestCaptured(ctx, captured); err != nil {
		return "", err
	}
	items := append([]packagedigest.CapturedEntry(nil), captured...)
	sort.Slice(items, func(i, j int) bool { return items[i].Path < items[j].Path })
	hash := sha256.New()
	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		kind := "file"
		switch item.Kind {
		case "directory":
			kind = "dir"
		case "file":
		default:
			return "", fmt.Errorf("installed package capture requires ordinary files/directories")
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
