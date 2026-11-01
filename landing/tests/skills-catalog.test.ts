import assert from 'node:assert/strict';
import { test } from 'node:test';
import { spawnSync } from 'node:child_process';
import { skillClients } from '../data/skillClients.ts';
import capabilities from '../data/clientCapabilities.generated.json' with { type: 'json' };
import type { SkillListing } from '../types/skillsCatalog.ts';
import {
  catalogModeQuery,
  restoreSkillsQuery,
  skillsCatalogQuery,
  skillInstallCommand,
  skillNeedsManualSelection,
  skillRepositoryUrl,
  skillPublisherUrl,
  skillDetailRoute,
  skillIdentityFromId,
} from '../utils/skillsCatalog.ts';
import { catalogQuery, restoreCatalogQuery } from '../utils/filter.ts';

const skill: SkillListing = {
  id: 'skills-sh:anthropics/skills:frontend-design',
  providerId: 'skills-sh',
  providerUrl: 'https://skills.sh/anthropics/skills/frontend-design',
  title: 'frontend-design',
  skillNameHint: 'frontend-design',
  source: {
    kind: 'github',
    repository: 'anthropics/skills',
    url: 'https://github.com/anthropics/skills',
  },
  metrics: { installs: 123, window: 'all-time' },
  availability: 'unverified',
};

test('Skill command preserves repository identity and exact selector without inventing a path', () => {
  assert.equal(
    skillInstallCommand(skill),
    'npx universal-agent-plugins@latest skills add https://github.com/anthropics/skills --skill frontend-design',
  );
  assert.equal(
    skillInstallCommand(skill, ['codex', 'claude', 'codex']),
    'npx universal-agent-plugins@latest skills add https://github.com/anthropics/skills --skill frontend-design --target "codex,claude"',
  );
  assert.equal(
    skillRepositoryUrl({
      ...skill,
      source: {
        ...skill.source,
        url: 'https://github.com/anthropics/skills/tree/main/guessed-folder',
      },
    }),
    'https://github.com/anthropics/skills',
  );
  assert.equal(skillInstallCommand(skill, []), undefined);
  for (const target of ['unknown-agent', 'codex; echo injected', 'codex"', 'codex$HOME']) {
    assert.equal(skillInstallCommand(skill, [target]), undefined);
  }
});

test(
  'portable quoting preserves Unicode names and target CSV in POSIX argv',
  { skip: process.platform === 'win32' },
  () => {
    const command = skillInstallCommand({ ...skill, skillNameHint: 'révision-2' }, [
      'codex',
      'claude',
    ])!;
    assert.ok(command.includes('--skill "révision-2" --target "codex,claude"'));
    // Set arguments without launching npx or an agent.
    const result = spawnSync('/bin/sh', ['-c', `set -- ${command}; printf '%s\\n' "$@"`], {
      encoding: 'utf8',
    });
    assert.equal(result.status, 0);
    assert.deepEqual(result.stdout.trim().split('\n'), [
      'npx',
      'universal-agent-plugins@latest',
      'skills',
      'add',
      'https://github.com/anthropics/skills',
      '--skill',
      'révision-2',
      '--target',
      'codex,claude',
    ]);
  },
);

test('Skill metadata cannot inject shell syntax or external install sources', () => {
  for (const hint of [
    'x’; Write-Output PWN; #',
    "designer's",
    '$(echo injected)',
    'UPPER',
    '-name',
    'name--other',
    'a'.repeat(65),
    'bad\nname',
  ]) {
    const untrusted = { ...skill, skillNameHint: hint };
    assert.equal(skillNeedsManualSelection(untrusted), true, hint);
    assert.equal(
      skillInstallCommand(untrusted, ['codex']),
      'npx universal-agent-plugins@latest skills add https://github.com/anthropics/skills --target codex',
      hint,
    );
  }
  assert.ok(
    skillInstallCommand({ ...skill, skillNameHint: 'révision-2' }, ['codex'])?.includes(
      '--skill "révision-2"',
    ),
  );
  assert.equal(
    skillInstallCommand({
      ...skill,
      source: { kind: 'external', url: 'https://example.com' },
      availability: 'unsupported-source',
    }),
    undefined,
  );
  assert.equal(
    skillInstallCommand({
      ...skill,
      source: { ...skill.source, repository: 'owner/repo/tree/main' },
    }),
    undefined,
  );
});

