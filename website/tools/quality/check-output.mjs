import fs from "node:fs/promises";
import path from "node:path";
import { docsBaseUrl, runtimeRoot, websiteRoot } from "../config/site.mjs";
import { listMarkdownFiles } from "../lib/fs.mjs";

const distRoot = path.join(websiteRoot, "dist");
const repoRoot = path.resolve(websiteRoot, "..");
const htmlFiles = (await listHtmlFiles(distRoot)).sort();
const generatedMarkdownFiles = (await listMarkdownFiles(runtimeRoot)).sort();
const editPrefix = `https://github.com/777genius/universal-agent-plugins/edit/main/website/source/`;

let hasError = false;
for (const filePath of htmlFiles) {
  const body = await fs.readFile(filePath, "utf8");
  if (body.includes("maintainer-docs")) {
    console.error(`Internal docs leaked into built output: ${filePath}`);
    hasError = true;
  }
  if (/href="\/(en|ru|es|fr|zh)\//.test(body) || /src="\/assets\//.test(body)) {
    console.error(`Root-relative path detected in built output: ${filePath}`);
    hasError = true;
  }
}

const gateway = await fs.readFile(path.join(distRoot, "index.html"), "utf8");
if (!gateway.includes("noindex,follow")) {
  console.error("Gateway page is missing robots noindex,follow.");
  hasError = true;
}

const notFound = await fs.readFile(path.join(distRoot, "404.html"), "utf8");
if (!notFound.includes("noindex,follow")) {
  console.error("404 page is missing robots noindex,follow.");
  hasError = true;
}

const sitemap = await fs.readFile(path.join(distRoot, "sitemap.xml"), "utf8");
if (sitemap.includes(`<loc>${docsBaseUrl}</loc>`)) {
  console.error("Gateway root leaked into sitemap.xml.");
  hasError = true;
}
if (/<loc>[^<]*\/404\/?<\/loc>/.test(sitemap)) {
  console.error("The not-found document leaked into sitemap.xml.");
  hasError = true;
}

const robots = await fs.readFile(path.join(distRoot, "robots.txt"), "utf8");
if (!robots.includes(`Sitemap: ${new URL("sitemap.xml", docsBaseUrl).toString()}`)) {
  console.error("robots.txt is missing the sitemap declaration.");
  hasError = true;
}

for (const filePath of generatedMarkdownFiles) {
  const body = await fs.readFile(filePath, "utf8");
  if (!/^generated:\s*true$/m.test(body)) {
    continue;
  }
  const titleMatch = body.match(/^title:\s*"([^"]+)"$/m);
  const headingMatch = body.match(/^#\s+(.+)$/m);
  if (!titleMatch || !headingMatch) {
    continue;
  }
  const title = titleMatch[1].trim();
  const heading = headingMatch[1].trim();
  if (title !== heading) {
    console.error(`Generated page title/H1 mismatch: ${filePath} ("${title}" vs "${heading}")`);
    hasError = true;
  }
}

const handAuthoredHome = await fs.readFile(path.join(distRoot, "en", "index.html"), "utf8");
if (!handAuthoredHome.includes(`${editPrefix}en/index.md`)) {
  console.error("Hand-authored EN home page is missing its edit link.");
  hasError = true;
}
if (!handAuthoredHome.includes("Default Start")) {
  console.error("EN home page is missing its expected default-start framing.");
  hasError = true;
}
if (!handAuthoredHome.includes("Supported Node And Python Paths")) {
  console.error("EN home page is missing its expected non-Go support block.");
  hasError = true;
}
if (handAuthoredHome.includes("delivery model") || handAuthoredHome.includes("repo-managed integration")) {
  console.error("EN home page still contains heavy front-door jargon.");
  hasError = true;
}

const generatedCli = await fs.readFile(path.join(distRoot, "en", "api", "cli", "plugin-kit-ai.html"), "utf8");
if (generatedCli.includes(`${editPrefix}en/api/cli/plugin-kit-ai.md`)) {
  console.error("Generated CLI page generated a hand-authored edit link.");
  hasError = true;
}
if (!generatedCli.includes(">Source<")) {
  console.error("Generated CLI page is missing the Source CTA.");
  hasError = true;
}

