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
	return readLineBuffer(ctx, reader, make([]byte, 1))
}

// Larger buffers are safe only when the kernel guarantees canonical record
// boundaries. Streams use one byte so this owner never prefetches another answer.
func readLineBuffer(ctx context.Context, reader io.Reader, b []byte) (string, error) {
	var line strings.Builder
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		n, err := reader.Read(b)
		if e := ctx.Err(); e != nil {
			return "", e
		}
		if err != nil && err != io.EOF {
			return "", fmt.Errorf("read prompt: %w", err)
		}
		for _, c := range b[:n] {
			if c == '\n' {
				return strings.TrimSuffix(line.String(), "\r"), nil
			}
			if line.Len() >= 4096 {
				return "", fmt.Errorf("prompt answer exceeds 4096 bytes")
			}
			line.WriteByte(c)
		}
		if err == io.EOF {
			return "", prompt.ErrPromptInputClosed
		}
		if n == 0 {
			return "", io.ErrNoProgress
		}
	}
}