test('HTTPS publishers use their origin and exact names, never a listing page or guessed selector', () => {
  const website: SkillListing = {
    ...skill,
    id: 'skills-sh:open.feishu.cn:lark-mail',
    providerUrl: 'https://www.skills.sh/site/open.feishu.cn/lark-mail',
    title: 'lark-mail',
    skillNameHint: 'lark-mail',
    source: { kind: 'external', url: 'https://open.feishu.cn' },
  };
  assert.equal(skillPublisherUrl(website), 'https://open.feishu.cn');
  assert.equal(
    skillInstallCommand(website, ['codex']),
    'npx universal-agent-plugins@latest skills add https://open.feishu.cn --skill lark-mail --target codex',
  );
  assert.deepEqual(skillDetailRoute(website), {
    path: '/skills/',
    query: { id: website.id, q: 'lark-mail' },
  });
  for (const url of [
    'https://www.skills.sh/site/open.feishu.cn/lark-mail',
    'https://open.feishu.cn/lark-cli/skills/regular',
    'https://open.feishu.cn:443',
    'https://open.feishu.cn?next=evil',
    'https://name:pass@open.feishu.cn',
    'https://127.0.0.1',
  ]) {
    assert.equal(skillInstallCommand({ ...website, source: { kind: 'external', url } }), undefined, url);
  }
  assert.equal(skillInstallCommand({ ...website, skillNameHint: 'BAD$(echo injected)' }), undefined);
  assert.equal(skillInstallCommand({ ...website, availability: 'unsupported-source' }), undefined);
});

test('Detail identity uses only a canonical source, never the listing-page URL', () => {
  assert.deepEqual(skillIdentityFromId('skills-sh:open.feishu.cn:lark-mail'), {
    kind: 'external',
    url: 'https://open.feishu.cn',
  });
  assert.deepEqual(skillIdentityFromId('skills-sh:anthropics/skills:frontend-design'), {
    kind: 'github',
    repository: 'anthropics/skills',
    url: 'https://github.com/anthropics/skills',
  });
  for (const id of [
    'skills-sh:127.0.0.1:lark-mail',
    'skills-sh:https://example.com:secret',
    'skills-sh:owner/repo/tree/main:bad',
    'skills-sh:open.feishu.cn:bad:extra',
    'not-a-skill',
    'skills-sh:bad\nsource:other',
    'skills-sh:open.feishu.cn:',
  ]) {
    assert.equal(skillIdentityFromId(id), undefined, id);
  }
});

test('Skill agents come only from generated saved-dependency and skill capabilities', () => {
  const expected = capabilities
    .filter((client) => client.saved_dependency && client.skill_support !== 'unsupported')
    .map((client) => client.id);
  assert.deepEqual(
    skillClients.map((client) => client.id),
    expected,
  );
  assert.ok(!skillClients.some((client) => ['chatgpt', 'vscode'].includes(client.id)));
});

test('catalog mode and Skill state preserve plugin filters, query and unrelated route keys', () => {
  const initial = {
    q: 'frontend',
    category: 'tools',
    source: 'community',
    skillView: 'hot',
    ref: 'launch',
  };
  const skills = catalogModeQuery(initial, true);
  assert.equal(skills.tab, 'skills');
  assert.deepEqual(restoreSkillsQuery(skills), { query: 'frontend', view: 'hot' });
  const searched = skillsCatalogQuery('design', 'trending', skills);
  assert.equal(searched.category, 'tools');
  assert.equal(searched.source, 'community');
  assert.equal(searched.ref, 'launch');
  const plugins = catalogModeQuery(searched, false);
  assert.equal(plugins.tab, undefined);
  assert.equal(restoreCatalogQuery(plugins)[0], 'design');
  const changed = catalogQuery(
    ['new query', 'all', 'all', 'all', 'all', 'all', 'all', 'all'],
    searched,
  );
  assert.equal(changed.tab, 'skills');
  assert.equal(changed.skillView, 'trending');
  assert.deepEqual(restoreSkillsQuery({ q: ['back-forward', 'ignored'], skillView: 'unknown' }), {
    query: 'back-forward',
    view: 'popular',
  });
});

// Exercise the real request coordinator with controlled responses and debounce timers.
import { readFileSync } from 'node:fs';
import { stripTypeScriptTypes } from 'node:module';
import { runInNewContext } from 'node:vm';
import { computed, effectScope, nextTick, onScopeDispose, ref, shallowRef, watch } from 'vue';

