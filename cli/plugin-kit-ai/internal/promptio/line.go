// Package promptio owns bounded, non-prefetching line input shared by prompts.
package promptio

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
)

func ReadLine(ctx context.Context, reader io.Reader) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return readCancelable(ctx, reader)
}
func readLine(ctx context.Context, reader io.Reader) (string, error) {
	var line strings.Builder
	var b [1]byte
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		n, err := reader.Read(b[:])
		if e := ctx.Err(); e != nil {
			return "", e
		}
		if n > 0 && b[0] == '\n' && (err == nil || err == io.EOF) {
			return strings.TrimSuffix(line.String(), "\r"), nil
		}
		if err != nil {
			if err == io.EOF {
				return "", prompt.ErrPromptInputClosed
			}
			return "", fmt.Errorf("read prompt: %w", err)
		}
		if n == 0 {
			return "", io.ErrNoProgress
		}
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if b[0] == '\n' {
			return strings.TrimSuffix(line.String(), "\r"), nil
		}
		if line.Len() >= 4096 {
			return "", fmt.Errorf("prompt answer exceeds 4096 bytes")
		}
		line.WriteByte(b[0])
	}
}
