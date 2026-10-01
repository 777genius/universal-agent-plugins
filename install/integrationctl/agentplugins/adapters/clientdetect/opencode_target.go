package clientdetect

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/opencodehost"
)

const OpenCodeProbeTimeout = 10 * time.Second

var (
	ErrInvalidProbeTarget    = errors.New("host_target_invalid")
	ErrUnverifiedProbeTarget = errors.New("host_target_unverified")
	ErrProbeTargetChanged    = errors.New("host_target_changed")
)

// ProbeTarget never selects an executable from PATH. Timeout may shorten the
// fixed maximum. Environment must contain exactly one PATH and no user config.
type ProbeTarget struct {
	Executable  string        `json:"-"`
	Environment []string      `json:"-"`
	Timeout     time.Duration `json:"-"`
}

func (t ProbeTarget) Clone() ProbeTarget { t.Environment = slices.Clone(t.Environment); return t }

type ProbeEvidence struct {
	opencodehost.VersionEvidence
	Reason string
}

// OpenCodeProbeEnvironment captures only process-launch variables. It does not
// inspect HOME, provider, config, or credential variables. New pins this copy.
func OpenCodeProbeEnvironment() []string {
	env := []string{"PATH=" + os.Getenv("PATH")}
	if runtime.GOOS == "windows" {
		for _, key := range []string{"SYSTEMROOT", "WINDIR"} {
			if value, ok := os.LookupEnv(key); ok {
				env = append(env, key+"="+value)
			}
		}
	}
	return env
}

// CopyOpenCodeProbeEnvironment validates and canonicalizes an independent copy.
// Case-equivalent duplicates are rejected on all platforms to keep snapshots
// portable. An empty PATH is valid and never replaced with ambient PATH.
func CopyOpenCodeProbeEnvironment(env []string) ([]string, error) {
	out := make([]string, 0, len(env))
	seen := map[string]bool{}
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		key = strings.ToUpper(key)
		allowed := key == "PATH" || runtime.GOOS == "windows" && (key == "SYSTEMROOT" || key == "WINDIR")
		if !ok || !allowed || seen[key] || strings.ContainsRune(value, 0) {
			return nil, ErrInvalidProbeTarget
		}
		seen[key] = true
		out = append(out, key+"="+value)
	}
	if !seen["PATH"] {
		return nil, ErrInvalidProbeTarget
	}
	slices.Sort(out)
	return out, nil
}

// ProbeOpenCodeTarget supports native executable bytes and bounded symlinks to
// native bytes only. Scripts/npm shims are deliberately unverified and are not
// executed. Runtime authority is checked both sides of the version invocation.
func ProbeOpenCodeTarget(ctx context.Context, target ProbeTarget) (ProbeEvidence, error) {
	evidence := ProbeEvidence{VersionEvidence: opencodehost.VersionEvidence{Source: "executable_version", ProbeStatus: "not_requested"}}
	if ctx == nil {
		return evidence, ErrInvalidProbeTarget
	}
	target = target.Clone()
	if target.Executable == "" {
		return evidence, nil
	}
	target, err := validatedOpenCodeTarget(target)
	if err != nil {
		return evidence, err
	}
	timeout := target.Timeout
	if timeout <= 0 || timeout > OpenCodeProbeTimeout {
		timeout = OpenCodeProbeTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	identity, err := openCodeTargetIdentity(ctx, target)
	if err != nil {
		return failedOpenCodeProbe(ctx, evidence, err)
	}
	stdout, _, err := runVersionProcess(ctx, target.Executable, target.Environment)
	if err != nil {
		return failedOpenCodeProbe(ctx, evidence, err)
	}
	version := strings.TrimSpace(stdout)
	if strings.HasPrefix(version, "opencode v") {
		version = strings.TrimPrefix(version, "opencode v")
	}
	if !opencodehost.ValidVersion(version) {
		evidence.ProbeStatus = "malformed"
		return evidence, errors.New("probe_malformed")
	}
	after, err := openCodeTargetIdentity(ctx, target)
	if err != nil && ctx.Err() != nil {
		return failedOpenCodeProbe(ctx, evidence, ctx.Err())
	}
	if err != nil || identity != after {
		evidence.ProbeStatus = "failed"
		evidence.Reason = "host_target_changed"
		return evidence, ErrProbeTargetChanged
	}
	evidence.Version, evidence.ProbeStatus, evidence.ExecutableIdentity = version, "ok", identity
	return evidence, nil
}

func sanitizedProbeError(ctx context.Context, status string, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, ErrUnverifiedProbeTarget) {
		return ErrUnverifiedProbeTarget
	}
	return errors.New("probe_" + status)
}

func validatedOpenCodeTarget(target ProbeTarget) (ProbeTarget, error) {
	if !filepath.IsAbs(target.Executable) || filepath.Clean(target.Executable) != target.Executable || strings.ContainsRune(target.Executable, 0) {
		return target, ErrInvalidProbeTarget
	}
	env, err := CopyOpenCodeProbeEnvironment(target.Environment)
	if err != nil {
		return target, err
	}
	target.Environment = env
	return target, nil
}

func failedOpenCodeProbe(ctx context.Context, evidence ProbeEvidence, err error) (ProbeEvidence, error) {
	evidence.ProbeStatus = "failed"
	switch {
	case ctx.Err() != nil:
		evidence.ProbeStatus = "timed_out"
	case errors.Is(err, errVersionOutputLimit):
		evidence.ProbeStatus = "output_limit"
	case errors.Is(err, os.ErrNotExist):
		evidence.ProbeStatus = "absent"
	}
	if errors.Is(err, ErrUnverifiedProbeTarget) {
		evidence.Reason = "host_target_unverified"
	}
	return evidence, sanitizedProbeError(ctx, evidence.ProbeStatus, err)
}
