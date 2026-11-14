package domain

import (
	"runtime"
	"testing"
)

func TestPickInstallAsset_singleTarGz(t *testing.T) {
	t.Parallel()
	arch := "p_1.0.0_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz"
	assets := []Asset{{Name: arch}}
	got, tar, err := PickInstallAsset(assets, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	if !tar || got == nil || got.Name != arch {
		t.Fatalf("got %+v tar=%v", got, tar)
	}
}

func TestPickInstallAsset_skipsUtilityRawBinaries(t *testing.T) {
	t.Parallel()
	goos, goarch := "darwin", "arm64"
	mainName := "notify-" + goos + "-" + goarch
	assets := []Asset{
		{Name: "sound-preview-" + goos + "-" + goarch},
		{Name: mainName},
	}
	got, tar, err := PickInstallAsset(assets, goos, goarch)
	if err != nil {
		t.Fatal(err)
	}
	if tar || got == nil || got.Name != mainName {
		t.Fatalf("want main binary %q, got %+v tar=%v err=%v", mainName, got, tar, err)
	}
}

func TestPickInstallAsset_ambiguousRaw(t *testing.T) {
	t.Parallel()
	goos, goarch := "linux", "amd64"
	suf := "-" + goos + "-" + goarch
	assets := []Asset{
		{Name: "a" + suf},
		{Name: "b" + suf},
	}
	_, _, err := PickInstallAsset(assets, goos, goarch)
	if err == nil {
		t.Fatal("expected error")
	}
}
