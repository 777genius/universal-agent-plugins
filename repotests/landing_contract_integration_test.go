package pluginkitairepo_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestLandingSurface_LocalesLinksAndBrandingStayAligned(t *testing.T) {
	root := RepoRoot(t)
	landingRoot := filepath.Join(root, "landing")
	canonicalLogo := readRepoFile(t, root, "assets", "logo.svg")
	if got := readRepoFile(t, landingRoot, "public", "icon.svg"); got != canonicalLogo {
		t.Fatal("landing favicon must match the canonical Universal Agent Plugins logo")
	}
	if got := readRepoFile(t, root, "website", "public", "icon.svg"); got != canonicalLogo {
		t.Fatal("documentation favicon must match the canonical Universal Agent Plugins logo")
	}

	i18nBody, err := os.ReadFile(filepath.Join(landingRoot, "data", "i18n.ts"))
	if err != nil {
		t.Fatal(err)
	}
	i18n := string(i18nBody)

	// Publication is independent of the preserved legacy content model.
	assertLandingLocaleModel(t, landingRoot, i18n)

	docsLinksBody, err := os.ReadFile(filepath.Join(landingRoot, "composables", "useDocsLinks.ts"))
	if err != nil {
		t.Fatal(err)
	}
	docsLinks := string(docsLinksBody)

	mustContain(t, docsLinks, "supportBoundaryUrl")
	mustContain(t, docsLinks, "customLogicGuideUrl")
	if strings.Contains(docsLinks, "from '~/data/docsAvailability'") {
		mustContain(t, docsLinks, "resolveDocsLink(id, locale.value, configured)")
		for _, id := range []string{"home", "quickstart", "supportBoundary", "customLogicGuide"} {
			mustContain(t, docsLinks, "link('"+id+"'")
		}
		docs := readRepoFile(t, landingRoot, "data", "docsAvailability.ts")
		for _, contract := range []string{
			"home: ''", "quickstart: 'guide/quickstart.html'",
			"supportBoundary: 'reference/support-boundary.html'",
			"customLogicGuide: 'guide/build-custom-plugin-logic.html'",
			"https://777genius.github.io/universal-agent-plugins/docs/",
			"locale === 'ru' ? 'ru' : 'en'", "englishFallback: locale === 'uk'",
			"configuredUrl || defaultUrl", "source === owned",
			"source.startsWith(owned)", "/^[?#]/.test(suffix)",
			"return { url: source, language: null, englishFallback: false }",
		} {
			mustContain(t, docs, contract)
		}
		mustContain(t, docsLinks, "t('shell.docs.englishLabel', { label })")
	} else {
		// Foundation checkpoint: the original docs resolver is still wired.
		mustContain(t, docsLinks, "import type { LocaleCode } from '~/data/i18n'")
		mustContain(t, docsLinks, `const docsLocalePattern = /\/(en|ru|es|fr|zh)(?=\/|$)/;`)
		mustContain(t, docsLinks, "new Set<LocaleCode>(['en', 'ru', 'es', 'fr', 'zh'])")
		for _, path := range []string{"", "guide/quickstart.html", "reference/support-boundary.html", "guide/build-custom-plugin-logic.html"} {
			mustContain(t, docsLinks, "https://777genius.github.io/universal-agent-plugins/docs/en/"+path+"'")
		}
		mustContain(t, docsLinks, "replaceDocsLocale(")
		mustContain(t, docsLinks, ": 'en'")
	}

	releaseComposableBody, err := os.ReadFile(filepath.Join(landingRoot, "composables", "useReleaseDownloads.ts"))
	if err != nil {
		t.Fatal(err)
	}
	releaseComposable := string(releaseComposableBody)
	mustContain(t, releaseComposable, `'/api/releases/latest'`)
	mustContain(t, releaseComposable, `server: true`)
	mustContain(t, releaseComposable, `lazy: false`)
	mustContain(t, releaseComposable, `plugin-kit-ai_release_meta`)

	releaseRouteBody, err := os.ReadFile(filepath.Join(landingRoot, "server", "api", "releases", "latest.get.ts"))
	if err != nil {
		t.Fatal(err)
	}
	releaseRoute := string(releaseRouteBody)
	mustContain(t, releaseRoute, `https://api.github.com/repos/${githubRepo}/releases/latest`)
	mustContain(t, releaseRoute, `cache-control`)
	mustContain(t, releaseRoute, `RELEASE_CACHE_TTL`)

	sectionsBody, err := os.ReadFile(filepath.Join(landingRoot, "data", "sections.ts"))
	if err != nil {
		t.Fatal(err)
	}
	sections := string(sectionsBody)
	mustNotContain(t, sections, `"pricing"`)

	seoBody, err := os.ReadFile(filepath.Join(landingRoot, "composables", "usePageSeo.ts"))
	if err != nil {
		t.Fatal(err)
	}
	seo := string(seoBody)
	mustContain(t, seo, `https://777genius.github.io/universal-agent-plugins`)
	mustNotContain(t, seo, `hookplex.dev`)

	seoUtilsBody, err := os.ReadFile(filepath.Join(landingRoot, "utils", "seo.ts"))
	if err != nil {
		t.Fatal(err)
	}
	seoUtils := string(seoUtilsBody)
	mustContain(t, seoUtils, `'@type': 'SoftwareApplication'`)
	mustContain(t, seoUtils, `isAccessibleForFree: true`)
	mustContain(t, seoUtils, `offers:`)
	mustContain(t, seoUtils, `price: '0'`)
	mustContain(t, seoUtils, `priceCurrency: 'USD'`)
	mustContain(t, seoUtils, `installUrl: npmUrl`)
	mustContain(t, seoUtils, `softwareRequirements:`)
	mustNotContain(t, seoUtils, `hookplex.dev`)

	nuxtConfigBody, err := os.ReadFile(filepath.Join(landingRoot, "nuxt.config.ts"))
	if err != nil {
		t.Fatal(err)
	}
	nuxtConfig := string(nuxtConfigBody)
	mustContain(t, nuxtConfig, `https://777genius.github.io/universal-agent-plugins/docs/en/`)
	mustContain(t, nuxtConfig, `https://777genius.github.io/universal-agent-plugins/docs/sitemap.xml`)

	ruContentBody, err := os.ReadFile(filepath.Join(landingRoot, "content", "ru.json"))
	if err != nil {
		t.Fatal(err)
	}
	ruContent := string(ruContentBody)
	mustContain(t, ruContent, `https://777genius.github.io/universal-agent-plugins/docs/ru/guide/quickstart.html`)
	mustNotContain(t, ruContent, `"testimonials"`)
	mustContain(t, ruContent, `"title": "Проверяемый установочный скрипт"`)
	mustContain(t, ruContent, `"title": "npx · Node.js 22+"`)
	mustContain(t, ruContent, `"invocation": "npx universal-agent-plugins"`)
	mustContain(t, ruContent, `"note": "Без постоянной установки: npx скачивает проверенный CLI и запускает команду плагина за один шаг."`)
	mustNotContain(t, ruContent, `Public-beta обёртка`)
	mustNotContain(t, ruContent, `"pricing"`)
	mustNotContain(t, ruContent, `"hookplex":`)
	var ruDoc struct {
		ComparisonRows []struct {
			PluginKitAI struct {
				Status string `json:"status"`
			} `json:"pluginKitAi"`
		} `json:"comparisonRows"`
	}
	if err := json.Unmarshal(ruContentBody, &ruDoc); err != nil {
		t.Fatalf("parse landing/content/ru.json: %v", err)
	}
	if len(ruDoc.ComparisonRows) == 0 {
		t.Fatalf("landing/content/ru.json missing comparisonRows")
	}
	for _, row := range ruDoc.ComparisonRows {
		if row.PluginKitAI.Status != "yes" {
			t.Fatalf("landing/content/ru.json comparisonRows pluginKitAi.status = %q want yes", row.PluginKitAI.Status)
		}
	}

	enLocaleBody, err := os.ReadFile(filepath.Join(landingRoot, "locales", "en.json"))
	if err != nil {
		t.Fatal(err)
	}
	enLocale := string(enLocaleBody)
	mustContain(t, enLocale, `"copy": "Copy"`)
	mustContain(t, enLocale, `"copied": "Copied"`)
	mustContain(t, enLocale, `"comparison": "Why it works"`)
	mustContain(t, enLocale, `"pluginKitAi": "Universal Agent Plugins"`)
	mustContain(t, enLocale, `"generate": "generate outputs"`)
	mustContain(t, enLocale, `"viewAll": "View all"`)
	mustContain(t, enLocale, `"viewDetails": "View details"`)
	mustContain(t, enLocale, `"filterLabel": "Filter by workflow"`)
	mustContain(t, enLocale, `"pluginDetailTitle": "{plugin} | Universal Agent Plugins"`)
	mustContain(t, enLocale, `"catalogTitle": "Every first-party plugin in one searchable catalog"`)
	mustContain(t, enLocale, `"pluginsTitle": "Plugin catalog | Universal Agent Plugins"`)
	mustNotContain(t, enLocale, `"pricing"`)
	mustNotContain(t, enLocale, `Hookplex`)
	mustNotContain(t, enLocale, `"hookplex":`)
	mustNotContain(t, enLocale, `"generate": "generate"`)

	ruLocaleBody, err := os.ReadFile(filepath.Join(landingRoot, "locales", "ru.json"))
	if err != nil {
		t.Fatal(err)
	}
	ruLocale := string(ruLocaleBody)
	mustContain(t, ruLocale, `"copy": "Копировать"`)
	mustContain(t, ruLocale, `"copied": "Скопировано"`)
	mustContain(t, ruLocale, `"comparison": "Почему это работает"`)
	mustContain(t, ruLocale, `"pluginKitAi": "Universal Agent Plugins"`)
	assertLandingApprovedString(t, ruLocaleBody, "hero.demo.steps.generate", "собрать варианты", "создать результаты")
	mustContain(t, ruLocale, `"viewAll": "Смотреть все"`)
	mustContain(t, ruLocale, `"viewDetails": "Подробнее"`)
	mustContain(t, ruLocale, `"filterLabel": "Фильтр по сценариям"`)
	mustContain(t, ruLocale, `"pluginDetailTitle": "{plugin} | Universal Agent Plugins"`)
	assertLandingApprovedString(t, ruLocaleBody, "plugins.catalogTitle", "Вся первая линейка плагинов в одном каталоге с поиском", "Все собственные плагины в одном каталоге с поиском")
	mustContain(t, ruLocale, `"pluginsTitle": "Каталог плагинов | Universal Agent Plugins"`)
	mustNotContain(t, ruLocale, `"pricing"`)
	mustNotContain(t, ruLocale, `Hookplex`)
	mustNotContain(t, ruLocale, `"hookplex":`)
	mustNotContain(t, ruLocale, `"generate": "generate"`)

	for _, localeFile := range []string{"es.json", "fr.json", "zh.json"} {
		if _, err := os.Stat(filepath.Join(landingRoot, "locales", localeFile)); err != nil {
			t.Fatalf("expected locale file %s: %v", localeFile, err)
		}
		if _, err := os.Stat(filepath.Join(landingRoot, "content", localeFile)); err != nil {
			t.Fatalf("expected content file %s: %v", localeFile, err)
		}
	}

	headerBody, err := os.ReadFile(filepath.Join(landingRoot, "components", "layout", "AppHeader.vue"))
	if err != nil {
		t.Fatal(err)
	}
	header := string(headerBody)
	mustContain(t, header, `const router = useRouter();`)
	mustContain(t, header, `const homeHref = computed(() => router.resolve(homePath.value).href);`)
	mustContain(t, header, `const sectionHref = (sectionId: string) =>`)
	mustContain(t, header, "isHomePage.value ? `#${sectionId}` : `${homeHref.value}#${sectionId}`")
	mustContain(t, header, `rel="noopener noreferrer"`)
	mustNotContain(t, header, `nav.pricing`)

	downloadBody, err := os.ReadFile(filepath.Join(landingRoot, "components", "sections", "DownloadSection.vue"))
	if err != nil {
		t.Fatal(err)
	}
	download := string(downloadBody)
	mustContain(t, download, `download.copy`)
	mustContain(t, download, `download.copied`)
	mustContain(t, download, `CommandSnippetCard`)

	commandSnippetBody, err := os.ReadFile(filepath.Join(landingRoot, "components", "shared", "CommandSnippetCard.vue"))
	if err != nil {
		t.Fatal(err)
	}
	commandSnippet := string(commandSnippetBody)
	mustContain(t, commandSnippet, `navigator.clipboard?.writeText`)

	robotsBody, err := os.ReadFile(filepath.Join(landingRoot, "server", "routes", "robots.txt.ts"))
	if err != nil {
		t.Fatal(err)
	}
	robots := string(robotsBody)
	mustContain(t, robots, "config.public.docsSitemapUrl")
	mustContain(t, robots, "Sitemap: ${docsSitemapUrl}")
	mustContain(t, robots, "https://777genius.github.io/universal-agent-plugins")
	mustContain(t, robots, "docs/sitemap.xml")
	mustContain(t, robots, "Allow: /")

	logoBody, err := os.ReadFile(filepath.Join(landingRoot, "components", "common", "AppLogo.vue"))
	if err != nil {
		t.Fatal(err)
	}
	logo := string(logoBody)
	if strings.Contains(logo, "from '~/utils/localizedRoutes'") {
		mustContain(t, logo, "localizedPath('/', isKnownLocale(locale.value) ? locale.value : 'en')")
	} else {
		mustContain(t, logo, "const localePath = useLocalePath();")
		mustContain(t, strings.ReplaceAll(logo, `"`, "'"), "localePath('/')")
	}
	mustContain(t, logo, `<NuxtLink :to="homePath" class="app-logo"`)
	mustContain(t, logo, `:src="asset('icon.svg')"`)
	mustContain(t, logo, `Universal Agent Plugins`)
	mustNotContain(t, logo, `plugin-kit-ai`)
	mustNotContain(t, logo, `Hookplex`)

	heroBody, err := os.ReadFile(filepath.Join(landingRoot, "components", "sections", "HeroSection.vue"))
	if err != nil {
		t.Fatal(err)
	}
	hero := string(heroBody)
	mustContain(t, hero, `class="hero-section__logo"`)
	mustContain(t, hero, `:src="asset('icon.svg')"`)
	mustNotContain(t, hero, `<span class="hero-section__logo">`)

	indexBody, err := os.ReadFile(filepath.Join(landingRoot, "pages", "index.vue"))
	if err != nil {
		t.Fatal(err)
	}
	indexPage := string(indexBody)
	mustNotContain(t, indexPage, `LazyPricingSection`)

	pluginsPageBody, err := os.ReadFile(filepath.Join(landingRoot, "pages", "plugins", "index.vue"))
	if err != nil {
		t.Fatal(err)
	}
	pluginsPage := string(pluginsPageBody)
	mustContain(t, pluginsPage, `const registry = await useRegistryPage({ discovery: true })`)
	if strings.Contains(pluginsPage, "t('registryUi.directoryPage.title')") {
		mustContain(t, pluginsPage, "usePageSeo(() => t('registryUi.directoryPage.title')")
		assertLandingApprovedString(t, enLocaleBody, "registryUi.directoryPage.title", "Agent Plugins 1.0 Directory | Search 2,500+ Plugins")
	} else {
		mustContain(t, pluginsPage, `usePageSeo('Agent Plugins 1.0 Directory | Search 2,500+ Plugins'`)
	}
	mustContain(t, pluginsPage, `'@type': 'ItemList'`)
	mustContain(t, pluginsPage, `<PluginCatalog`)
	mustContain(t, pluginsPage, `:plugins="registry.plugins"`)

	pluginDetailPageBody, err := os.ReadFile(filepath.Join(landingRoot, "pages", "plugins", "[slug].vue"))
	if err != nil {
		t.Fatal(err)
	}
	pluginDetailPage := string(pluginDetailPageBody)
	mustContain(t, pluginDetailPage, `registry.plugins.find`)
	mustContain(t, pluginDetailPage, `projection: { kind: 'plugin', value: slug }`)
	mustContain(t, pluginDetailPage, `usePageSeo(`)
	mustContain(t, pluginDetailPage, `'@type': 'SoftwareSourceCode'`)
	mustContain(t, pluginDetailPage, `'@type': 'BreadcrumbList'`)
	mustContain(t, pluginDetailPage, `<InstallPanel`)
	mustContain(t, pluginDetailPage, `sourceUrl(plugin)`)

	agentPageBody, err := os.ReadFile(filepath.Join(landingRoot, "pages", "agents", "[client].vue"))
	if err != nil {
		t.Fatal(err)
	}
	agentPage := string(agentPageBody)
	mustContain(t, agentPage, `clientLandingBySlug.get`)
	mustContain(t, agentPage, `projection: { kind: 'client', value: client.id }`)
	mustContain(t, agentPage, `'@type': 'ItemList'`)
	mustContain(t, agentPage, `'@type': 'BreadcrumbList'`)
	mustContain(t, agentPage, `npx universal-agent-plugins add context7 --target`)

	enContentBody, err := os.ReadFile(filepath.Join(landingRoot, "content", "en.json"))
	if err != nil {
		t.Fatal(err)
	}
	enContent := string(enContentBody)
	mustContain(t, enContent, `"logoSrc": "context7.svg"`)
	mustContain(t, enContent, `"logoAlt": "GitHub logo"`)
	mustContain(t, enContent, `"slug": "context7"`)
	mustContain(t, enContent, `"highlights": [`)
	mustContain(t, enContent, `"useCases": [`)
	var enDoc struct {
		Plugins []struct {
			ID         string   `json:"id"`
			Categories []string `json:"categories"`
		} `json:"plugins"`
	}
	if err := json.Unmarshal(enContentBody, &enDoc); err != nil {
		t.Fatalf("parse landing/content/en.json: %v", err)
	}
	assertPluginCategories(t, enDoc.Plugins, "context7", []string{"docs", "research", "codeSearch"})

	ruContentAgainBody, err := os.ReadFile(filepath.Join(landingRoot, "content", "ru.json"))
	if err != nil {
		t.Fatal(err)
	}
	ruContentAgain := string(ruContentAgainBody)
	mustContain(t, ruContentAgain, `"logoSrc": "greptile.svg"`)
	mustContain(t, ruContentAgain, `"logoAlt": "Логотип GitLab"`)
	mustContain(t, ruContentAgain, `"slug": "greptile"`)
	mustContain(t, ruContentAgain, `"highlights": [`)
	mustContain(t, ruContentAgain, `"useCases": [`)
	var ruDocAgain struct {
		Plugins []struct {
			ID         string   `json:"id"`
			Categories []string `json:"categories"`
		} `json:"plugins"`
	}
	if err := json.Unmarshal(ruContentAgainBody, &ruDocAgain); err != nil {
		t.Fatalf("parse landing/content/ru.json for plugin catalog checks: %v", err)
	}
	assertPluginCategories(t, ruDocAgain.Plugins, "greptile", []string{"codeSearch", "review", "research"})

	matches, err := scanRemovedBranding(landingRoot)
	if err != nil {
		t.Fatalf("removed brand scan failed: %v", err)
	}
	if len(matches) > 0 {
		t.Fatalf("removed brand string still present:\n%s", strings.Join(matches, "\n"))
	}
}

