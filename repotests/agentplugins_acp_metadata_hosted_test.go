//go:build linux || darwin

package pluginkitairepo_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type acpMetadataFile struct {
	Mode     uint32      `json:"mode"`
	Bytes    int64       `json:"bytes"`
	SHA      string      `json:"sha256,omitempty"`
	Identity []uint64    `json:"identity"`
	info     os.FileInfo
}

func acpMetadataFileFact(path string) (acpMetadataFile, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return acpMetadataFile{}, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) ||
		(stat.Uid != uint32(os.Getuid()) && stat.Uid != 0) {
		return acpMetadataFile{}, fmt.Errorf("metadata custody type/owner")
	}
	fact := acpMetadataFile{Mode: uint32(info.Mode()), Bytes: info.Size(),
		Identity: []uint64{uint64(stat.Dev), stat.Ino, uint64(stat.Nlink), uint64(stat.Uid)}, info: info}
	if info.Mode().IsRegular() {
		if stat.Nlink != 1 {
			return acpMetadataFile{}, fmt.Errorf("metadata custody hardlink")
		}
		body, err := readProjectNativeSnapshot(path, info, 512<<20)
		if err != nil {
			return acpMetadataFile{}, err
		}
		hash := sha256.Sum256(body)
		fact.SHA = hex.EncodeToString(hash[:])
	}
	return fact, nil
}

