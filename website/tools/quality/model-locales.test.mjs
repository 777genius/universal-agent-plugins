import test from "node:test";
import assert from "node:assert/strict";
import { docsLocales, historicalLocales } from "../config/locales.mjs";
import { generatedLocaleErrors, requiredEntityLocales } from "./model-locales.mjs";

const currentId = "prepared-authoring-v2:agentplugins author";
const currentPage = (locale) => ({ canonicalId: currentId, locale, relativePath: `${locale}/api/cli/prepared-authoring-v2-agentplugins-author.md` });
const archivedPage = (locale) => ({ canonicalId: "command:plugin-kit-ai", locale, relativePath: `${locale}/api/cli/plugin-kit-ai.md` });

test("current authored and mirrored pages retain the nine-locale contract", () => {
  assert.equal(docsLocales.length, 9);
  assert.deepEqual(requiredEntityLocales({ sourceKind: "hand-authored", localeStrategy: "canonical-en" }), docsLocales);
  const entity = { canonicalId: currentId, localeStrategy: "mirrored" };
  assert.deepEqual(generatedLocaleErrors(docsLocales.map(currentPage), [entity]), []);
  for (const missing of docsLocales) {
    assert.match(generatedLocaleErrors(docsLocales.filter((locale) => locale !== missing).map(currentPage), [entity]).join("\n"), /Generated locale parity failure/);
  }
});

test("archive coverage stays exactly five locales outside the public registry", () => {
  assert.deepEqual(historicalLocales, ["en", "ru", "es", "fr", "zh"]);
  assert.deepEqual(generatedLocaleErrors(historicalLocales.map(archivedPage), []), []);
  for (const missing of historicalLocales) {
    assert.match(generatedLocaleErrors(historicalLocales.filter((locale) => locale !== missing).map(archivedPage), []).join("\n"), /Generated locale parity failure/);
  }
  for (const extra of docsLocales.filter((locale) => !historicalLocales.includes(locale))) {
    assert.match(generatedLocaleErrors([...historicalLocales, extra].map(archivedPage), []).join("\n"), /Generated locale parity failure/);
  }
});

test("English-only references require a registry entry and honor explicit translations", () => {
  const entity = { canonicalId: currentId, sourceKind: "authoring-docs-adapter", localeStrategy: "canonical-en", pathEn: "/en/api/cli/prepared-authoring-v2-agentplugins-author" };
  assert.deepEqual(generatedLocaleErrors([currentPage("en")], [entity]), []);
  assert.match(generatedLocaleErrors([currentPage("en")], []).join("\n"), /missing from the public registry/);
  assert.match(generatedLocaleErrors([currentPage("ru")], [entity]).join("\n"), /Generated locale parity failure/);
  const translated = { ...entity, pathUk: "/uk/api/cli/prepared-authoring-v2-agentplugins-author" };
  assert.deepEqual(generatedLocaleErrors([currentPage("en"), currentPage("uk")], [translated]), []);
  assert.match(generatedLocaleErrors([currentPage("en")], [translated]).join("\n"), /Generated locale parity failure/);
});

test("locale metadata, duplicate files and mixed public/archive IDs fail closed", () => {
  const archived = historicalLocales.map(archivedPage);
  assert.match(generatedLocaleErrors([...archived, archivedPage("en")], []).join("\n"), /Duplicate generated locale/);
  assert.match(generatedLocaleErrors([{ ...archivedPage("en"), locale: "ru" }], []).join("\n"), /mismatched locale/);
  assert.match(generatedLocaleErrors([archivedPage("de")], []).join("\n"), /unsupported.*locale/);
  assert.match(generatedLocaleErrors([{ ...archivedPage("en"), locale: "" }], []).join("\n"), /missing locale frontmatter/);
  assert.match(generatedLocaleErrors([{ ...archivedPage("en"), canonicalId: "" }], []).join("\n"), /missing canonicalId frontmatter/);
  assert.match(generatedLocaleErrors([...archived, { ...currentPage("uk"), canonicalId: archived[0].canonicalId }], []).join("\n"), /spans current and archived routes/);
});
