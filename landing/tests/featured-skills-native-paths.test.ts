import assert from 'node:assert/strict';
import { existsSync, mkdirSync, mkdtempSync, renameSync, rmSync, symlinkSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import test, { type TestContext } from 'node:test';
import { installedDirectory, nativeSkillFile } from '../scripts/featured-skills-native-paths.ts';

const slug = 'source-skill';
const markdown = '---\nname: source-skill\ndescription: Inert source Skill\n---\nFixture only.\n';

function fixture(t: TestContext, target = 'claude') {
  const parent = mkdtempSync(join(tmpdir(), 'uap-featured-path-test-'));
  t.after(() => rmSync(parent, { recursive: true, force: true }));
  const project = join(parent, 'project');
  const roots: Record<string, string> = { claude: '.claude/skills', codex: '.agents/skills', opencode: '.opencode/skills' };
  const relativeRoot = roots[target]!;
  const root = join(project, relativeRoot);
  const reported = target === 'claude' ? join(root, 'skill-physical-id') :
    join(project, '.uap', 'managed', 'clients', target, 'skill-physical-id');
  const directory = target === 'claude' ?
    join(reported, '.agentplugins-plugin', '.agentplugins-runtime', 'skills', slug) : join(root, slug);
  mkdirSync(reported, { recursive: true });
  mkdirSync(directory, { recursive: true });
  const file = join(directory, 'SKILL.md');
  writeFileSync(file, markdown);
  return { parent, project, root, relativeRoot, reported, directory, file, target };
}

function dto(destination: string, status = 'succeeded', alias = slug, target = 'claude') {
  return JSON.stringify({ schema_version: 1, command: 'skills add', result: 'success', status: 'succeeded',
    scope: 'project', readiness: 'ready', items: [{ alias, targetId: target, status, destinationPath: destination }] });
}

test('released Claude Project bundle resolves exact physical receipt and source Skill', (t) => {
  const f = fixture(t);
  const reported = installedDirectory(dto(f.reported), slug, 'claude');
  assert.equal(reported, f.reported);
  assert.equal(nativeSkillFile(f.project, f.relativeRoot, reported, f.target, slug), f.file);
  // This is the precise old assumption: User layout directly under physical ID.
  assert.equal(existsSync(join(f.reported, '.agentplugins-runtime', 'skills', slug, 'SKILL.md')), false);
  assert.equal(installedDirectory(dto(f.reported, 'unchanged'), slug, 'claude', true), f.reported);
});

test('Project path does not accept old User layout as an alternate fallback', (t) => {
  const f = fixture(t);
  rmSync(join(f.reported, '.agentplugins-plugin'), { recursive: true });
  const oldDirectory = join(f.reported, '.agentplugins-runtime', 'skills', slug);
  mkdirSync(oldDirectory, { recursive: true });
  writeFileSync(join(oldDirectory, 'SKILL.md'), markdown);
  assert.equal(nativeSkillFile(f.project, f.relativeRoot, f.reported, f.target, slug), null);
});

test('ready JSON alone cannot provide missing native files', (t) => {
  const f = fixture(t);
  rmSync(f.file);
  const reported = installedDirectory(dto(f.reported), slug, 'claude');
  assert.equal(reported, f.reported);
  assert.equal(nativeSkillFile(f.project, f.relativeRoot, reported, f.target, slug), null);
});

test('public receipt requires exact alias, target, scope and fresh/repeat status', async (t) => {
  const f = fixture(t);
  for (const [name, body, repeated] of [
    ['wrong alias', dto(f.reported, 'succeeded', 'foreign'), false],
    ['wrong target', dto(f.reported, 'succeeded', slug, 'codex'), false],
    ['internal batch synonym', dto(f.reported, 'completed'), false],
    ['unchanged fresh', dto(f.reported, 'unchanged'), false],
    ['succeeded repeat', dto(f.reported), true],
    ['wrong scope', dto(f.reported).replace('"project"', '"user"'), false],
    ['relative destination', dto('relative/skill'), false],
    ['noncanonical destination', dto(f.reported + '/.'), false],
    ['null item', '{"command":"skills add","result":"success","status":"succeeded","scope":"project","items":[null]}', false],
    ['malformed', '{', false],
  ] as const) {
    await t.test(name, () => assert.equal(installedDirectory(body, slug, 'claude', repeated), null));
  }
});

test('receipt must remain an immediate child of the selected project native root', async (t) => {
  const f = fixture(t);
  const otherProject = join(f.parent, 'project-B');
  mkdirSync(otherProject);
  await t.test('project B cannot borrow A receipt', () => {
    assert.equal(nativeSkillFile(otherProject, f.relativeRoot, f.reported, f.target, slug), null);
  });
  await t.test('nested physical receipt refused', () => {
    assert.equal(nativeSkillFile(f.project, f.relativeRoot, join(f.reported, 'nested'), f.target, slug), null);
  });
  for (const name of ['../escape', '..', '.', 'nested/skill', 'nested\\skill', 'nul\0skill']) {
    await t.test(name.replaceAll('\0', 'NUL'), () => {
      assert.equal(nativeSkillFile(f.project, f.relativeRoot, f.reported, f.target, name), null);
    });
  }
});

test('every Project ancestor and selected Skill leaf must be ordinary', async (t) => {
  for (const component of ['project', '.claude', 'skills', 'physical', 'bundle', 'runtime', 'skill-root', 'skill', 'SKILL.md']) {
    await t.test(component, (st) => {
      const f = fixture(st);
      const selected: Record<string, string> = {
        project: f.project, '.claude': join(f.project, '.claude'), skills: f.root,
        physical: f.reported, bundle: join(f.reported, '.agentplugins-plugin'),
        runtime: join(f.reported, '.agentplugins-plugin', '.agentplugins-runtime'),
        'skill-root': dirname(f.directory), skill: f.directory, 'SKILL.md': f.file,
      };
      const original = selected[component]!;
      const foreign = join(f.parent, 'foreign-' + component);
      renameSync(original, foreign);
      symlinkSync(foreign, original, component === 'SKILL.md' ? 'file' : 'dir');
      assert.equal(nativeSkillFile(f.project, f.relativeRoot, f.reported, f.target, slug), null);
    });
  }
});

test('existing Codex and OpenCode per-Skill projections remain exact', async (t) => {
  for (const target of ['codex', 'opencode']) {
    await t.test(target, (st) => {
      const f = fixture(st, target);
      assert.equal(nativeSkillFile(f.project, f.relativeRoot, f.reported, target, slug), f.file);
      const foreign = join(f.parent, 'foreign-uap');
      renameSync(join(f.project, '.uap'), foreign);
      symlinkSync(foreign, join(f.project, '.uap'), 'dir');
      assert.equal(nativeSkillFile(f.project, f.relativeRoot, f.reported, target, slug), null);
    });
  }
});

test('existing native metadata boundary remains required', async (t) => {
  for (const body of ['no frontmatter', '---\nname: source-skill\n---\n', '---\ndescription: Missing name\n---\n']) {
    await t.test(body, (st) => {
      const f = fixture(st);
      writeFileSync(f.file, body);
      assert.equal(nativeSkillFile(f.project, f.relativeRoot, f.reported, f.target, slug), null);
    });
  }
});
