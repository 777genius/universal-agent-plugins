//go:build (linux || windows) && (amd64 || arm64)

package commands_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/report"
)

// This bounded native lifecycle complements the Linux syscall/guard suites.
// It makes no syscall-observation claim. The shared runner isolates HOME,
// profiles, PATH and scratch and bounds every actual CLI invocation to 30s.
func TestNativeBinaryLifecycle(t *testing.T) {
	supplied, head := os.Getenv("AUTHORING_NATIVE_BIN_DIR"), os.Getenv("EXPECTED_HEAD")
	if os.Getenv("EXPECTED_OS") != "" && (supplied == "" || head == "") {
		t.Fatal("native CI requires AUTHORING_NATIVE_BIN_DIR and EXPECTED_HEAD; no fallback builds")
	}
	if supplied != "" || head != "" {
		decoded, err := hex.DecodeString(head)
		if supplied == "" || !filepath.IsAbs(supplied) || err != nil || len(decoded) != 20 {
			t.Fatal("external binaries require an absolute AUTHORING_NATIVE_BIN_DIR and full hex EXPECTED_HEAD")
		}
	}
	b := buildPathFixBinaries(t)
	parents := [2]string{t.TempDir(), t.TempDir()}
	scratch := [2]string{t.TempDir(), t.TempDir()}
	roots := [2]string{filepath.Join(parents[0], "demo"), filepath.Join(parents[1], "demo")}
	// Each entrypoint creates and mutates its own fresh tree. Reports must match
	// byte-for-byte without normalizing away identities or platform evidence.
	pair := func(t *testing.T, cwd [2]string, want int, mutate bool, args ...string) report.Report {
		t.Helper()
		var first report.Report
		var output []byte
		for i := range b.paths {
			before := tree(t, parents[i])
			r, code, out := b.run(t, i, cwd[i], scratch[i], args...)
			if code != want || r.Runtime.Status != report.NotEvaluated {
				t.Fatalf("binary=%d args=%q exit=%d want=%d: %s", i, args, code, want, out)
			}
			if !mutate && (!reflect.DeepEqual(before, tree(t, parents[i])) || r.Committed) {
				t.Fatalf("binary=%d args=%q changed source or claimed commit", i, args)
			}
			if len(tree(t, scratch[i])) != 0 {
				t.Fatal("scratch residue")
			}
			t.Logf("binary=%d command=%s exit=%d report_sha256=%x", i, r.Command, code, sha256.Sum256(out))
			if i == 0 {
				first, output = r, out
			} else if !bytes.Equal(output, out) {
				t.Fatalf("report parity args=%q:\n%s\n%s", args, output, out)
			}
		}
		if !reflect.DeepEqual(tree(t, parents[0]), tree(t, parents[1])) {
			t.Fatal("tree byte/mode parity")
		}
		return first
	}
	requireSkills := func(t *testing.T, r report.Report, pass, fail int) {
		t.Helper()
		gotPass, gotFail := 0, 0
		for _, c := range r.Components {
			if c.Type == "skill" {
				if c.Status == report.Pass {
					gotPass++
				}
				if c.Status == report.Fail {
					gotFail++
				}
			}
		}
		if gotPass != pass || gotFail != fail || r.HostSafety.Status != report.Pass || len(r.Profiles) == 0 {
			t.Fatalf("skill isolation/profile evidence: %+v", r)
		}
		for _, p := range r.Profiles {
			if p.ID == "" || p.Revision == "" || p.Digest == "" {
				t.Fatal("incomplete embedded profile identity")
			}
		}
	}
	// Stop dependent stages after failure; absent subtest passes must fail CI's
	// required-name checker, never be interpreted as an unsupported-platform skip.
	if !t.Run("generated-skill", func(t *testing.T) {
		r := pair(t, parents, 0, true, "init", "demo", "--template=skill", "--name=demo", "--description=Disposable lifecycle fixture.")
		if !r.Committed {
			t.Fatal("template not committed")
		}
		requireSkills(t, r, 1, 0)
	}) {
		return
	}
	if !t.Run("skills-init", func(t *testing.T) {
		before := tree(t, roots[0])
		r := pair(t, roots, 0, true, "skills", "init", "docs-helper", "--description=Use for documentation requests.")
		if !r.Committed {
			t.Fatal("skill not committed")
		}
		after := tree(t, roots[0])
		for path, value := range before {
			if after[path] != value {
				t.Fatalf("init changed existing path %s", path)
			}
		}
		if _, err := os.Stat(filepath.Join(roots[0], "skills/docs-helper/SKILL.md")); err != nil {
			t.Fatal(err)
		}
		requireSkills(t, r, 2, 0)
	}) {
		return
	}
	t.Run("static-validation", func(t *testing.T) {
		for _, args := range [][]string{{"skills", "validate"}, {"validate", "."}, {"test", "."}} {
			r := pair(t, roots, 0, false, args...)
			requireSkills(t, r, 2, 0)
			if r.Conformance.Status != report.Pass || r.Readiness.Status != report.Pass || r.Identity.ScopeDigest == "" || r.Identity.TreeDigest == "" {
				t.Fatalf("static evidence: %+v", r)
			}
		}
	})
	t.Run("duplicate-unchanged", func(t *testing.T) {
		r := pair(t, roots, 1, false, "skills", "init", "docs-helper", "--description=Different duplicate content.")
		if r.Error == nil {
			t.Fatal("duplicate missing structured error")
		}
	})
	t.Run("readiness", func(t *testing.T) {
		r := pair(t, roots, 0, false, "capabilities")
		if r.Capabilities == nil || len(r.Capabilities.Commands) != 8 || r.Readiness.Status != report.NotEvaluated {
			t.Fatal("capabilities contract")
		}
		r = pair(t, roots, 0, false, "compat", ".", "--target=cursor,codex,claude")
		if len(r.Clients) != 3 || r.Compatibility.Status != report.Pass {
			t.Fatal("explicit static compatibility")
		}
		r = pair(t, roots, 0, false, "doctor", ".")
		if r.Toolchain.Status != report.Pass || r.Conformance.Status != report.Pass {
			t.Fatal("skill static doctor")
		}
	})
	t.Run("strict-flags", func(t *testing.T) {
		for _, args := range [][]string{
			{"compat", ".", "--target=codex", "--dry-run=false"},
			{"doctor", ".", "--runtime"}, {"doctor", ".", "--target=codex"},
			{"capabilities", "unexpected"}, {"skills", "validate", "--profile=fixture"},
			{"skills", "init", "new", "--description=Fixture", "--scope=user"},
			{"test", ".", "--runtime"},
		} {
			r := pair(t, roots, 2, false, args...)
			if r.Error == nil || r.Error.Code != "arguments_invalid" {
				t.Fatalf("strict argument report: %+v", r)
			}
		}
	})
	t.Run("malformed-skill", func(t *testing.T) {
		for _, root := range roots {
			write(t, root, "skills/bad/SKILL.md", "---\nname: bad\ndescription: 7\n---\n")
		}
		for _, args := range [][]string{{"skills", "validate"}, {"validate", "."}, {"test", "."}} {
			r := pair(t, roots, 1, false, args...)
			requireSkills(t, r, 2, 1)
			if r.Conformance.Status != report.Fail || r.Readiness.Status != report.Fail {
				t.Fatal("malformed skill did not fail static checks")
			}
		}
	})
	t.Run("sdk-static-only", func(t *testing.T) {
		r := pair(t, parents, 0, true, "init", "sdk", "--template=mcp-stdio", "--runtime=node", "--name=sdk", "--description=Disposable SDK fixture.")
		if !r.Committed {
			t.Fatal("SDK fixture not committed")
		}
		r = pair(t, parents, 0, false, "test", "sdk")
		if r.Conformance.Status != report.Pass {
			t.Fatal("SDK static conformance")
		}
		r = pair(t, parents, 1, false, "doctor", "sdk")
		if r.Toolchain.Status != report.NotEvaluated || len(r.DoctorChecks) == 0 || r.Conformance.Status != report.Pass {
			t.Fatalf("SDK runtime inferred from static package: %+v", r)
		}
	})
}
