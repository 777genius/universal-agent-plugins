//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"syscall"
	"testing"
)

func qoderMetadataFixture(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), ".qoder")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	projectNativeRuntimeWrite(t, filepath.Join(root, "settings.json"), []byte("{\"general\":{\"enableAutoUpdate\":false}}\n"), 0600)
	return root
}

func qoderMetadataAuthor(t *testing.T, root string) (*qoderOriginalMetadata, map[string]string) {
	t.Helper()
	protected := map[string]string{}
	recipe, err := qoderAuthorOriginalMetadata(root, protected)
	if err != nil {
		t.Fatal(err)
	}
	return recipe, protected
}

func TestQoderOriginalMetadataExactIndependentAnonymousRecipe(t *testing.T) {
	root := qoderMetadataFixture(t)
	before := qoderDeltaSnapshot(t, root)
	recipe, protected := qoderMetadataAuthor(t, root)
	if err := recipe.check(); err != nil {
		t.Fatal(err)
	}
	after := qoderDeltaSnapshot(t, root)
	if len(after) != len(before)+4 || after["settings.json"] != before["settings.json"] || len(protected) != 3 {
		t.Fatal("recipe changed original settings or authored a different inventory")
	}
	for key, value := range before {
		if key != "." && after[key] != value {
			t.Fatal("preexisting profile entry changed")
		}
	}
	id := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	values := make([][]byte, 0, 3)
	for _, item := range qoderMetadataRecipe {
		path := filepath.Join(root, item.name)
		info, err := os.Lstat(path)
		if err != nil || !qoderLogOwned(info, false) || info.Mode() != item.mode || info.Size() != item.size {
			t.Fatal("anonymous file shape differs")
		}
		body, err := projectNativeRuntimeRead(path, 36)
		if err != nil || (item.size == 36 && !id.Match(body)) || (item.size == 0 && len(body) != 0) {
			t.Fatal("anonymous ID/empty transaction format differs")
		}
		sum := sha256.Sum256(body)
		if protected[path] != hex.EncodeToString(sum[:]) {
			t.Fatal("initial protected digest differs")
		}
		values = append(values, body)
	}
	if bytes.Equal(values[0], values[1]) || recipe.auth.Mode() != os.ModeDir|0700 || !qoderLogOwned(recipe.auth, true) {
		t.Fatal("IDs were reused or anonymous parent authority differs")
	}
	if _, err := os.Lstat(filepath.Join(root, ".auth", "user")); !os.IsNotExist(err) {
		t.Fatal("fixture authored account credentials")
	}
}

func TestQoderOriginalMetadataPreexistingTargetsRefusedWithoutOverwrite(t *testing.T) {
	for _, mode := range []string{"installation-file", "installation-link", "installation-hardlink", "installation-fifo", "auth-directory", "auth-file", "auth-link"} {
		t.Run(mode, func(t *testing.T) {
			root := qoderMetadataFixture(t)
			path := filepath.Join(root, "installation_id")
			if mode[:4] == "auth" {
				path = filepath.Join(root, ".auth")
			}
			outside := filepath.Join(t.TempDir(), "held-original")
			projectNativeRuntimeWrite(t, outside, []byte("foreign original bytes"), 0600)
			switch mode {
			case "auth-directory":
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			case "installation-link", "auth-link":
				if err := os.Symlink(outside, path); err != nil {
					t.Fatal(err)
				}
			case "installation-hardlink":
				if err := os.Link(outside, path); err != nil {
					t.Fatal(err)
				}
			case "installation-fifo":
				if err := syscall.Mkfifo(path, 0600); err != nil {
					t.Fatal(err)
				}
			default:
				projectNativeRuntimeWrite(t, path, []byte("preserve-existing"), 0600)
			}
			before, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			protected := map[string]string{}
			if result, err := qoderAuthorOriginalMetadata(root, protected); err == nil || result != nil || len(protected) != 0 {
				t.Fatal("preexisting target accepted or published partial recipe")
			}
			after, err := os.Lstat(path)
			if err != nil || !projectNativeRuntimeStableStat(before.Sys(), after.Sys()) {
				t.Fatal("refusal changed preexisting target")
			}
			body, err := os.ReadFile(outside)
			if err != nil || string(body) != "foreign original bytes" {
				t.Fatal("refusal changed aliased original data")
			}
		})
	}
}

