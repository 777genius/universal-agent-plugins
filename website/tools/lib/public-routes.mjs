const localePrefix = "(?:en|ru|es|fr|zh)";
const currentPage = new RegExp(`^${localePrefix}/(?:index\\.md|use/.+\\.md|build/.+\\.md|guide/quickstart\\.md|reference/client-compatibility\\.md|api/cli/prepared-authoring-v2-agentplugins-author(?:-[^/]*)?\\.md)$`);

export function isRetiredArchive(relativePath) {
  const pathOnly = relativePath.replace(/\\/g, "/").split(/[?#]/, 1)[0].replace(/^\/+/, "");
  const normalized = pathOnly.endsWith(".md")
    ? pathOnly
    : pathOnly.endsWith("/")
      ? `${pathOnly}index.md`
      : `${pathOnly}.md`;
  return new RegExp(`^${localePrefix}/`).test(normalized) && !currentPage.test(normalized);
}
