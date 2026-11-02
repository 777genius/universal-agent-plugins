import assert from 'node:assert/strict';
import { mkdirSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import type { SkillsCatalogPage, SkillListing } from '../types/skillsCatalog.ts';
import {
  skillInstallCommand,
  skillPublisherUrl,
  skillRepositoryUrl,
} from '../utils/skillsCatalog.ts';

// Live, read-only API contract: never replace requests with fixtures.
// This is intentionally separate from the GitHub Pages release proof.
assert.equal(process.env.UAP_E2E_LIVE_SKILLS, '1', 'Explicit live Skills opt-in required');
const base = 'https://api.universal-agent-plugins.com';
const checkedAt = new Date().toISOString();

async function read(url: string): Promise<Response> {
  const response = await fetch(url, {
    headers: {
      Accept: 'application/json',
      ...(url.includes('/v1/skills?') ? { Origin: 'https://universal-agent-plugins.com' } : {}),
    },
    signal: AbortSignal.timeout(15000),
    redirect: 'error',
  });
  return response;
}

const report = {
  checked_at: checkedAt,
  api: base,
  health_status: 0,
  browse_status: 0,
  cors_allowed_origin: null as string | null,
  view: 'popular',
  total_items: 0,
  installable: 0,
  github: 0,
  https_publisher: 0,
  unsupported: 0,
  legacy_publisher_recovered: 0,
  blocked_samples: [] as Array<{ id: string; reason: string }>,
  failure: null as string | null,
};

try {
  const health = await read(base + '/healthz');
  report.health_status = health.status;
  assert.equal(health.status, 200, 'Skills API health must be HTTP 200');
  const response = await read(base + '/v1/skills?view=popular&page=0');
  report.browse_status = response.status;
  assert.equal(response.status, 200, 'Real Skills API browse must return HTTP 200');
  report.cors_allowed_origin = response.headers.get('access-control-allow-origin');
  assert.equal(report.cors_allowed_origin, 'https://universal-agent-plugins.com', 'Production CORS origin mismatch');
  const page = (await response.json()) as SkillsCatalogPage;
  assert.ok(Array.isArray(page.items) && page.items.length > 0, 'Empty/invalid Skills catalog');
  assert.equal(page.view, 'popular');
  assert.equal(page.page, 0);
  assert.ok(page.cache && ['fresh', 'stale'].includes(page.cache.status));
  report.total_items = page.items.length;
  for (const item of page.items as SkillListing[]) {
    assert.equal(typeof item.id, 'string');
    assert.equal(typeof item.title, 'string');
    assert.equal(typeof item.skillNameHint, 'string');
    assert.equal(item.providerId, 'skills-sh');
    const repo = skillRepositoryUrl(item);
    const publisher = skillPublisherUrl(item);
    const command = skillInstallCommand(item);
    report.github += Number(Boolean(repo));
    report.https_publisher += Number(Boolean(publisher));
    report.unsupported += Number(item.availability === 'unsupported-source');
    report.installable += Number(Boolean(command));
    report.legacy_publisher_recovered += Number(
      item.availability === 'unsupported-source' && Boolean(publisher),
    );
    if (command) {
      assert.ok(repo || publisher, 'Command has no validated Git/HTTPS source');
      assert.ok(!command.includes('skills.sh'), 'Directory listings are never install URLs');
    } else if (report.blocked_samples.length < 12) {
      report.blocked_samples.push({
        id: item.id,
        reason: repo ? 'invalid-name' : publisher ? 'invalid-publisher-name' : 'source-unavailable',
      });
    }
  }
  assert.ok(report.installable > 0, 'No installable Skills were returned');
  console.log(JSON.stringify(report, null, 2));
} catch (error) {
  report.failure = error instanceof Error ? error.message : String(error);
  console.error('Skills live API acceptance failed:', report.failure);
  throw error;
} finally {
  const directory = process.env.UAP_SKILLS_INSTALL_REPORT_DIR;
  if (directory) {
    mkdirSync(directory, { recursive: true });
    writeFileSync(join(directory, 'api-classification.json'), JSON.stringify(report, null, 2) + '\n');
  }
}
