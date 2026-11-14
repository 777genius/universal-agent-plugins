package process

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

type IO struct{}

func (IO) ReadStdin(ctx context.Context) ([]byte, error) {
	return readAllLimited(ctx, os.Stdin, runtime.MaxPayloadBytes, "stdin payload")
}

func readAllLimited(ctx context.Context, r io.Reader, limit int, label string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	limited := io.LimitReader(r, int64(limit)+1)
	b, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read stdin: %w", err)
	}
	if len(b) > limit {
		return nil, fmt.Errorf("read stdin: %s exceeds max payload size of %d bytes", label, limit)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return b, nil
}

func (IO) WriteStdout(b []byte) error {
	_, err := os.Stdout.Write(b)
	return err
}

func (IO) WriteStderr(s string) error {
	_, err := os.Stderr.WriteString(s)
	return err
}

type Env struct{}

func (Env) LookupEnv(k string) (string, bool) {
	return os.LookupEnv(k)
}

func BuildEnvelope(ctx context.Context, inv runtime.Invocation, carrier runtime.CarrierKind, args []string, io runtime.IO, env runtime.Env) (runtime.Envelope, error) {
	out := runtime.Envelope{
		Invocation: inv,
		Args:       append([]string(nil), args...),
		Env:        env,
	}
	switch carrier {
	case runtime.CarrierStdinJSON:
		b, err := io.ReadStdin(ctx)
		if err != nil {
			return runtime.Envelope{}, err
		}
		out.Stdin = b
	case runtime.CarrierArgvJSON:
	default:
		return runtime.Envelope{}, fmt.Errorf("unsupported carrier %q", carrier)
	}
	return out, nil
}