func scanRemovedBranding(root string) ([]string, error) {
	searchRoots := []string{
		filepath.Join(root, "components"),
		filepath.Join(root, "content"),
		filepath.Join(root, "composables"),
		filepath.Join(root, "locales"),
		filepath.Join(root, "types"),
	}
	if _, err := exec.LookPath("rg"); err == nil {
		args := append([]string{"-n", "claude_agent_teams_ui|claude-agent-teams"}, searchRoots...)
		out, scanErr := exec.Command("rg", args...).CombinedOutput()
		if scanErr == nil {
			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			if len(lines) == 1 && lines[0] == "" {
				return nil, nil
			}
			return lines, nil
		}
		if exitErr, ok := scanErr.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, scanErr
	}

	patterns := []string{"claude_agent_teams_ui", "claude-agent-teams"}
	var matches []string
	for _, base := range searchRoots {
		walkErr := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			body, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			text := string(body)
			for _, pattern := range patterns {
				if strings.Contains(text, pattern) {
					rel, relErr := filepath.Rel(root, path)
					if relErr != nil {
						rel = path
					}
					matches = append(matches, filepath.ToSlash(rel)+": "+pattern)
					break
				}
			}
			return nil
		})
		if walkErr != nil {
			return nil, walkErr
		}
	}
	return matches, nil
}