function catalogHarness() {
  const requests: Array<{ url: URL; signal: AbortSignal; resolve: (value: unknown) => void }> = [];
  const timers = new Map<number, () => void>();
  let mounted: () => void = () => {};
  let timerId = 0;
  const source = readFileSync(
    new URL('../composables/useSkillsCatalog.ts', import.meta.url),
    'utf8',
  )
    .replace(/^import[^\n]+\n/gm, '')
    .replace('export function', 'function');
  const useCatalog = runInNewContext(stripTypeScriptTypes(source) + '\nuseSkillsCatalog;', {
    computed,
    ref,
    shallowRef,
    watch,
    onScopeDispose,
    AbortController,
    URLSearchParams,
    onMounted: (callback: () => void) => {
      mounted = callback;
    },
    useRuntimeConfig: () => ({
      public: { skillsCatalogEndpoint: 'https://example.com/v1/skills' },
    }),
    setTimeout: (callback: () => void, delay: number) => {
      assert.equal(delay, 300);
      timers.set(++timerId, callback);
      return timerId;
    },
    clearTimeout: (id: number) => timers.delete(id),
    fetch: (url: string, init: { signal: AbortSignal }) =>
      new Promise((resolve) => requests.push({ url: new URL(url), signal: init.signal, resolve })),
  });
  const query = ref('');
  const view = ref('popular');
  const scope = effectScope();
  const state = scope.run(() => useCatalog(query, view));
  mounted();
  const flush = async () => {
    await new Promise<void>((resolve) => setImmediate(resolve));
    await nextTick();
  };
  async function respond(index: number, count = 1, page = 0, status = 200) {
    requests[index]!.resolve({
      ok: status === 200,
      json: async () => ({
        items: Array.from({ length: count }, (_, i) => ({
          ...skill,
          id: `${skill.id}-${page}-${i}`,
        })),
        page,
        hasMore: page === 0,
        view: 'popular',
        cache: { status: 'stale', fetchedAt: '2026-10-24T10:00:00Z' },
      }),
    });
    await flush();
  }
  function debounce() {
    const callbacks = [...timers.values()];
    timers.clear();
    callbacks.forEach((callback) => callback());
  }
  return { state, query, view, requests, respond, debounce, flush, stop: () => scope.stop() };
}

test('Skill requests debounce, cancel and ignore late results; short searches make no request', async () => {
  const catalog = catalogHarness();
  catalog.query.value = 'old';
  await nextTick();
  assert.equal(catalog.requests[0]!.signal.aborted, true);
  catalog.debounce();
  catalog.query.value = 'new';
  await nextTick();
  catalog.debounce();
  assert.equal(catalog.requests.length, 3);
  await catalog.respond(2, 2);
  assert.equal(catalog.state.items.value.length, 2);
  await catalog.respond(1, 9);
  await catalog.respond(0, 10);
  assert.equal(catalog.state.items.value.length, 2);
  catalog.query.value = 'x';
  await nextTick();
  catalog.debounce();
  assert.equal(catalog.state.searchTooShort.value, true);
  assert.equal(catalog.state.loading.value, false);
  assert.equal(catalog.requests.length, 3);
  catalog.stop();
});

test('Skill pagination shows only 48 new items at a partial page boundary and retains results on failure', async () => {
  const catalog = catalogHarness();
  await catalog.respond(0, 200);
  assert.equal(catalog.state.metadata.value.cache.status, 'stale');
  assert.equal(catalog.state.displayed.value.length, 48);
  for (const count of [96, 144, 192, 200]) {
    catalog.state.showMore();
    assert.equal(catalog.state.displayed.value.length, count);
  }
  assert.equal(catalog.requests.length, 1);
  catalog.state.showMore();
  assert.equal(catalog.requests[1]!.url.searchParams.get('page'), '1');
  await catalog.respond(1, 0, 1, 503);
  assert.equal(catalog.state.items.value.length, 200);
  assert.equal(catalog.state.failed.value, true);
  const retry = catalog.state.retry();
  await catalog.respond(2, 200, 1);
  await retry;
  assert.equal(catalog.state.displayed.value.length, 248);
  assert.equal(catalog.state.failed.value, false);
  catalog.stop();
});
