package securityscan

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const (
	maxScannerOutput = 8 << 20
	maxScannerError  = 64 << 10
)

type Scanner interface {
	Scan(context.Context, string) ([]byte, error)
}

type CommandScanner struct {
	Executable string
	Timeout    time.Duration
}

func (scanner CommandScanner) Scan(ctx context.Context, packageRoot string) ([]byte, error) {
	if strings.TrimSpace(scanner.Executable) == "" {
		return nil, errors.New("lintai executable is unavailable")
	}
	executable, err := filepath.Abs(scanner.Executable)
	if err != nil || !filepath.IsAbs(executable) {
		return nil, fmt.Errorf("resolve lintai executable: %w", err)
	}
	deadline := scanner.Timeout
	if deadline <= 0 {
		deadline = 30 * time.Second
	}
	scanCtx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()
	command := exec.CommandContext(scanCtx, executable, "scan-agent-plugin", packageRoot)
	command.Dir = packageRoot
	command.Env = scannerEnvironment()
	stdout := &limitedBuffer{limit: maxScannerOutput}
	stderr := &limitedBuffer{limit: maxScannerError}
	command.Stdout = stdout
	command.Stderr = stderr
	err = command.Run()
	if errors.Is(scanCtx.Err(), context.DeadlineExceeded) {
		return nil, fmt.Errorf("lintai security scan timed out")
	}
	if stdout.overflow || stderr.overflow {
		return nil, fmt.Errorf("lintai security scan exceeded its output limit")
	}
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			return nil, fmt.Errorf("lintai security scan failed: %s", strings.TrimSpace(stderr.String()))
		}
	}
	return append([]byte(nil), stdout.Bytes()...), nil
}

func scannerEnvironment() []string {
	allowed := []string{"SystemRoot", "WINDIR", "TEMP", "TMP", "TMPDIR"}
	environment := make([]string, 0, len(allowed)+1)
	for _, key := range allowed {
		if value, ok := os.LookupEnv(key); ok {
			environment = append(environment, key+"="+value)
		}
	}
	if runtime.GOOS != "windows" {
		environment = append(environment, "PATH=/usr/bin:/bin")
	}
	return environment
}

type limitedBuffer struct {
	bytes.Buffer
	limit    int
	overflow bool
}

func (buffer *limitedBuffer) Write(value []byte) (int, error) {
	if buffer.Buffer.Len()+len(value) > buffer.limit {
		remaining := buffer.limit - buffer.Buffer.Len()
		if remaining > 0 {
			_, _ = buffer.Buffer.Write(value[:remaining])
		}
		buffer.overflow = true
		return len(value), nil
	}
	return buffer.Buffer.Write(value)
}

type rawReport struct {
	SchemaVersion int `json:"schema_version"`
	Tool          struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"tool"`
	Policy struct {
		ID      string   `json:"id"`
		Version int      `json:"version"`
		Presets []string `json:"presets"`
	} `json:"policy"`
	Stats struct {
		ScannedFiles int `json:"scanned_files"`
		SkippedFiles int `json:"skipped_files"`
	} `json:"stats"`
	Findings      []rawFinding      `json:"findings"`
	Diagnostics   []json.RawMessage `json:"diagnostics"`
	RuntimeErrors []json.RawMessage `json:"runtime_errors"`
}

type rawFinding struct {
	RuleCode   string `json:"rule_code"`
	Category   string `json:"category"`
	Severity   string `json:"severity"`
	Confidence string `json:"confidence"`
	Message    string `json:"message"`
	Location   struct {
		NormalizedPath string `json:"normalized_path"`
		Start          *struct {
			Line int `json:"line"`
		} `json:"start"`
	} `json:"location"`
}

func decodeReport(body []byte) (rawReport, error) {
	if len(body) == 0 || len(body) > maxScannerOutput || !json.Valid(body) {
		return rawReport{}, errors.New("lintai returned invalid JSON")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	var report rawReport
	if err := decoder.Decode(&report); err != nil {
		return rawReport{}, fmt.Errorf("decode lintai report: %w", err)
	}
	if report.SchemaVersion != 1 || report.Tool.Name != "lintai" || report.Tool.Version != domain.SecurityScannerVersion || report.Policy.ID != domain.SecurityPolicyID || report.Policy.Version != domain.SecurityPolicyVersion {
		return rawReport{}, errors.New("lintai report has an unsupported scanner or policy identity")
	}
	if len(report.RuntimeErrors) != 0 {
		return rawReport{}, errors.New("lintai could not scan every selected package surface")
	}
	if report.Stats.ScannedFiles < 0 || report.Stats.SkippedFiles < 0 {
		return rawReport{}, errors.New("lintai report contains invalid scan counts")
	}
	return report, nil
}