func qoderMetadataReplace(t *testing.T, path string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, path+".held-original"); err != nil {
		t.Fatal(err)
	}
	projectNativeRuntimeWrite(t, path, body, info.Mode())
}

func TestQoderOriginalMetadataRejectsDriftBeforeCatalogBaseline(t *testing.T) {
	for _, mode := range []string{"mode", "same-bytes-inode", "bytes", "removed", "symlink", "hardlink", "directory", "auth-mode", "auth-inode", "profile-inode"} {
		t.Run(mode, func(t *testing.T) {
			root := qoderMetadataFixture(t)
			recipe, _ := qoderMetadataAuthor(t, root)
			path := filepath.Join(root, ".auth", "machine_id")
			switch mode {
			case "mode", "auth-mode":
				if mode == "auth-mode" {
					path = filepath.Join(root, ".auth")
					t.Cleanup(func() {
						if err := os.Chmod(path, 0700); err != nil {
							t.Error(err)
						}
					})
				}
				if err := os.Chmod(path, 0644); err != nil {
					t.Fatal(err)
				}
			case "same-bytes-inode":
				qoderMetadataReplace(t, path)
			case "bytes":
				if err := os.WriteFile(path, bytes.Repeat([]byte("x"), 36), 0600); err != nil {
					t.Fatal(err)
				}
			case "removed", "symlink", "hardlink", "directory":
				if err := os.Rename(path, path+".held-original"); err != nil {
					t.Fatal(err)
				}
				qoderMetadataAlterLeaf(t, path, mode)
			case "auth-inode", "profile-inode":
				parent := root
				if mode == "auth-inode" {
					parent = filepath.Join(root, ".auth")
				}
				if err := os.Rename(parent, parent+".held-original"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(parent, 0700); err != nil {
					t.Fatal(err)
				}
				// Preserve every child inode and byte so only the held parent
				// identity can reject this otherwise coherent replacement.
				children := []string{"machine_id", ".credential-transaction"}
				if mode == "profile-inode" {
					children = []string{"installation_id", ".auth", "settings.json"}
				}
				for _, child := range children {
					if err := os.Rename(filepath.Join(parent+".held-original", child), filepath.Join(parent, child)); err != nil {
						t.Fatal(err)
					}
				}
			}
			// No profile snapshot has run since authoring: early version/public
			// mutations cannot become the later catalog preservation baseline.
			if err := recipe.check(); err == nil {
				t.Fatal("pre-baseline identity/mode/byte drift accepted")
			}
		})
	}
}

func qoderMetadataAlterLeaf(t *testing.T, path, mode string) {
	t.Helper()
	var err error
	switch mode {
	case "symlink":
		err = os.Symlink(path+".held-original", path)
	case "hardlink":
		err = os.Link(path+".held-original", path)
	case "directory":
		err = os.Mkdir(path, 0600)
	}
	if err != nil {
		t.Fatal(err)
	}
}

func TestQoderOriginalMetadataFullProfileGuardStillRefusesDriftAndNewSibling(t *testing.T) {
	for _, mode := range []string{"bytes", "mode", "new-sibling"} {
		t.Run(mode, func(t *testing.T) {
			root := qoderMetadataFixture(t)
			qoderMetadataAuthor(t, root)
			before := qoderDeltaSnapshot(t, root)
			path := filepath.Join(root, "installation_id")
			switch mode {
			case "bytes":
				if err := os.WriteFile(path, bytes.Repeat([]byte("x"), 36), 0644); err != nil {
					t.Fatal(err)
				}
			case "mode":
				if err := os.Chmod(path, 0600); err != nil {
					t.Fatal(err)
				}
			case "new-sibling":
				projectNativeRuntimeWrite(t, filepath.Join(root, "unknown-native-entry"), nil, 0600)
			}
			after := qoderDeltaSnapshot(t, root)
			if reflect.DeepEqual(before, after) {
				t.Fatal("original whole-profile equality accepted metadata/sibling drift")
			}
		})
	}
}

func TestQoderOriginalMetadataAllFilesRejectEarlySameByteModeAndIdentityChange(t *testing.T) {
	for _, item := range qoderMetadataRecipe {
		for _, change := range []string{"mode", "inode"} {
			t.Run(item.name+"/"+change, func(t *testing.T) {
				root := qoderMetadataFixture(t)
				recipe, _ := qoderMetadataAuthor(t, root)
				path := filepath.Join(root, item.name)
				if change == "inode" {
					qoderMetadataReplace(t, path)
				} else if err := os.Chmod(path, item.mode^0040); err != nil {
					t.Fatal(err)
				}
				if err := recipe.check(); err == nil {
					t.Fatal("early same-byte mode/identity mutation accepted")
				}
			})
		}
	}
}

type qoderMetadataForeignInfo struct {
	os.FileInfo
	stat syscall.Stat_t
}

func (i qoderMetadataForeignInfo) Sys() any { return &i.stat }

func TestQoderOriginalMetadataRejectsForeignHeldOwnerAndGroup(t *testing.T) {
	root := qoderMetadataFixture(t)
	recipe, _ := qoderMetadataAuthor(t, root)
	for _, info := range []os.FileInfo{recipe.profile, recipe.auth, recipe.files[0].info, recipe.files[1].info, recipe.files[2].info} {
		for _, group := range []bool{false, true} {
			foreign := qoderMetadataForeignInfo{info, *info.Sys().(*syscall.Stat_t)}
			if group {
				foreign.stat.Gid++
			} else {
				foreign.stat.Uid++
			}
			if qoderMetadataSame(info, foreign, info.IsDir()) || qoderLogOwned(foreign, info.IsDir()) {
				t.Fatal("foreign held owner/group accepted")
			}
		}
	}
}

func TestQoderOriginalMetadataRejectsRealForeignOwnerWhenSupported(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("real ownership mutation requires root in this inert fixture")
	}
	for _, target := range []string{".", ".auth", "installation_id", ".auth/machine_id", ".auth/.credential-transaction"} {
		for _, group := range []bool{false, true} {
			root := qoderMetadataFixture(t)
			recipe, _ := qoderMetadataAuthor(t, root)
			uid, gid := os.Getuid(), os.Getgid()
			if group {
				gid++
			} else {
				uid++
			}
			path := filepath.Join(root, target)
			if err := os.Chown(path, uid, gid); errors.Is(err, syscall.EINVAL) || errors.Is(err, syscall.EPERM) {
				t.Skip("current user namespace cannot represent a foreign UID/GID; held-stat ownership controls remain required")
			} else if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.Chown(path, os.Getuid(), os.Getgid()); err != nil {
					t.Error(err)
				}
			})
			if err := recipe.check(); err == nil {
				t.Fatal("foreign owner/group accepted")
			}
		}
	}
}