const latestRelease = await fs.readFile(path.join(distRoot, "en", "releases", "v1-1-2.html"), "utf8");
if (!latestRelease.includes("Why This Release Matters")) {
  console.error("Latest public release page is missing its expected headline content.");
  hasError = true;
}

const latestReleaseAlias = await fs.readFile(path.join(distRoot, "releases", "v1-1-2.html"), "utf8");
if (!latestReleaseAlias.includes('http-equiv="refresh"')) {
  console.error("Latest public release alias is missing its redirect metadata.");
  hasError = true;
}
const latestReleaseUrl = new URL("en/releases/v1-1-2", docsBaseUrl).toString();
if (!latestReleaseAlias.includes(latestReleaseUrl)) {
  console.error("Latest public release alias is missing its canonical EN destination.");
  hasError = true;
}

const guideAlias = await fs.readFile(path.join(distRoot, "guide", "index.html"), "utf8");
const guideUrl = new URL("en/guide/", docsBaseUrl).toString();
if (!guideAlias.includes(guideUrl)) {
  console.error("Guide alias is missing its canonical EN destination.");
  hasError = true;
}

const productionReadiness = await fs.readFile(path.join(distRoot, "en", "guide", "production-readiness.html"), "utf8");
if (!productionReadiness.includes("Pick The Right Path On Purpose")) {
  console.error("Production Readiness page is missing its expected checklist structure.");
  hasError = true;
}

const quickstart = await fs.readFile(path.join(distRoot, "en", "guide", "quickstart.html"), "utf8");
if (!quickstart.includes("Recommended Default")) {
  console.error("Quickstart page is missing its expected canonical default flow.");
  hasError = true;
}
if (!quickstart.includes("Supported Node And Python Paths")) {
  console.error("Quickstart page is missing its expected non-Go support block.");
  hasError = true;
}
if (!quickstart.includes("If You Are Intentionally Starting On Node Or Python")) {
  console.error("Quickstart page is missing its expected intentional non-Go flow.");
  hasError = true;
}
if (!quickstart.includes("What You Get")) {
  console.error("Quickstart page is missing its expected outcome-first section.");
  hasError = true;
}
if (!quickstart.includes("What To Do Next")) {
  console.error("Quickstart page is missing its expected next-steps section.");
  hasError = true;
}
if (quickstart.includes("runtime language") || quickstart.includes("repo-managed integration")) {
  console.error("Quickstart page still contains heavy front-door jargon.");
  hasError = true;
}

const ciIntegration = await fs.readFile(path.join(distRoot, "en", "guide", "ci-integration.html"), "utf8");
if (!ciIntegration.includes("The Minimal CI Gate")) {
  console.error("CI Integration page is missing its expected CI gate section.");
  hasError = true;
}

const teamReadyPlugin = await fs.readFile(path.join(distRoot, "en", "guide", "team-ready-plugin.html"), "utf8");
if (!teamReadyPlugin.includes("The repo is ready when another teammate can clone it")) {
  console.error("Team-Ready Plugin page is missing its expected handoff rule.");
  hasError = true;
}

const examplesAndRecipes = await fs.readFile(path.join(distRoot, "en", "guide", "examples-and-recipes.html"), "utf8");
if (!examplesAndRecipes.includes("Production Plugin Examples")) {
  console.error("Examples And Recipes page is missing its expected examples section.");
  hasError = true;
}

const chooseStarter = await fs.readFile(path.join(distRoot, "en", "guide", "choose-a-starter.html"), "utf8");
if (!chooseStarter.includes("Starter Matrix")) {
  console.error("Choose A Starter page is missing its expected starter-matrix section.");
  hasError = true;
}

const chooseDeliveryModel = await fs.readFile(path.join(distRoot, "en", "guide", "choose-delivery-model.html"), "utf8");
if (!chooseDeliveryModel.includes("The Two Modes")) {
  console.error("Choose Delivery Model page is missing its expected mode-comparison section.");
  hasError = true;
}

const bundleHandoff = await fs.readFile(path.join(distRoot, "en", "guide", "bundle-handoff.html"), "utf8");
if (!bundleHandoff.includes("What It Covers")) {
  console.error("Bundle Handoff page is missing its expected capability-coverage section.");
  hasError = true;
}

