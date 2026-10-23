package sourcespec

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseDirectSources(t *testing.T) {
	sha := strings.Repeat("a", 40)
	tests := []struct {
		raw     string
		kind    Kind
		git     *Git
		archive *Archive
	}{
		{"Owner/Repo@" + sha + "//plugins/demo", KindGit, &Git{URL: "https://github.com/owner/repo.git", Repository: "owner/repo", Commit: sha, Path: "plugins/demo"}, nil},
		{"github:owner/repo@refs/heads/main//pkg", KindGit, &Git{URL: "https://github.com/owner/repo.git", Repository: "owner/repo", Ref: "refs/heads/main", Path: "pkg"}, nil},
		{"https://example.test/Team/Repo.git#v1//pkg", KindGit, &Git{URL: "https://example.test/Team/Repo.git", Ref: "v1", Path: "pkg"}, nil},
		{"https://example.test/Team%2FRepo.git", KindGit, &Git{URL: "https://example.test/Team%2FRepo.git"}, nil},
		{"git+https://example.test/Team/Repo", KindGit, &Git{URL: "https://example.test/Team/Repo"}, nil},
		{"ssh://git@example.test:2222/Team/Repo#main", KindGit, &Git{URL: "ssh://git@example.test:2222/Team/Repo", Ref: "main"}, nil},
		{"git@ssh.dev.azure.com:v3/org/project/repo", KindGit, &Git{URL: "git@ssh.dev.azure.com:v3/org/project/repo"}, nil},
		{"git.example.test:team/repo.git", KindGit, &Git{URL: "git.example.test:team/repo.git"}, nil},
		{"git@example.test:/srv/repo.git", KindGit, &Git{URL: "git@example.test:/srv/repo.git"}, nil},
		{"https://github.com/owner/repo/tree/feature/a/pkg", KindGit, &Git{URL: "https://github.com/owner/repo.git", Repository: "owner/repo", Browser: &BrowserTree{Provider: "github", Suffix: "feature/a/pkg"}}, nil},
		{"https://gitlab.com/group/sub/repo/-/tree/main/pkg", KindGit, &Git{URL: "https://gitlab.com/group/sub/repo.git", Browser: &BrowserTree{Provider: "gitlab", Suffix: "main/pkg"}}, nil},
		{"https://self.test/group/sub/repo/-/tree/main/pkg", KindGit, &Git{URL: "https://self.test/group/sub/repo.git", Browser: &BrowserTree{Provider: "gitlab", Suffix: "main/pkg"}}, nil},
		{"https://org@dev.azure.com/org/project/_git/repo?version=GBfeature%2Fa&path=%2Fplugins%2Fdemo&_a=contents", KindGit, &Git{URL: "https://dev.azure.com/org/project/_git/repo", Ref: "refs/heads/feature/a", Path: "plugins/demo"}, nil},
		{"https://org.visualstudio.com/project/_git/repo?version=GC" + sha, KindGit, &Git{URL: "https://org.visualstudio.com/project/_git/repo", Commit: sha}, nil},
		{"https://dev.azure.com/org/project/_git/repo.git?version=GBmain", KindGit, &Git{URL: "https://dev.azure.com/org/project/_git/repo.git", Ref: "refs/heads/main"}, nil},
		{"https://github.com/owner/repo/releases/download/v1/plugin.zip", KindArchive, nil, &Archive{URL: "https://github.com/owner/repo/releases/download/v1/plugin.zip"}},
		{"https://gitlab.com/g/r/-/archive/main/r.tar.gz", KindArchive, nil, &Archive{URL: "https://gitlab.com/g/r/-/archive/main/r.tar.gz"}},
		{"https://downloads.test/package#sha256=" + strings.Repeat("b", 64) + "&path=plugins%2Fdemo", KindArchive, nil, &Archive{URL: "https://downloads.test/package", SHA256: strings.Repeat("b", 64), Path: "plugins/demo"}},
		{"https://example.test/package", KindHTTPSAuto, nil, &Archive{URL: "https://example.test/package"}},
		{"./plugin.tgz", KindLocalArchive, nil, &Archive{URL: "./plugin.tgz"}},
		{"./package@1.0.zip", KindLocalArchive, nil, &Archive{URL: "./package@1.0.zip"}},
		{"plugin.zip", KindLocalArchive, nil, &Archive{URL: "plugin.zip"}},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got, err := Parse(tt.raw)
			if err != nil {
				t.Fatal(err)
			}
			if got.Kind != tt.kind || !reflect.DeepEqual(got.Git, tt.git) || !reflect.DeepEqual(got.Archive, tt.archive) {
				t.Fatalf("got %#v git=%#v archive=%#v", got, got.Git, got.Archive)
			}
		})
	}
}

func TestUnsafeSourcesFailWithoutEchoingInput(t *testing.T) {
	for _, raw := range []string{
		"ssh://user%0Aname@example.test/repo", "ssh://user%01name@example.test/repo",
		"C:/repo.git", "C:\\repo.git",
		"https://secret:TOKEN@example.test/r.git", "ssh://secret:TOKEN@example.test/r.git",
		"https://example.test/a.zip?token=TOKEN", "https://example.test/r.git#--upload-pack=TOKEN",
		"https://example.test/r.git#main//../escape", "git+file:///tmp/repo",
		"https://github.com/o/r/blob/main/SKILL.md", "https://gitlab.com/g/r/-/blob/main/SKILL.md",
		"https://dev.azure.com/o/p/_git/r?token=TOKEN", "https://dev.azure.com/o/p/_git/r?version=GBa&version=GBb",
		"https://example.test/r.git#main\nTOKEN", "https://example.test/../r.git", "ssh://git@example.test/repo#main//CON",
		"https://github.com/owner%2Frepo.git", "https://gitlab.com/g/r/-/tree/feature%2Fa/pkg", "https://example.test/r%01.git",
	} {
		t.Run(raw, func(t *testing.T) {
			_, err := Parse(raw)
			if err == nil {
				t.Fatal("unsafe input accepted")
			}
			if strings.Contains(err.Error(), "TOKEN") {
				t.Fatal("credential material leaked")
			}
		})
	}
}

func TestStableTupleKeysAreUnambiguousAndCaseSensitive(t *testing.T) {
	a := Key(KindGit, "ssh://git@example.test/a:b", "c")
	b := Key(KindGit, "ssh://git@example.test/a", "b:c")
	c := Key(KindGit, "ssh://git@example.test/a:b", "C")
	if a == b || a == c || a == Key(KindArchive, "ssh://git@example.test/a:b", "c") {
		t.Fatal("distinct source tuples collided")
	}
	if !strings.HasPrefix(a, "git-url-v1:") || len(a) != len("git-url-v1:")+64 {
		t.Fatalf("invalid key %q", a)
	}
}
