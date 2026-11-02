import assert from 'node:assert/strict';
import { mkdirSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import type { SkillsCatalogPage } from '../types/skillsCatalog.ts';
import { skillInstallCommand, skillPublisherUrl, skillRepositoryUrl } from '../utils/skillsCatalog.ts';

// Bounded live sampling: it does NOT claim to audit the entire catalog. The
// publisher API may rate limit deeper pages; preserve partial evidence.
const views = [
  { name: 'popular', maxPages: 5 },
  { name: 'trending', maxPages: 2 },
  { name: 'hot', maxPages: 2 },
] as const;
const report = {
  checked_at: new Date().toISOString(),
  scope: 'bounded public API sample; not full catalog coverage',
  full_catalog_verified: false,
  pages: [] as Array<{ view: string; page: number; count: number; total?: number }>,
  distinct: 0,
  installable: 0,
  github: 0,
  https_publisher: 0,
  source_unavailable: 0,
  invalid_selector: 0,
  blocked_samples: [] as Array<{ id: string; reason: string }>,
  incomplete: [] as string[],
};
const seen = new Set<string>();
for (const view of views) {
  for (let page = 0; page < view.maxPages; page++) {
    try {
      const response = await fetch(
        `https://api.universal-agent-plugins.com/v1/skills?view=${view.name}&page=${page}`,
        {
          headers: { Accept: 'application/json', Origin: 'https://universal-agent-plugins.com' },
          redirect: 'error',
          signal: AbortSignal.timeout(15000),
        },
      );
      if (!response.ok) {
        report.incomplete.push(`${view.name}/${page}: HTTP ${response.status}`);
        break;
      }
      const result = (await response.json()) as SkillsCatalogPage;
      assert.equal(result.view, view.name);
      assert.equal(result.page, page);
      assert.ok(Array.isArray(result.items) && result.items.length <= 200);
      report.pages.push({
        view: view.name, page, count: result.items.length,
        ...(typeof result.total === 'number' ? { total: result.total } : {}),
      });
      for (const item of result.items) {
        if (seen.has(item.id)) continue;
        seen.add(item.id);
        const github = Boolean(skillRepositoryUrl(item));
        const publisher = Boolean(skillPublisherUrl(item));
        const installable = Boolean(skillInstallCommand(item));
        report.github += Number(github);
        report.https_publisher += Number(publisher);
        report.installable += Number(installable);
        if (!installable) {
          const reason = github || publisher ? 'invalid-selector' : 'source-unavailable';
          report.source_unavailable += Number(reason === 'source-unavailable');
          report.invalid_selector += Number(reason === 'invalid-selector');
          if (report.blocked_samples.length < 24) report.blocked_samples.push({ id: item.id, reason });
        }
      }
      if (!result.hasMore || !result.items.length) break;
    } catch (error) {
      report.incomplete.push(`${view.name}/${page}: ${error instanceof Error ? error.name : 'request-error'}`);
      break;
    }
  }
}
report.distinct = seen.size;
const output = process.env.UAP_SKILLS_INSTALL_REPORT_DIR;
if (output) {
  mkdirSync(output, { recursive: true });
  writeFileSync(join(output, 'coverage-sample.json'), JSON.stringify(report, null, 2) + '\n');
}
console.log(JSON.stringify(report, null, 2));
assert.ok(report.distinct > 0, 'No live Skills could be sampled');