func TestQoderOriginalMetadataExclusivePartialWriteAndParentAliasRefuse(t *testing.T) {
	root := qoderMetadataFixture(t)
	parent, err := qoderLogOpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := parent.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := qoderMetadataCreate(parent, "installation_id", []byte("first independent authored bytes"), 0644); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(root, "installation_id"))
	if err != nil {
		t.Fatal(err)
	}
	if err := qoderMetadataCreate(parent, "installation_id", []byte("replacement must refuse"), 0600); err == nil {
		t.Fatal("exclusive partial-recipe failure replaced earlier file")
	}
	after, err := os.ReadFile(filepath.Join(root, "installation_id"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("partial-recipe refusal changed earlier authored bytes")
	}
	alias := filepath.Join(t.TempDir(), ".qoder")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	if result, err := qoderAuthorOriginalMetadata(alias, map[string]string{}); err == nil || result != nil {
		t.Fatal("aliased original profile accepted")
	}
}

func TestQoderOriginalMetadataOrdinaryReadsAndSameModeChmodDoNotPinTimestamps(t *testing.T) {
	root := qoderMetadataFixture(t)
	recipe, _ := qoderMetadataAuthor(t, root)
	for _, item := range qoderMetadataRecipe {
		path := filepath.Join(root, item.name)
		if _, err := os.ReadFile(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, item.mode); err != nil {
			t.Fatal(err)
		}
	}
	if err := recipe.check(); err != nil {
		t.Fatal("recipe incorrectly pinned ordinary access/change timestamps")
	}
}