func assertPluginCategories(t *testing.T, plugins []struct {
	ID         string   `json:"id"`
	Categories []string `json:"categories"`
}, wantID string, wantCategories []string) {
	t.Helper()
	for _, plugin := range plugins {
		if plugin.ID != wantID {
			continue
		}
		if strings.Join(plugin.Categories, "\x00") != strings.Join(wantCategories, "\x00") {
			t.Fatalf("plugin %s categories = %#v want %#v", wantID, plugin.Categories, wantCategories)
		}
		return
	}
	t.Fatalf("plugin %s missing from landing content", wantID)
}

// Keep the shipped locale set explicit so route and metadata coverage cannot drift silently.
func assertLandingLocaleModel(t *testing.T, root, source string) {
	t.Helper()
	literals := func(pattern string) []string {
		match := regexp.MustCompile(pattern).FindStringSubmatch(source)
		if len(match) != 2 {
			t.Fatalf("missing locale declaration matching %s", pattern)
		}
		var values []string
		for _, item := range regexp.MustCompile(`['"]([^'"]+)['"]`).FindAllStringSubmatch(match[1], -1) {
			values = append(values, item[1])
		}
		sort.Strings(values)
		return values
	}
	assertSet := func(got []string, want string) {
		if strings.Join(got, ",") != want {
			t.Fatalf("locale set = %v, want %s", got, want)
		}
	}
	assertSet(literals(`(?s)type\s+LegacyContentLocale\s*=([^;]+);`), "en,es,fr,ru,zh")
	mustContain(t, source, "LocaleCode = LegacyContentLocale")
	mustContain(t, source, "KnownLocale = LegacyContentLocale")
	assertSet(literals(`(?s)type\s+KnownLocale\s*=([^;]+);`), "ar,hi,pt,uk")
	assertSet(literals(`(?s)const\s+candidateLocales\s*=\s*\[([^]]+)\]`), "ar,en,es,fr,hi,pt,ru,uk,zh")
	mustContain(t, source, "publishedLocales = candidateLocales")
	metadata := regexp.MustCompile(`(?s)const\s+localeMetadata\s*=\s*\{(.*?)\}\s*as const`).FindStringSubmatch(source)
	if len(metadata) != 2 {
		t.Fatal("missing localeMetadata")
	}
	var codes []string
	for _, row := range regexp.MustCompile(`(\w+):\s*\{([^}]+)\}`).FindAllStringSubmatch(metadata[1], -1) {
		code := row[1]
		codes = append(codes, code)
		mustContain(t, row[2], "code: '"+code+"'")
		mustContain(t, row[2], "file: '"+code+".json'")

	}
	// Candidate metadata can precede a dictionary. Legacy and published
	// dictionaries must always remain present and valid.
	required := map[string]bool{"en": true, "es": true, "fr": true, "ru": true, "zh": true}
	for _, code := range []string{"en", "ru", "uk", "zh", "es", "hi", "ar", "pt", "fr"} {
		required[code] = true
	}
	for code := range required {
		var dictionary map[string]interface{}
		if err := json.Unmarshal([]byte(readRepoFile(t, root, "locales", code+".json")), &dictionary); err != nil {
			t.Fatalf("%s dictionary: %v", code, err)
		}
		if len(dictionary) == 0 {
			t.Fatalf("%s dictionary is empty", code)
		}
	}

	sort.Strings(codes)
	assertSet(codes, "ar,en,es,fr,hi,pt,ru,uk,zh")
	mustContain(t, source, "publishedLocales.map((code) => localeMetadata[code])")
	config := readRepoFile(t, root, "nuxt.config.ts")
	mustContain(t, config, "locales: [...supportedLocales]")
	mustContain(t, config, "defaultLocale: 'en'")
}

// Compare keyed dictionary values, allowing only approved staged wording.
// Vue I18n literal pipes render identically to the older unescaped strings.
func assertLandingApprovedString(t *testing.T, body []byte, key string, allowed ...string) {
	t.Helper()
	var value interface{}
	if err := json.Unmarshal(body, &value); err != nil {
		t.Fatal(err)
	}
	for _, part := range strings.Split(key, ".") {
		object, ok := value.(map[string]interface{})
		if !ok {
			t.Fatalf("missing dictionary path %s", key)
		}
		value = object[part]
	}
	got, ok := value.(string)
	if !ok {
		t.Fatalf("dictionary %s is not a string", key)
	}
	got = strings.ReplaceAll(got, "{'|'}", "|")
	for _, want := range allowed {
		if got == want {
			return
		}
	}
	t.Fatalf("dictionary %s = %q, want one of %q", key, got, allowed)
}