func acpMetadataTree(root string) (map[string]acpMetadataFile, error) {
	result := map[string]acpMetadataFile{}
	var total int64
	err := filepath.WalkDir(root, func(path string, _ os.DirEntry, walkErr error) error {
		if walkErr != nil || len(result) >= 1024 {
			return fmt.Errorf("metadata custody walk/bound")
		}
		fact, err := acpMetadataFileFact(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		result[filepath.ToSlash(relative)] = fact
		if fact.info.Mode().IsRegular() {
			total += fact.Bytes
		}
		if total > 1<<30 {
			return fmt.Errorf("metadata custody aggregate")
		}
		return nil
	})
	return result, err
}

func acpMetadataSame(before, after map[string]acpMetadataFile) bool {
	if len(before) != len(after) {
		return false
	}
	for path, a := range before {
		b, ok := after[path]
		if !ok || !os.SameFile(a.info, b.info) || a.Mode != b.Mode || !reflect.DeepEqual(a.Identity, b.Identity) ||
			a.SHA != b.SHA || a.Bytes != b.Bytes || !a.info.ModTime().Equal(b.info.ModTime()) {
			return false
		}
	}
	return true
}

func acpMetadataInputs(t *testing.T, client string) (string, string, string, []string) {
	t.Helper()
	temp, err := filepath.EvalSymlinks(os.Getenv("RUNNER_TEMP"))
	if err != nil || !filepath.IsAbs(temp) {
		t.Fatal("ordinary hosted runner scratch required")
	}
	vendor := filepath.Join(temp, "vendor-root")
	if client == "cursor" && runtime.GOOS == "linux" {
		root := filepath.Join(vendor, "dist-package")
		return root, filepath.Join(root, "cursor-agent"), filepath.Join(temp, "cursor-agent.tar.gz"),
			[]string{"85296944e4600a83c4b764f578d1502bf9c416db200d33a593e8f3ecabfdc294", "2026.10.22-27dcf5f"}
	}
	if client == "kiro" && runtime.GOOS == "darwin" {
		root := filepath.Join(vendor, "kirocli")
		return root, filepath.Join(root, "bin", "kiro-cli"), filepath.Join(temp, "kirocli-universal-darwin.zip"),
			[]string{"0a8f067c4fe6a23a231aa3a18e3e4332120ef9ed3d2d7b6eea907beb8b4cdd8b", "3.0.0"}
	}
	t.Fatal("only exact existing Cursor Linux or Kiro macOS acquisition is admitted")
	return "", "", "", nil
}

func acpMetadataProfile(t *testing.T, client string) (string, string, []string, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home, project := filepath.Join(root, "home"), filepath.Join(root, "project")
	env := nativePlatformEnvironment(root, home)
	for _, entry := range env {
		key, value, _ := strings.Cut(entry, "=")
		if key != "PATH" && filepath.IsAbs(value) {
			if err := os.MkdirAll(value, 0700); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := os.Mkdir(project, 0700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(project, "."+client, "skills", acpMetadataMarker, "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(marker), 0700); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: " + acpMetadataMarker + "\ndescription: Inert ACP metadata fixture\n---\nDo not execute this Skill.\n"
	if err := os.WriteFile(marker, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	for parent := filepath.Dir(project); ; parent = filepath.Dir(parent) {
		for _, name := range []string{".cursor", ".kiro", ".agents", ".mcp.json", "AGENTS.md"} {
			if _, err := os.Lstat(filepath.Join(parent, name)); !os.IsNotExist(err) {
				t.Fatal("ancestor startup policy is not empty")
			}
		}
		if parent == filepath.Dir(parent) {
			break
		}
	}
	return root, project, env, marker
}

func acpMetadataWrite(t *testing.T, report map[string]any) {
	t.Helper()
	path := os.Getenv("AGENTPLUGINS_ACP_METADATA_REPORT")
	if !filepath.IsAbs(path) {
		t.Error("absolute metadata report path required")
		return
	}
	body, err := json.MarshalIndent(report, "", "  ")
	if err != nil || len(body) > 2<<20 {
		t.Error("metadata report bound")
		return
	}
	if err := os.WriteFile(path, append(body, '\n'), 0600); err != nil {
		t.Error(err)
	}
}

func acpMetadataFreeze(t *testing.T, client, runtimeRoot, executable, archive string, pin []string, report map[string]any) map[string]acpMetadataFile {
	t.Helper()
	archiveFact, err := acpMetadataFileFact(archive)
	if err != nil || archiveFact.SHA != pin[0] {
		t.Fatal("exact official archive pin differs")
	}
	versionPath := filepath.Join(filepath.Dir(archive), "vendor-proof", "version.txt")
	info, err := os.Lstat(versionPath)
	if err != nil {
		t.Fatal(err)
	}
	version, err := readProjectNativeSnapshot(versionPath, info, 65536)
	if err != nil || !strings.Contains(" "+strings.TrimSpace(string(version))+" ", " "+pin[1]+" ") {
		t.Fatal("existing exact acquired version receipt differs")
	}
	before, err := acpMetadataTree(runtimeRoot)
	if err != nil {
		t.Fatal(err)
	}
	if client == "cursor" {
		expected := map[string]string{"cursor-agent": "2ccc9a8e167797641448b5e5c936f006ba137a2555f117f38c5eb76a5238a233",
			"node": "e32e2d16081cf644f88392934288086e7662529d0d43277c34f7aad2207cfafb",
			"index.js": "7b4bb3badb36cff7b6cee7996d61bd3c32866cf354214374b77f5a5d33a0b3f2"}
		for path, expectedSHA := range expected {
			if before[path].SHA != expectedSHA {
				t.Fatal("Cursor exact launcher/Node/index differs")
			}
		}
	}
	report["archive_path"], report["archive_before"] = archive, archiveFact
	report["version"], report["runtime_before"], report["executable"] = strings.TrimSpace(string(version)), before, executable
	return before
}

func TestAgentpluginsACPMetadataSupplement(t *testing.T) {
	client := os.Getenv("AGENTPLUGINS_ACP_METADATA_CLIENT")
	if client == "" {
		t.Skip("explicit hosted metadata supplement only")
	}
	nativeRequireDisposable(t)
	report := map[string]any{"client": client, "status": "unqualified", "skills_ready": false,
		"model_runtime_qualified": false, "vendor_authentication_qualified": false,
		"stderr": "existing runner retains at most32KiB diagnostics; not returned on nil; no empty-stderr claim"}
	defer acpMetadataWrite(t, report)
	source, err := nativeSourceIdentity(os.Getenv("AGENTPLUGINS_ACP_METADATA_COMMIT"), os.Getenv("AGENTPLUGINS_ACP_METADATA_TREE"), "")
	if err != nil {
		t.Fatal(err)
	}
	report["source"] = source
	report["network"] = "ordinary hosted runner; no egress firewall claim; no prompt/authenticate/token/permission response sent"
	runtimeRoot, executable, archive, pin := acpMetadataInputs(t, client)
	before := acpMetadataFreeze(t, client, runtimeRoot, executable, archive, pin, report)
	root, project, env, marker := acpMetadataProfile(t, client)
	profileBefore, err := acpMetadataTree(root)
	if err != nil {
		t.Fatal(err)
	}
	report["profile_root"], report["cwd"], report["marker_path"], report["profile_before"] = root, project, marker, profileBefore
	report["intended_marker"] = acpMetadataMarker
	argv := []string{executable, "acp"}
	if client == "kiro" {
		argv = append(argv, "--agent-engine=v3", "--auth-method=client")
	}
	exchange := &acpMetadataExchange{}
	report["argv"], report["exchange"] = argv, exchange
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	started := time.Now()
	err = (process.OS{}).RunDuplexToCompletion(ctx, ports.Command{Argv: argv, Dir: project, Env: env, StdoutLimitBytes: acpMetadataLimit},
		func(w io.Writer, r io.Reader) error { return exchange.exchange(w, r, project) })
	report["elapsed_seconds"], report["natural_completion"] = time.Since(started).Seconds(), err == nil
	acpMetadataFinish(t, report, runtimeRoot, archive, root, before, profileBefore, exchange, err)
}

func acpMetadataCompletionClass(err error) string {
	if err == nil {
		return "natural-completion"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "deadline"
	}
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	if errors.Is(err, os.ErrPermission) {
		return "permission-refused"
	}
	// Classify known supervisor boundaries without publishing native stderr,
	// OAuth URLs, request parameters or an unrestricted error string.
	text := err.Error()
	for _, boundary := range []struct{ match, class string }{
		{"forced", "forced-cleanup"},
		{"before the exchange completed", "early-exit"},
		{"containment", "containment-refused"},
		{"enumerat", "process-enumeration-refused"},
		{"ACP ", "protocol-refused"},
	} {
		if strings.Contains(text, boundary.match) {
			return boundary.class
		}
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return fmt.Sprintf("native-exit-%d", exit.ExitCode())
	}
	return "completion-refused"
}

func acpMetadataFinish(t *testing.T, report map[string]any, runtimeRoot, archive, root string, before, profileBefore map[string]acpMetadataFile, exchange *acpMetadataExchange, runErr error) {
	t.Helper()
	after, runtimeErr := acpMetadataTree(runtimeRoot)
	profileAfter, profileErr := acpMetadataTree(root)
	archiveAfter, archiveErr := acpMetadataFileFact(archive)
	report["runtime_after"], report["profile_after"], report["archive_after"] = after, profileAfter, archiveAfter
	archiveBefore := report["archive_before"].(acpMetadataFile)
	archiveSame := archiveErr == nil && archiveAfter.SHA == archiveBefore.SHA &&
		os.SameFile(archiveBefore.info, archiveAfter.info) && reflect.DeepEqual(archiveBefore.Identity, archiveAfter.Identity) &&
		archiveBefore.Mode == archiveAfter.Mode && archiveBefore.info.ModTime().Equal(archiveAfter.info.ModTime())
	custody := runtimeErr == nil && profileErr == nil && archiveSame && acpMetadataSame(before, after) && acpMetadataSame(profileBefore, profileAfter)
	report["custody_unchanged"] = custody
	report["completion_class"] = acpMetadataCompletionClass(runErr)
	report["stage"] = "post-completion-custody"
	if runErr != nil {
		sum := sha256.Sum256([]byte(runErr.Error()))
		report["completion_error_sha256"] = hex.EncodeToString(sum[:])
	}
	if runErr == nil && custody && exchange.StdoutEOF && exchange.SessionID != "" && exchange.MarkerListed && exchange.StoppedMethod == "" && exchange.Observation == "session-created-catalog-unobserved" {
		report["status"] = "metadata-observed"
		return
	}
	report["observation"] = exchange.Observation
	t.Errorf("ACP metadata client=%s stage=post-completion-custody observation=%s completion=%s custody_unchanged=%t; supplement unqualified; original full30 remains separate",
		report["client"], exchange.Observation, acpMetadataCompletionClass(runErr), custody)
}
