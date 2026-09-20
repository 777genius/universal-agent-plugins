package checksum

import (
	"encoding/hex"
	"testing"
)

func TestExpectedSum(t *testing.T) {
	t.Parallel()
	hash := "945fac8c0a94b6982c47b54eb9fa6470236bdd1ddd163b7e51f0cfb8aedf3109"
	file := "myproj_1.0.0_linux_amd64.tar.gz"
	body := []byte(hash + "  " + file + "\n")
	got, err := ExpectedSum(body, file)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := hex.DecodeString(hash)
	if string(got) != string(want) {
		t.Fatalf("got %x want %x", got, want)
	}
}

func TestExpectedSum_missing(t *testing.T) {
	t.Parallel()
	_, err := ExpectedSum([]byte("abc  other.tar.gz\n"), "want.tar.gz")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExpectedSum_caseInsensitiveFilename(t *testing.T) {
	t.Parallel()
	hash := "945fac8c0a94b6982c47b54eb9fa6470236bdd1ddd163b7e51f0cfb8aedf3109"
	body := []byte(hash + "  MyProj_LINUX_AMD64.tar.gz\n")
	got, err := ExpectedSum(body, "myproj_linux_amd64.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	want, _ := hex.DecodeString(hash)
	if string(got) != string(want) {
		t.Fatalf("got %x want %x", got, want)
	}
}