const packageAndWorkspaceTargets = await fs.readFile(
  path.join(distRoot, "en", "guide", "package-and-workspace-targets.html"),
  "utf8"
);
if (!packageAndWorkspaceTargets.includes("The Short Rule")) {
  console.error("Package And Workspace Targets page is missing its expected decision-rule section.");
  hasError = true;
}

const whatYouCanBuild = await fs.readFile(path.join(distRoot, "en", "guide", "what-you-can-build.html"), "utf8");
if (!whatYouCanBuild.includes("One Repo, Many Supported Outputs")) {
  console.error("What You Can Build page is missing its expected product-shape section.");
  hasError = true;
}
if (!whatYouCanBuild.includes("Choosing Node or Python does not force you to decide every packaging or integration detail on day one")) {
  console.error("What You Can Build page is missing its expected language-vs-shipping guidance.");
  hasError = true;
}

const oneProjectMultipleTargets = await fs.readFile(
  path.join(distRoot, "en", "guide", "one-project-multiple-targets.html"),
  "utf8"
);
if (!oneProjectMultipleTargets.includes("The Short Rule")) {
  console.error("One Project, Multiple Targets page is missing its expected mental-model section.");
  hasError = true;
}

const chooseTarget = await fs.readFile(path.join(distRoot, "en", "guide", "choose-a-target.html"), "utf8");
if (!chooseTarget.includes("Target Directory")) {
  console.error("Choose A Target page is missing its expected target-decision section.");
  hasError = true;
}
if (!chooseTarget.includes("that changes the language choice")) {
  console.error("Choose A Target page is missing its expected language-vs-target clarification.");
  hasError = true;
}

const authoringArchitecture = await fs.readFile(path.join(distRoot, "en", "concepts", "authoring-architecture.html"), "utf8");
if (!authoringArchitecture.includes("The Core Shape")) {
  console.error("Authoring Architecture page is missing its expected core-shape section.");
  hasError = true;
}

const choosingRuntime = await fs.readFile(path.join(distRoot, "en", "concepts", "choosing-runtime.html"), "utf8");
if (!choosingRuntime.includes("Safe Default Matrix")) {
  console.error("Choosing Runtime page is missing its expected decision matrix.");
  hasError = true;
}

const targetModel = await fs.readFile(path.join(distRoot, "en", "concepts", "target-model.html"), "utf8");
if (!targetModel.includes("Quick Rule")) {
  console.error("Target Model page is missing its expected quick-rule section.");
  hasError = true;
}

const glossary = await fs.readFile(path.join(distRoot, "en", "reference", "glossary.html"), "utf8");
if (!glossary.includes("Authored State")) {
  console.error("Glossary page is missing its expected term content.");
  hasError = true;
}
if (!glossary.includes("Use this page when a docs term is slowing you down")) {
  console.error("Glossary page is missing its expected plain-language framing.");
  hasError = true;
}

const faq = await fs.readFile(path.join(distRoot, "en", "reference", "faq.html"), "utf8");
if (!faq.includes("Are npm And PyPI")) {
  console.error("FAQ page is missing its expected install-vs-runtime question.");
  hasError = true;
}
if (!faq.includes("Different paths carry different support promises")) {
  console.error("FAQ page is missing its expected support-difference answer.");
  hasError = true;
}

const troubleshooting = await fs.readFile(path.join(distRoot, "en", "reference", "troubleshooting.html"), "utf8");
if (!troubleshooting.includes("Use this page when the workflow stops moving")) {
  console.error("Troubleshooting page is missing its expected recovery framing.");
  hasError = true;
}
if (!troubleshooting.includes("Treat this as signal, not noise")) {
  console.error("Troubleshooting page is missing its expected validate guidance.");
  hasError = true;
}

const repositoryStandard = await fs.readFile(path.join(distRoot, "en", "reference", "repository-standard.html"), "utf8");
if (!repositoryStandard.includes("The Main Rule")) {
  console.error("Repository Standard page is missing its expected main-rule section.");
  hasError = true;
}

const supportBoundary = await fs.readFile(path.join(distRoot, "en", "reference", "support-boundary.html"), "utf8");
if (!supportBoundary.includes("Safe Defaults")) {
  console.error("Support Boundary page is missing its expected safety framing.");
  hasError = true;
}
if (!supportBoundary.includes("How This Maps To The Formal Contract")) {
  console.error("Support Boundary page is missing its expected public-to-formal mapping.");
  hasError = true;
}

