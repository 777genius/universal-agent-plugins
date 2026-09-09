package pluginkitairepo_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestPagesSite_CombinesLandingRootAndDocsSubpath(t *testing.T) {
	root := RepoRoot(t)

	workflowBody, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "docs-pages.yml"))
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(workflowBody)
	mustContain(t, workflow, "name: Pages")
	mustContain(t, workflow, "working-directory: landing")
	mustContain(t, workflow, "pnpm generate")
	mustContain(t, workflow, "NUXT_APP_BASE_URL: /universal-agent-plugins/")
	mustContain(t, workflow, "DOCS_BASE_PATH: /universal-agent-plugins/docs/")
	mustContain(t, workflow, "go run ./cmd/agentplugins-registry-mirror")
	mustContain(t, workflow, "MIRROR_METADATA.json")
	mustContain(t, workflow, "uap-registry-mirror")
	mustContain(t, workflow, "777genius/universal-agent-plugins-registry")
	mustContain(t, workflow, "pnpm run build:pages")
	mustContain(t, workflow, "path: .pages-dist")

	packageBody, err := os.ReadFile(filepath.Join(root, "landing", "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	pkg := string(packageBody)
	mustContain(t, pkg, `"build:pages": "node ./scripts/build-pages-artifact.mjs"`)

	scriptBody, err := os.ReadFile(filepath.Join(root, "landing", "scripts", "build-pages-artifact.mjs"))
	if err != nil {
		t.Fatal(err)
	}
	script := string(scriptBody)
	mustContain(t, script, `const landingRoot = path.resolve(scriptDir, '..');`)
	mustContain(t, script, `const repoRoot = path.resolve(landingRoot, '..');`)
	mustContain(t, script, `const docsTarget = path.join(pagesDist, 'docs');`)
	mustContain(t, script, `await fs.cp(landingDist, pagesDist, { recursive: true });`)
	mustContain(t, script, `await fs.cp(docsDist, docsTarget, { recursive: true });`)

	nodeRuntimeExtractorBody, err := os.ReadFile(filepath.Join(root, "website", "tools", "extractors", "node-runtime.mjs"))
	if err != nil {
		t.Fatal(err)
	}
	nodeRuntimeExtractor := string(nodeRuntimeExtractorBody)
	mustContain(t, nodeRuntimeExtractor, `--tsconfig`)
	mustContain(t, nodeRuntimeExtractor, `../npm/plugin-kit-ai-runtime/tsconfig.docs.json`)

	nodeRuntimeTsconfigBody, err := os.ReadFile(filepath.Join(root, "npm", "plugin-kit-ai-runtime", "tsconfig.docs.json"))
	if err != nil {
		t.Fatal(err)
	}
	nodeRuntimeTsconfig := string(nodeRuntimeTsconfigBody)
	mustContain(t, nodeRuntimeTsconfig, `"ignoreDeprecations": "6.0"`)
	mustContain(t, nodeRuntimeTsconfig, `"include": ["index.d.ts"]`)

	siteBody, err := os.ReadFile(filepath.Join(root, "website", "tools", "config", "site.mjs"))
	if err != nil {
		t.Fatal(err)
	}
	site := string(siteBody)
	mustContain(t, site, `export const docsBasePath = process.env.DOCS_BASE_PATH || "/plugin-kit-ai/docs/";`)

	nuxtConfigBody, err := os.ReadFile(filepath.Join(root, "landing", "nuxt.config.ts"))
	if err != nil {
		t.Fatal(err)
	}
	nuxtConfig := string(nuxtConfigBody)
	mustContain(t, nuxtConfig, `'/api/registry/catalog'`)
	mustContain(t, nuxtConfig, "`/api/registry/plugin/${plugin.name}`")
	mustContain(t, nuxtConfig, `const sitemapRoutes =`)

	mirrorBody, err := os.ReadFile(filepath.Join(root, "cmd", "agentplugins-registry-mirror", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	mirror := string(mirrorBody)
	mustContain(t, mirror, `"security/latest.json"`)
	mustContain(t, mirror, `securityv1.Verify`)
	mustContain(t, mirror, `Security sequence has conflicting authenticated bytes`)

	pluginDetailPageBody, err := os.ReadFile(filepath.Join(root, "landing", "pages", "plugins", "[slug].vue"))
	if err != nil {
		t.Fatal(err)
	}
	pluginDetailPage := string(pluginDetailPageBody)
	mustContain(t, pluginDetailPage, `sourceUrl(plugin)`)
	mustContain(t, pluginDetailPage, `:aria-label="t('registryUi.detail.backToPluginDirectory')"`)

	docsConfigBody, err := os.ReadFile(filepath.Join(root, "website", ".vitepress", "config", "shared.ts"))
	if err != nil {
		t.Fatal(err)
	}
	docsConfig := string(docsConfigBody)
	mustContain(t, docsConfig, `logo: "/icon.svg"`)
	mustNotContain(t, docsConfig, "logo: `${docsBasePath}")

	robotsBody, err := os.ReadFile(filepath.Join(root, "landing", "server", "routes", "robots.txt.ts"))
	if err != nil {
		t.Fatal(err)
	}
	robots := string(robotsBody)
	mustContain(t, robots, `Sitemap: ${docsSitemapUrl}`)
}

// Resolve the executable selected by each step, including the job-wide PATH
// updates from action-setup and the step-local docs override. A single pnpm
// version cannot read both committed lockfiles; docs scripts also invoke pnpm.
func TestPagesWorkflows_PackageManagerAlignment(t *testing.T) {
	root := RepoRoot(t)
	managers := map[string]string{}
	for _, dir := range []string{"landing", "website"} {
		body, err := os.ReadFile(filepath.Join(root, dir, "package.json"))
		if err != nil {
			t.Fatal(err)
		}
		var pkg struct {
			PackageManager string `json:"packageManager"`
		}
		if err := json.Unmarshal(body, &pkg); err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(pkg.PackageManager, "pnpm@") {
			t.Fatalf("%s must declare pnpm", dir)
		}
		managers[dir] = strings.TrimPrefix(pkg.PackageManager, "pnpm@")
	}
	files, err := filepath.Glob(filepath.Join(root, ".github", "workflows", "*.yml"))
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, file := range files {
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var workflow struct {
			Jobs map[string]struct {
				Defaults struct {
					Run struct {
						Directory string `yaml:"working-directory"`
					}
				}
				Steps []struct {
					ID        string            `yaml:"id"`
					Uses      string            `yaml:"uses"`
					With      map[string]string `yaml:"with"`
					Directory string            `yaml:"working-directory"`
					Run       string            `yaml:"run"`
				} `yaml:"steps"`
			} `yaml:"jobs"`
		}
		if err := yaml.Unmarshal(body, &workflow); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		for jobName, job := range workflow.Jobs {
			relevant := false
			for _, step := range job.Steps {
				dir := step.Directory
				if dir == "" {
					dir = job.Defaults.Run.Directory
				}
				relevant = relevant || (managers[dir] != "" && strings.Contains(step.Run, "pnpm "))
			}
			if !relevant {
				continue
			}
			t.Run(filepath.Base(file)+"/"+jobName, func(t *testing.T) {
				active := ""
				outputs := map[string]string{}
				destinations := map[string]bool{}
				for _, step := range job.Steps {
					if strings.HasPrefix(step.Uses, "pnpm/action-setup@") {
						active = step.With["version"]
						if manifest := step.With["package_json_file"]; manifest != "" {
							want := managers[filepath.Dir(manifest)]
							if want == "" || (active != "" && active != want) {
								t.Fatalf("setup conflicts with %s", manifest)
							}
							active = want
						}
						if dest := step.With["dest"]; dest != "" {
							if destinations[dest] {
								t.Fatalf("toolchains overwrite %s", dest)
							}
							destinations[dest] = true
						}
						outputs[step.ID] = active
					}
					dir := step.Directory
					if dir == "" {
						dir = job.Defaults.Run.Directory
					}
					want := managers[dir]
					if want == "" || !strings.Contains(step.Run, "pnpm ") {
						continue
					}
					checked++
					selected := active
					invocations := 0
					for _, line := range strings.Split(step.Run, "\n") {
						line = strings.TrimSpace(line)
						for id, version := range outputs {
							// Export is intentional: nested package scripts must use
							// the same manager as the outer invocation.
							if line == `export PATH="${{ steps.`+id+`.outputs.bin_dest }}:$PATH"` {
								selected = version
							}
						}
						if strings.HasPrefix(line, "pnpm ") {
							invocations++
							if selected != want {
								t.Errorf("%s: selected pnpm@%s, packageManager requires pnpm@%s", dir, selected, want)
							}
							if strings.HasPrefix(line, "pnpm install") && !strings.Contains(line, "--frozen-lockfile") {
								t.Errorf("%s install must remain frozen", dir)
							}
						}
					}
					if invocations == 0 {
						t.Fatalf("unrecognized pnpm invocation in %s: %s", dir, step.Run)
					}
				}
			})
		}
	}
	if checked == 0 {
		t.Fatal("no package-manager steps checked")
	}
}
