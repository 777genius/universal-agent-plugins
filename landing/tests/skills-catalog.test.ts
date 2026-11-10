import assert from 'node:assert/strict';
import { test } from 'node:test';
import { spawnSync } from 'node:child_process';
import { clients } from '../data/clients.ts';
import { skillClients } from '../data/skillClients.ts';
import { sortSkillsForCatalog } from '../data/catalogCuration.ts';
import capabilities from '../data/clientCapabilities.generated.json' with { type: 'json' };
import type { SkillListing } from '../types/skillsCatalog.ts';
import {
  catalogModeQuery,
  restoreSkillsQuery,
  restoreSkillsFilters,
  skillsFiltersQuery,
  skillsCatalogQuery,
  skillInstallCommand,
  skillNeedsManualSelection,
  skillRepositoryUrl,
  skillPublisherUrl,
  skillDetailRoute,
  skillDetailIdAvailable,
  skillIdentityFromId,
  skillMatchesSelection,
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


test('Skills user/project choice uses the same explicit CLI scope as plugins', () => {
  assert.equal(skillInstallCommand(skill, undefined, 'user'), skillInstallCommand(skill));
  assert.equal(skillInstallCommand(skill, undefined, 'project'),
    'npx universal-agent-plugins@latest skills add https://github.com/anthropics/skills --skill frontend-design --scope project');
  assert.equal(skillInstallCommand(skill, ['codex', 'claude'], 'project'),
    'npx universal-agent-plugins@latest skills add https://github.com/anthropics/skills --skill frontend-design --target "codex,claude" --scope project');
  assert.equal(skillInstallCommand(skill, ['chatgpt'], 'project'), undefined);
  assert.equal(skillInstallCommand(skill, ['codex; echo unsafe'], 'project'), undefined);
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

test('Skill document URLs accept only the document API source grammar', () => {
  for (const id of [
    'skills-sh:anthropics/skills:frontend-design',
    'skills-sh:uizze.com:ui-radar',
    'skills-sh:Open.Feishu.Cn:lark-mail',
  ]) assert.equal(skillDetailIdAvailable(id), true, id);
  for (const id of [
    'skills-sh:owner/repo.git:frontend-design',
    'skills-sh:https://github.com/owner/repo:frontend-design',
    'skills-sh:https://publisher.com/skills:review',
    'skills-sh:owner/repo/tree/main/skills/frontend-design:frontend-design',
    'skills-sh:open.feishu.cn/lark-cli/skills/regular:lark-mail',
    'skills-sh:owner/repo:bad' + String.fromCharCode(10) + 'name',
  ]) assert.equal(skillDetailIdAvailable(id), false, id);
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
    undefined,
  );
  const supported = { ...website,
    id: 'skills-sh:uizze.com:ui-radar',
    skillNameHint: 'ui-radar',
    source: { kind: 'external' as const, url: 'https://uizze.com' },
  };
  assert.equal(skillInstallCommand(supported, ['codex']),
    'npx universal-agent-plugins@latest skills add https://uizze.com --skill ui-radar --target codex');
  // An explicit current API source may use a non-/skills scope: only ambiguous
  // legacy unsupported-source IDs must satisfy the narrower recovery rule.
  const explicitScoped: SkillListing = {
    ...supported,
    id: 'skills-sh:https://publisher.com/team/custom:ui-radar',
    providerUrl: 'https://www.skills.sh/',
    source: { kind: 'external', url: 'https://publisher.com/team/custom' },
    availability: 'unverified',
  };
  assert.equal(skillInstallCommand(explicitScoped),
    'npx universal-agent-plugins@latest skills add https://publisher.com/team/custom --skill ui-radar');
  assert.deepEqual(skillDetailRoute(website), {
    path: '/skills/',
    query: { id: website.id, q: 'lark-mail' },
  });
  for (const url of [
    'https://www.skills.sh/site/open.feishu.cn/lark-mail',
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

test('legacy Skills API website records recover only canonical publisher installs', () => {
  // The pre-#643 API returned a skills.sh listing URL as a source, marking
  // website publishers unsupported even when the provider ID was canonical.
  const listingUrl = 'https://www.skills.sh/site/open.feishu.cn/lark-mail';
  const legacy: SkillListing = {
    ...skill,
    id: 'skills-sh:open.feishu.cn:lark-mail',
    title: 'lark-mail',
    skillNameHint: 'lark-mail',
    providerUrl: listingUrl,
    source: { kind: 'external', url: listingUrl },
    availability: 'unsupported-source',
  };
  assert.equal(skillPublisherUrl(legacy), 'https://open.feishu.cn');
  assert.equal(skillInstallCommand(legacy), undefined);
  assert.equal(skillInstallCommand(legacy, ['codex', 'claude']), undefined);
  // The real Uizze well-known publisher supports the same legacy DTO shape.
  const uizze = { ...legacy,
    id: 'skills-sh:uizze.com:ui-radar', title: 'ui-radar', skillNameHint: 'ui-radar',
    providerUrl: 'https://www.skills.sh/site/uizze.com/ui-radar',
    source: { kind: 'external' as const, url: 'https://www.skills.sh/site/uizze.com/ui-radar' },
  };
  assert.equal(skillInstallCommand(uizze),
    'npx universal-agent-plugins@latest skills add https://uizze.com --skill ui-radar');

  // Do not broaden the recovery into trusting arbitrary directory or publisher
  // URLs: the canonical listing ID, origin, slug and source must all agree.
  for (const modified of [
    { ...legacy, id: 'skills-sh:other.example:lark-mail' },
    { ...legacy, providerId: 'unrelated-provider' },
    { ...legacy, providerUrl: 'https://www.skills.sh/site/open.feishu.cn/other' },
    { ...legacy, providerUrl: 'https://skills.sh/site/open.feishu.cn/lark-mail' },
    { ...legacy, source: { kind: 'external' as const, url: 'https://evil.example' } },
    { ...legacy, source: { kind: 'external' as const, url: 'https://open.feishu.cn' } },
    { ...legacy, id: 'skills-sh:https://evil.example:lark-mail' },
    { ...legacy, id: 'skills-sh:127.0.0.1:lark-mail' },
    { ...legacy, id: 'skills-sh:owner/repo/tree/main:lark-mail' },
    { ...legacy, skillNameHint: 'other-valid-name' },
  ]) {
    assert.equal(skillPublisherUrl(modified), undefined, modified.id);
    assert.equal(skillInstallCommand(modified), undefined, modified.id);
  }

  // Malformed IDNA host labels and lone-surrogate slugs cannot crash a
  // complete Vue Skills catalog while one legacy record is inspected.
  for (const invalid of [
    {
      ...legacy,
      id: 'skills-sh:xn--a.example:lark-mail',
      providerUrl: 'https://www.skills.sh/site/xn--a.example/lark-mail',
      source: { kind: 'external' as const, url: 'https://www.skills.sh/site/xn--a.example/lark-mail' },
    },
    { ...legacy, id: 'skills-sh:open.feishu.cn:\ud800' },
  ]) {
    assert.equal(skillPublisherUrl(invalid), undefined);
    assert.equal(skillInstallCommand(invalid), undefined);
  }

  // The HTTPS CLI requires exact normative names; never invent one from the
  // listing slug, display title or a guessed source folder.
  for (const name of ['Lark Mail', 'BAD$(touch injected)', 'bad--name', 'bad/name']) {
    assert.equal(skillInstallCommand({ ...legacy, skillNameHint: name }), undefined, name);
  }
});

test('Direct Skill selection never reuses stale details for a different name', () => {
  assert.equal(skillMatchesSelection(skill, skill.id, 'frontend-design'), true);
  assert.equal(skillMatchesSelection(skill, skill.id, 'different-name'), false);
  assert.equal(skillMatchesSelection(skill, 'skills-sh:other/repo:frontend-design', 'frontend-design'), false);
});

test('Detail identity uses only a canonical source, never the listing-page URL', () => {
  assert.deepEqual(skillIdentityFromId('skills-sh:open.feishu.cn:lark-mail'), {
    kind: 'external',
    url: 'https://open.feishu.cn',
  });
  assert.deepEqual(skillIdentityFromId('skills-sh:Open.Feishu.Cn:lark-mail'), {
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
    'skills-sh:https://example.com:8443:secret',
    'skills-sh:owner/repo/tree:bad',
    'skills-sh:open.feishu.cn:bad:extra',
    'not-a-skill',
    'skills-sh:bad\nsource:other',
    'skills-sh:open.feishu.cn:',
  ]) {
    assert.equal(skillIdentityFromId(id), undefined, id);
  }
});

test('Skill agents require an explicit published landing entry and generated capability', () => {
  // A newly registered SDK client must not automatically appear on the public
  // site before its installation guidance and published CLI are qualified.
  const publishedClients = new Set<string>(clients.map((client) => client.id));
  const expected = capabilities
    .filter((client) => publishedClients.has(client.id))
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
    .replace(/^import[\s\S]*?;\n/gm, '')
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
    skillInstallCommand,
    sortSkillsForCatalog,
    useRuntimeConfig: () => ({
      public: { skillsCatalogEndpoint: 'https://example.com/v1/skills' },
    }),
    useNuxtApp: () => ({}),
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
  async function respond(index: number, count = 1, page = 0, status = 200, unavailable = 0, more = page === 0) {
    requests[index]!.resolve({
      ok: status === 200,
      json: async () => ({
        items: Array.from({ length: count }, (_, i) => i < unavailable
          ? {
              ...skill, id: `skills-sh:unknown/path/subfolder:${page}-${i}`,
              source: { kind: 'external', url: 'https://www.skills.sh/' },
              providerUrl: 'https://www.skills.sh/',
              availability: 'unsupported-source',
            }
          : { ...skill, id: `${skill.id}-${page}-${i}` }),
        page,
        hasMore: more,
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


test('legacy source variants produce CLI commands without using directory pages as origins', () => {
  const unknown = (raw: string, hint = 'frontend-design'): SkillListing => ({
    ...skill,
    id: `skills-sh:${raw}:${hint}`,
    title: hint,
    skillNameHint: hint,
    providerUrl: 'https://www.skills.sh/',
    source: { kind: 'external', url: 'https://www.skills.sh/' },
    availability: 'unsupported-source',
  });
  const supported = [
    ['owner/repo/tree/main/skills/frontend-design', 'https://github.com/owner/repo/tree/main/skills/frontend-design'],
    ['https://github.com/owner/repo/blob/main/skills/frontend-design/SKILL.md', 'https://github.com/owner/repo/blob/main/skills/frontend-design/SKILL.md'],
    ['https://github.com/owner/repo.git', 'https://github.com/owner/repo'],
    ['git+https://github.com/owner/repo.git', 'https://github.com/owner/repo'],
    ['git@github.com:owner/repo.git', 'https://github.com/owner/repo'],
    ['github.com/owner/repo', 'https://github.com/owner/repo'],
    ['owner/repo.git', 'https://github.com/owner/repo'],
    ['https://raw.githubusercontent.com/owner/repo/main/skills/frontend-design/SKILL.md',
      'https://raw.githubusercontent.com/owner/repo/main/skills/frontend-design/SKILL.md'],
  ] as const;
  for (const [raw, source] of supported) {
    const listing = unknown(raw);
    assert.equal(skillRepositoryUrl(listing), source, raw);
    assert.equal(skillIdentityFromId(listing.id)?.kind, 'github', raw);
    assert.equal(
      skillInstallCommand(listing),
      `npx universal-agent-plugins@latest skills add ${source} --skill frontend-design`,
      raw,
    );
  }
  const mismatchedGithub = unknown('https://github.com/owner/repo/tree/main/skills/frontend-design');
  mismatchedGithub.skillNameHint = 'other-valid-name';
  assert.equal(skillRepositoryUrl(mismatchedGithub), undefined);
  assert.equal(skillInstallCommand(mismatchedGithub), undefined);
  const scoped = unknown('https://publisher.com/scoped/skills', 'review');
  assert.equal(skillPublisherUrl(scoped), 'https://publisher.com/scoped/skills');
  assert.equal(
    skillInstallCommand(scoped),
    'npx universal-agent-plugins@latest skills add https://publisher.com/scoped/skills --skill review',
  );
    const bareScoped = unknown('open.feishu.cn/lark-cli/skills/regular', 'lark-mail');
  assert.equal(skillPublisherUrl(bareScoped), 'https://open.feishu.cn/lark-cli/skills/regular');
  assert.equal(
    skillInstallCommand(bareScoped),
    undefined,
  );
  const canonicalBareHost = unknown('Open.Feishu.Cn/lark-cli/skills/regular', 'lark-mail');
  assert.equal(skillPublisherUrl(canonicalBareHost), 'https://open.feishu.cn/lark-cli/skills/regular');
  const publisher = unknown('https://open.feishu.cn', 'lark-mail');
  assert.equal(skillPublisherUrl(publisher), 'https://open.feishu.cn');
  assert.equal(
    skillInstallCommand(publisher),
    undefined,
  );
  assert.equal(
    skillInstallCommand(unknown('https://github.com/owner/repo', 'UPPER')),
    'npx universal-agent-plugins@latest skills add https://github.com/owner/repo',
  );
  for (const raw of [
    'owner/repo#main', 'owner/repo/tree', 'owner/repo/tree/../secret',
    'owner/repo/blob/main/not-skill.txt', 'https://gitlab.com/owner/repo', 'https://bitbucket.org/owner/repo', 'https://codeberg.org/owner/repo',
    'https://github.com/owner/repo?ref=main', 'https://github.com/owner/repo/tree/main/../../x',
    'https://github.com/owner/repo;touch-hack', 'git@evil.example:owner/repo',
    'https://evil.com/owner/repo', 'https://www.skills.sh/site/owner/repo',
    'https://localhost', 'https://127.0.0.1', 'https://xn--a.example',
    'https://publisher.internal', 'owner/repo/tree/main/$hack',
    'open.feishu.cn/lark-cli/../secret',
    'open.feishu.cn/lark-cli//regular',
    'open.feishu.cn/lark-cli/regular?token=secret',
    'https://publisher.com/scoped/../private',
    'https://publisher.com/scoped//bad',
    'https://publisher.com/scoped/?token=1',
    'https://raw.githubusercontent.com/owner/repo/main/secret.sh',
    'https://raw.githubusercontent.com/owner/repo/main/../SKILL.md',
    'https://raw.githubusercontent.com/owner/repo/main/SKILL.md?ref=main',
  ]) {
    assert.equal(skillInstallCommand(unknown(raw)), undefined, raw);
  }
});

test('catalog shows non-installable entries without inventing install commands or extra requests', async () => {
  const catalog = catalogHarness();
  await catalog.respond(0, 200, 0, 200, 175);
  assert.equal(catalog.requests.length, 1);
  assert.equal(catalog.state.items.value.length, 200);
  assert.equal(catalog.state.displayed.value.length, 48);
  assert.ok(catalog.state.displayed.value.every((item: SkillListing) => !skillInstallCommand(item)));
  for (const count of [96, 144, 192, 200]) {
    catalog.state.showMore();
    assert.equal(catalog.state.displayed.value.length, count);
    assert.equal(catalog.requests.length, 1);
  }
  catalog.state.showMore();
  assert.equal(catalog.requests.length, 2);
  await catalog.respond(1, 200, 1, 200, 0, false);
  assert.equal(catalog.state.items.value.length, 400);
  assert.equal(catalog.state.displayed.value.length, 248);
  assert.ok(catalog.state.items.value.some((item: SkillListing) => skillInstallCommand(item)));
  catalog.stop();
});

test('empty upstream page stops rather than looping on unsupported or missing data', async () => {
  const catalog = catalogHarness();
  await catalog.respond(0, 0, 0, 200, 0, true);
  assert.equal(catalog.requests.length, 1);
  assert.equal(catalog.state.displayed.value.length, 0);
  assert.equal(catalog.state.hasMore.value, false);
  catalog.state.showMore();
  assert.equal(catalog.requests.length, 1);
  catalog.stop();
});

test('Skill search honors API page-zero-only contract even if response advertises more', async () => {
  const catalog = catalogHarness();
  catalog.query.value = 'frontend';
  await nextTick();
  catalog.debounce();
  assert.equal(catalog.requests[1]!.url.searchParams.get('page'), '0');
  assert.equal(catalog.requests[1]!.url.searchParams.get('q'), 'frontend');
  await catalog.respond(1, 200, 0, 200, 200, true);
  assert.equal(catalog.requests.length, 2);
  assert.equal(catalog.state.displayed.value.length, 48);
  assert.ok(catalog.state.displayed.value.every((item: SkillListing) => !skillInstallCommand(item)));
  assert.equal(catalog.state.hasMore.value, true); // Remaining page-zero candidates, not a second search page.
  catalog.state.showMore();
  assert.equal(catalog.requests.length, 2);
  assert.equal(catalog.state.displayed.value.length, 96);
  catalog.stop();
});

test('legacy GitHub DTO disagreement never interpolates unsafe Skill selectors', () => {
  const legacy = {
    ...skill,
    id: 'skills-sh:https://github.com/anthropics/skills:some-skill',
    skillNameHint: 'BAD$(touch injected)',
    providerUrl: 'https://www.skills.sh/',
    source: { kind: 'external' as const, url: 'https://www.skills.sh/' },
    availability: 'unsupported-source' as const,
  };
  // A mismatched source-ID selector is not a valid installation candidate.
  assert.equal(skillNeedsManualSelection(legacy), false);
  assert.equal(skillInstallCommand(legacy), undefined);
});

test('malformed DTOs disable their own Install action without crashing the catalog', () => {
  assert.equal(skillIdentityFromId(undefined as unknown as string), undefined);
  assert.equal(skillInstallCommand({ ...skill, id: undefined as unknown as string }), undefined);
  assert.equal(skillInstallCommand({ ...skill, source: null as unknown as SkillListing['source'] }), undefined);
  assert.equal(skillInstallCommand(null as unknown as SkillListing), undefined);
});

test('Skill filters default unchecked and serialize each independent checkbox without losing other URL state', () => {
  const none = { scannedOnly: false, noRemarksOnly: false };
  for (const params of [
    {},
    { skillScan: 'all' },
    { skillScan: 'unknown', skillRemarks: 'some' },
    { skillScan: ['scanned'], skillRemarks: ['none'] },
  ])
    assert.deepEqual(restoreSkillsFilters(params), none);
  const existing = {
    tab: 'skills',
    q: 'frontend',
    skillView: 'hot',
    source: 'keep',
    skillScan: 'all',
  };
  for (const [filters, expected] of [
    [none, {}],
    [{ scannedOnly: true, noRemarksOnly: false }, { skillScan: 'scanned' }],
    [{ scannedOnly: false, noRemarksOnly: true }, { skillRemarks: 'none' }],
    [
      { scannedOnly: true, noRemarksOnly: true },
      { skillScan: 'scanned', skillRemarks: 'none' },
    ],
  ] as const) {
    const route = skillsFiltersQuery(existing, filters);
    assert.deepEqual(route, {
      tab: 'skills',
      q: 'frontend',
      skillView: 'hot',
      source: 'keep',
      ...expected,
    });
    assert.deepEqual(restoreSkillsFilters(route), filters);
  }
});