const conceptsIndex = await fs.readFile(path.join(distRoot, "en", "concepts", "index.html"), "utf8");
if (conceptsIndex.includes("Managed Project Model")) {
  console.error("Concepts index still leads with the old Managed Project Model framing.");
  hasError = true;
}
if (!conceptsIndex.includes("How plugin-kit-ai Works")) {
  console.error("Concepts index is missing the merged how-it-works concept entry.");
  hasError = true;
}

const managedProjectModel = await fs.readFile(path.join(distRoot, "en", "concepts", "managed-project-model.html"), "utf8");
if (!managedProjectModel.includes("How plugin-kit-ai Works")) {
  console.error("Merged how-it-works concept page is missing its expected title.");
  hasError = true;
}
if (!managedProjectModel.includes("source -&gt; generate -&gt; validate --strict -&gt; handoff")) {
  console.error("Merged how-it-works concept page is missing its expected core loop.");
  hasError = true;
}

const guideIndex = await fs.readFile(path.join(distRoot, "en", "guide", "index.html"), "utf8");
if (!guideIndex.includes("Use this as the product map")) {
  console.error("Guide index no longer distinguishes the product-map page.");
  hasError = true;
}
if (!guideIndex.includes("when you need to decide whether one repo should grow")) {
  console.error("Guide index no longer distinguishes the scaling guide.");
  hasError = true;
}

const stabilityModel = await fs.readFile(path.join(distRoot, "en", "concepts", "stability-model.html"), "utf8");
if (!stabilityModel.includes("How To Read Recommended")) {
  console.error("Stability Model page is missing its expected Recommended-language framing.");
  hasError = true;
}

const referenceIndex = await fs.readFile(path.join(distRoot, "en", "reference", "index.html"), "utf8");
if (!referenceIndex.includes("what is safe to standardize")) {
  console.error("Reference index is missing its expected user-question framing.");
  hasError = true;
}
if (referenceIndex.includes("compact lane matrix")) {
  console.error("Reference index still uses matrix-first framing.");
  hasError = true;
}

const versionAndCompatibility = await fs.readFile(
  path.join(distRoot, "en", "reference", "version-and-compatibility.html"),
  "utf8"
);
if (!versionAndCompatibility.includes("Recommended Lanes And Formal Tiers")) {
  console.error("Version And Compatibility page is missing its expected recommended-lane mapping.");
  hasError = true;
}

const supportPolicy = await fs.readFile(path.join(repoRoot, "docs", "SUPPORT.md"), "utf8");
if (!supportPolicy.includes("## Recommended Production Lanes")) {
  console.error("SUPPORT.md is missing its expected Recommended Production Lanes section.");
  hasError = true;
}
if (!supportPolicy.includes("## Public Language And Formal Terms")) {
  console.error("SUPPORT.md is missing its expected public-to-formal term mapping.");
  hasError = true;
}
if (!supportPolicy.includes("## Exact Contract Vocabulary")) {
  console.error("SUPPORT.md is missing its expected exact contract vocabulary section.");
  hasError = true;
}

const releasesIndex = await fs.readFile(path.join(distRoot, "en", "releases", "index.html"), "utf8");
if (!releasesIndex.toLowerCase().includes("what changed for users")) {
  console.error("Releases index is missing its expected user-facing summary framing.");
  hasError = true;
}
if (releasesIndex.includes("first-class public path") || releasesIndex.includes("replace friction")) {
  console.error("Releases index still contains maintainer-centric wording.");
  hasError = true;
}

if (hasError) {
  process.exit(1);
}

async function listHtmlFiles(rootDir) {
  const out = [];
  await walk(rootDir, out);
  return out;
}

async function walk(currentDir, out) {
  const entries = await fs.readdir(currentDir, { withFileTypes: true });
  for (const entry of entries) {
    const currentPath = path.join(currentDir, entry.name);
    if (entry.isDirectory()) {
      await walk(currentPath, out);
      continue;
    }
    if (entry.name.endsWith(".html")) {
      out.push(currentPath);
    }
  }
}
