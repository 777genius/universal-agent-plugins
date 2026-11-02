// Test-only HTTP adapter. Nothing in src/ or the production entry imports this module.
import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import type { Page } from '@playwright/test';
import type { components } from '../../src/shared/api/generated/schema';

export async function fixture<T>(name: string): Promise<T> {
  const path = fileURLToPath(new URL(`../../../../cli/internal/agentpluginscli/skillspicker/testdata/api-v1/${name}.json`, import.meta.url));
  return JSON.parse(await readFile(path, 'utf8')) as T;
}
export async function serveFixtures(page: Page, options: { loseApply?: boolean; stale?: boolean; count?: number; unavailableTargets?: number; planDelayMs?: number } = {}) {
  const catalog = await fixture<components['schemas']['Catalog']>('catalog');
  const session = await fixture<components['schemas']['Session']>('session');
  const targets = await fixture<components['schemas']['Targets']>('targets');
  if (options.unavailableTargets) {
    targets.targets.push(...Array.from({ length: options.unavailableTargets }, (_, index) => ({
      id: `unavailable-${index}`, label: `Unavailable client ${index}`, scope: 'user', displayPath: '', available: false,
      reason: 'This client is not detected; install it before selecting it.',
    })));
  }
  const plan = await fixture<components['schemas']['Plan']>('plan');
  const operation = await fixture<components['schemas']['Operation']>('operation');
  const preview = await fixture<components['schemas']['Preview']>('preview');
  if (options.count) {
    catalog.candidates = Array.from({ length: options.count }, (_, index) => ({ id: `skill-${index}`, name: `skill-${index}`, description: 'A useful synthetic skill', path: `category-${index % 20}/skill-${index}`, status: 'available', diagnosticCodes: [] }));
    catalog.counts = { total: options.count, available: options.count, invalid: 0, unavailable: 0 };
  }
  let applyCount = 0, scanCount = 0, planCount = 0;
  let accepted = false;
  await page.route('**/api/v1/**', async (route) => {
    const path = new URL(route.request().url()).pathname;
    const method = route.request().method();
    const json = (body: unknown, status = 200) => route.fulfill({ status, json: body });
    if (route.request().headers().authorization !== 'Bearer test-capability') return json({ code: 'UNAUTHORIZED', message: 'Missing session capability', retryable: false }, 401);
    if (path.endsWith('/session')) return json({ ...session, operationId: accepted ? operation.operationId : undefined, state: accepted ? 'done' : 'ready' });
    if (path.endsWith('/catalog') && method === 'POST') { scanCount++; catalog.catalogId = 'catalog-new'; return json({ ...session, catalogId: catalog.catalogId }, 202); }
    if (path.endsWith('/catalog')) return json(catalog);
    if (path.endsWith('/targets')) return json(targets);
    if (path.endsWith('/preview')) {
      const candidate = catalog.candidates.find((candidate) => path.includes(encodeURIComponent(candidate.id)));
      return json({ ...preview, catalogId: catalog.catalogId, candidateId: candidate?.id, path: candidate?.path,
        content: '# Read carefully\n\nUseful instructions.\n\n<script>window.injected=true</script>\n\n![tracking](https://tracking.invalid/pixel.png)\n\n[command](javascript:alert(1)) [relative](../api/v1/plans) [safe](https://example.com)' });
    }
    if (path.endsWith('/plans')) {
      planCount++;
      if (options.planDelayMs) await new Promise((resolve) => setTimeout(resolve, options.planDelayMs));
      const input = route.request().postDataJSON() as components['schemas']['PlanRequest'];
      const actions = input.selectedIds.flatMap((id) => input.targetIds.map((targetId) => ({ ...plan.actions[0], candidateId: id, targetId, alias: catalog.candidates.find((c) => c.id === id)?.name ?? id })));
      return json({ ...plan, actions, expiresAt: new Date(Date.now() + 600_000).toISOString() });
    }
    if (path.endsWith('/apply')) {
      applyCount++;
      if (options.stale) return json({ code: 'PLAN_STALE', message: 'Plan is stale. Review again.', retryable: false }, 409);
      accepted = true;
      if (options.loseApply && applyCount === 1) return route.abort('connectionreset');
      return json({ operationId: operation.operationId }, 202);
    }
    if (path.includes('/operations/')) return json(operation);
    return json({ code: 'NOT_FOUND', message: path, retryable: false }, 404);
  });
  return { catalog, targets, counts: () => ({ applyCount, planCount, scanCount }) };
}
