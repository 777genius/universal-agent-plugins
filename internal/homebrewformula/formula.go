package homebrewformula

import (
	"bytes"
	"text/template"
)

type Asset struct {
	GOOS   string
	GOARCH string
	Name   string
	SHA256 string
	URL    string
}

type Formula struct {
	Tag       string
	Version   string
	Repo      string
	ClassName string
	Desc      string
	Homepage  string
	Assets    []Asset
}

var supportedPairs = []struct {
	goos   string
	goarch string
}{
	{goos: "darwin", goarch: "amd64"},
	{goos: "darwin", goarch: "arm64"},
	{goos: "linux", goarch: "amd64"},
	{goos: "linux", goarch: "arm64"},
}

func Build(tag, repo, checksumsPath, downloadBase string) (Formula, error) {
	return buildFormula(tag, repo, checksumsPath, downloadBase)
}

func Generate(f Formula) ([]byte, error) {
	return generateFormula(f)
}

func Write(outputPath string, body []byte) error {
	return writeFormula(outputPath, body)
}

func generateFormula(f Formula) ([]byte, error) {
	const tpl = `class {{ .ClassName }} < Formula
  desc "{{ .Desc }}"
  homepage "{{ .Homepage }}"
  version "{{ .Version }}"

  on_macos do
    if Hardware::CPU.arm?
      url "{{ assetURL .Assets "darwin" "arm64" }}"
      sha256 "{{ assetSHA .Assets "darwin" "arm64" }}"
    else
      url "{{ assetURL .Assets "darwin" "amd64" }}"
      sha256 "{{ assetSHA .Assets "darwin" "amd64" }}"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "{{ assetURL .Assets "linux" "arm64" }}"
      sha256 "{{ assetSHA .Assets "linux" "arm64" }}"
    else
      url "{{ assetURL .Assets "linux" "amd64" }}"
      sha256 "{{ assetSHA .Assets "linux" "amd64" }}"
    end
  end

  def install
    bin.install "plugin-kit-ai"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/plugin-kit-ai version")
  end
end
`

	t, err := template.New("formula").Funcs(template.FuncMap{
		"assetURL": func(assets []Asset, goos, goarch string) string {
			return mustAsset(assets, goos, goarch).URL
		},
		"assetSHA": func(assets []Asset, goos, goarch string) string {
			return mustAsset(assets, goos, goarch).SHA256
		},
	}).Parse(tpl)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, f); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
