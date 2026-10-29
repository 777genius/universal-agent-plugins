package kimi

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestVersionProbeEnvironmentBindsOnlySelectedKimiProfileAndCache(t *testing.T) {
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(parent, ".kimi-code")
	for _, key := range []string{"HOME", "USERPROFILE", "KIMI_CODE_HOME", "KIMI_CODE_CACHE_DIR", "KIMI_CODE_NO_AUTO_UPDATE", "HTTPS_PROXY", "NODE_OPTIONS", "KIMI_API_KEY"} {
		t.Setenv(key, "ambient-canary")
	}
	want := []string{"KIMI_CODE_HOME=" + root, "KIMI_CODE_CACHE_DIR=" + filepath.Join(root, "cache"), "KIMI_CODE_NO_AUTO_UPDATE=1"}
	for _, existing := range []bool{false, true} {
		if existing {
			if err := os.MkdirAll(filepath.Join(root, "cache"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "config.toml"), []byte("foreign = true\n"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		env, err := New().VersionProbeEnvironment(root)
		if err != nil || !reflect.DeepEqual(env, want) {
			t.Fatalf("version profile environment differs: %v (%v)", env, err)
		}
		if !existing {
			if _, err := os.Lstat(root); !os.IsNotExist(err) {
				t.Fatalf("version environment created missing native profile: %v", err)
			}
		} else if body, err := os.ReadFile(filepath.Join(root, "config.toml")); err != nil || string(body) != "foreign = true\n" {
			t.Fatal("version environment changed original-user config")
		}
	}
}

func TestVersionProbeEnvironmentRejectsUnsafeKimiProfileOrCache(t *testing.T) {
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{"", "relative", parent + string(filepath.Separator), filepath.VolumeName(parent) + string(filepath.Separator), parent + string(filepath.Separator) + ".." + string(filepath.Separator) + "profile", parent + "\x00bad", parent + "\nbad", parent + string([]byte{0xff})} {
		if _, err := New().VersionProbeEnvironment(root); err == nil {
			t.Fatalf("unsafe profile spelling admitted: %q", root)
		}
	}
	if _, err := New().VersionProbeEnvironment(filepath.Join(parent, "missing-parent", "profile")); err == nil {
		t.Fatal("missing profile parent admitted")
	}
	for _, kind := range []string{"profile-file", "cache-file", "profile-link", "ancestor-link", "cache-link"} {
		t.Run(kind, func(t *testing.T) {
			base, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			root := filepath.Join(base, "profile")
			if kind == "profile-file" {
				if err := os.WriteFile(root, []byte("foreign"), 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.Mkdir(root, 0700); err != nil {
					t.Fatal(err)
				}
				if kind == "cache-file" {
					if err := os.WriteFile(filepath.Join(root, "cache"), []byte("foreign"), 0600); err != nil {
						t.Fatal(err)
					}
				} else {
					alias := filepath.Join(base, "alias")
					if kind == "cache-link" {
						alias = filepath.Join(root, "cache")
					}
					if err := os.Symlink(root, alias); err != nil {
						t.Skipf("symlink creation unavailable: %v", err)
					}
					switch kind {
					case "profile-link":
						root = alias
					case "ancestor-link":
						root = filepath.Join(alias, "missing-profile")
					}
				}
			}
			if _, err := New().VersionProbeEnvironment(root); err == nil {
				t.Fatal("unsafe selected profile/cache admitted")
			}
		})
	}
}
