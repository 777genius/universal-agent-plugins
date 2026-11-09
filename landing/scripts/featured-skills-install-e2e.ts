import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { spawnSync } from 'node:child_process';
import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { recommendedSkills } from '../data/catalogCuration.ts';
import { skillInstallCommand } from '../utils/skillsCatalog.ts';
import type { SkillListing } from '../types/skillsCatalog.ts';

// The six IDs are the exact public website recommendations. Never replace
// missing native evidence with a mock CLI, simulated client or provider URL.
const target = process.env.UAP_FEATURED_SKILLS_TARGET;
const roots: Record<string, string> = {
  codex: '.agents/skills', claude: '.claude/skills', opencode: '.opencode/skills',
};
assert.ok(target && roots[target], 'Explicit genuine Linux agent target required');
assert.equal(process.platform, 'linux', 'This supplements the Windows/macOS 30-source matrix');
assert.equal(recommendedSkills.length, 6, 'Review changes to the public featured Skills list');
assert.equal(new Set(recommendedSkills.map(({ repository, slug }) => repository + '/' + slug)).size, 6);
const outputDir = process.env.UAP_FEATURED_SKILLS_REPORT_DIR;
assert.ok(outputDir, 'Explicit artifact directory required');
mkdirSync(outputDir, { recursive: true });
const parent = mkdtempSync(join(tmpdir(), 'uap-featured-skills-'));
const evidence: Array<Record<string, unknown>> = [];
const errors: string[] = [];
const run = (args: string[], cwd: string, env: NodeJS.ProcessEnv) => {
  const result = spawnSync('agentplugins', args, {
    cwd, env, encoding: 'utf8', timeout: 150_000, maxBuffer: 2 * 1024 * 1024,
  });
  return {
    code: result.status, error: result.error?.message ?? null,
    stdout: (result.stdout || '').slice(-1200), stderr: (result.stderr || '').slice(-1200),
  };
};
const sha256 = (file: string) => createHash('sha256').update(readFileSync(file)).digest('hex');
const version = spawnSync('agentplugins', ['version'], { encoding: 'utf8', timeout: 15_000 });
const report = {
  schema_version: 1, target, github_sha: process.env.GITHUB_SHA ?? '',
  cli_version: (version.stdout || '').trim(), native_client: process.env.UAP_NATIVE_CLIENT_VERSION ?? '',
  source: 'public website recommendedSkills; live GitHub checkout; genuine Linux client',
  simulated_native_activation: false, model_calls: false, evidence,
};
try {
  assert.equal(version.status, 0, 'Published CLI version must execute');
  for (const [index, { repository, slug }] of recommendedSkills.entries()) {
    const source = 'https://github.com/' + repository;
    const project = join(parent, String(index), 'project');
    const home = join(parent, String(index), 'home');
    const config = join(parent, String(index), 'config');
    mkdirSync(project, { recursive: true });
    mkdirSync(home, { recursive: true });
    mkdirSync(config, { recursive: true });
    mkdirSync(join(home, '.codex'), { recursive: true });
    const env: NodeJS.ProcessEnv = {
      ...process.env, HOME: home, USERPROFILE: home, CODEX_HOME: join(home, '.codex'),
      XDG_CONFIG_HOME: config, XDG_DATA_HOME: join(home, 'data'), XDG_CACHE_HOME: join(home, 'cache'),
      GIT_CONFIG_NOSYSTEM: '1', GIT_CONFIG_GLOBAL: join(config, 'gitconfig'),
      GIT_TERMINAL_PROMPT: '0', GCM_INTERACTIVE: 'never', DO_NOT_TRACK: '1', CI: 'true',
    };
    for (const secret of ['OPENAI_API_KEY', 'ANTHROPIC_API_KEY', 'GITHUB_TOKEN',
      'GH_TOKEN', 'NPM_TOKEN', 'GIT_ASKPASS']) delete env[secret];
    const git = spawnSync('git', ['init', '--quiet', project], { env, encoding: 'utf8', timeout: 15_000 });
    assert.equal(git.status, 0, 'Disposable Git project must initialize');
    const listing: SkillListing = {
      id: 'skills-sh:' + repository + ':' + slug, providerId: 'skills-sh',
      providerUrl: 'https://www.skills.sh/', title: slug, skillNameHint: slug,
      source: { kind: 'github', url: source, repository },
      availability: 'unverified', metrics: { installs: 0, window: 'all-time' },
    };
    const command = skillInstallCommand(listing, [target], 'project');
    const expected = 'npx universal-agent-plugins@latest skills add ' + source +
      ' --skill ' + slug + ' --target ' + target + ' --scope project';
    assert.equal(command, expected, 'The actual website command must match this native test');
    const addArgs = ['skills', 'add', source, '--skill', slug, '--target', target,
      '--scope', 'project', '--project-root', project, '--no-ui', '--yes', '--format', 'json'];
    const ownedPath = join(project, roots[target]!, slug, 'SKILL.md');
    const lockPath = join(project, 'uap.lock');
    const sentinel = join(project, 'unmanaged.txt');
    writeFileSync(sentinel, 'do not touch this file\n');
    const row: Record<string, unknown> = { repository, slug, command, target };
    evidence.push(row);
    const installed = run(addArgs, project, env);
    row.install = installed;
    if (installed.code !== 0 || !existsSync(ownedPath) || !existsSync(lockPath)) {
      errors.push(repository + '/' + slug + ': install/native file/lock failed');
      continue;
    }
    const originalHash = sha256(ownedPath);
    const lockText = readFileSync(lockPath, 'utf8');
    if (!lockText.includes(slug)) errors.push(repository + '/' + slug + ': lock omits skill');
    row.installed_sha256 = originalHash;
    const repeated = run(addArgs, project, env);
    row.repeat = repeated;
    if (repeated.code !== 0 || !existsSync(ownedPath) || sha256(ownedPath) !== originalHash)
      errors.push(repository + '/' + slug + ': repeat mutated native Skill');
    const removed = run(['remove', slug, '--target', target, '--scope', 'project',
      '--project-root', project, '--format', 'json'], project, env);
    row.remove = removed;
    row.native_removed = !existsSync(ownedPath);
    row.sentinel_preserved = existsSync(sentinel) && readFileSync(sentinel, 'utf8') === 'do not touch this file\n';
    if (removed.code !== 0 || !row.native_removed || !row.sentinel_preserved)
      errors.push(repository + '/' + slug + ': removal/ownership failed');
  }
  assert.equal(evidence.length, 6, 'Every featured source must be attempted');
  assert.deepEqual(errors, [], 'Unqualified live installs; inspect all six original receipts');
} finally {
  writeFileSync(join(outputDir, 'featured-' + target + '.json'),
    JSON.stringify({ ...report, errors }, null, 2) + '\n');
  rmSync(parent, { recursive: true, force: true });
}
