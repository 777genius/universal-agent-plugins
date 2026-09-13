package main

import (
	"flag"
	"log"
	"os"
	"strings"

	"github.com/777genius/plugin-kit-ai/internal/homebrewformula"
)

func main() {
	var (
		tag          = flag.String("tag", "", "release tag (for example plugin-kit-ai-v2.0.1)")
		repo         = flag.String("repo", "777genius/plugin-kit-ai", "repository owner/repo")
		checksums    = flag.String("checksums", "", "path to checksums.txt")
		downloadBase = flag.String("download-base", "", "release download base URL")
		output       = flag.String("output", "", "formula output path")
	)
	flag.Parse()

	if strings.TrimSpace(*tag) == "" || strings.TrimSpace(*checksums) == "" || strings.TrimSpace(*downloadBase) == "" || strings.TrimSpace(*output) == "" {
		flag.Usage()
		os.Exit(2)
	}

	formula, err := homebrewformula.Build(*tag, *repo, *checksums, *downloadBase)
	if err != nil {
		log.Fatal(err)
	}
	body, err := homebrewformula.Generate(formula)
	if err != nil {
		log.Fatal(err)
	}
	if err := homebrewformula.Write(*output, body); err != nil {
		log.Fatal(err)
	}
}
