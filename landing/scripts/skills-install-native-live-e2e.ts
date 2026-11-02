import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import type { SkillListing } from '../types/skillsCatalog.ts';
import { skillInstallCommand } from '../utils/skillsCatalog.ts';

// Actual public HTTPS/GitHub acquisition through the built production CLI.
// Every installation uses a disposable project and home, not runner profiles.
assert.equal(process.env.UAP_E2E_LIVE_SKILLS, '1');
const binary = resolve(process.env.UAP_E2E_SKILLS_BIN || '');
assert.ok(existsSync(binary), 'Build the actual CLI before this test');
const reportRoot = process.env.UAP_SKILLS_INSTALL_REPORT_DIR || join(tmpdir(), 'uap-native-skills-report');
mkdirSync(reportRoot, { recursive: true });

const fixtures = [
  { source: 'https://github.com/anthropics/skills/tree/main/skills/frontend-design',
    skill: 'frontend-design',
    id: 'skills-sh:anthropics/skills/tree/main/skills/frontend-design:frontend-design',
    providerUrl: 'https://www.skills.sh/' },
  { source: 'https://github.com/anthropics/skills/blob/main/skills/frontend-design/SKILL.md',
    skill: 'frontend-design',
    id: 'skills-sh:https://github.com/anthropics/skills/blob/main/skills/frontend-design/SKILL.md:frontend-design',
    providerUrl: 'https://www.skills.sh/' },
  { source: 'https://raw.githubusercontent.com/anthropics/skills/main/skills/frontend-design/SKILL.md',
    skill: 'frontend-design',
    id: 'skills-sh:https://raw.githubusercontent.com/anthropics/skills/main/skills/frontend-design/SKILL.md:frontend-design',
    providerUrl: 'https://www.skills.sh/' },
  { source: 'https://uizze.sh', skill: 'ui-taste',
    id: 'skills-sh:uizze.sh:ui-taste',
    providerUrl: 'https://www.skills.sh/' },
  { source: 'https://uizze.com', skill: 'ui-radar',
    id: 'skills-sh:https://uizze.com:ui-radar',
    providerUrl: 'https://www.skills.sh/' },
  { source: 'https://open.feishu.cn', skill: 'lark-mail',
    id: 'skills-sh:https://open.feishu.cn:lark-mail',
    providerUrl: 'https://www.skills.sh/' },
] as const;
type Evidence = { id: string; source: string; skill: string; command: string; exit_code: number | null;
  installed: boolean; lock_written: boolean; native_skill_present: boolean; resources: number;
  stderr_tail: string; error: string | null };
const report = {
  checked_at: new Date().toISOString(), production_cli: binary, profile: 'isolated temporary project',
  live_network: true, results: [] as Evidence[],
};
const runRoot = mkdtempSync(join(tmpdir(), 'uap-live-skills-'));
function listing(fixture: typeof fixtures[number]): SkillListing {
  return {
    id: fixture.id, providerId: 'skills-sh', providerUrl: fixture.providerUrl,
    title: fixture.skill, skillNameHint: fixture.skill, availability: 'unsupported-source',
    source: { kind: 'external', url: fixture.providerUrl },
    metrics: { installs: 1, window: 'all-time' },
  };
}
function countSkills(root: string): number {
  if (!existsSync(root)) return 0;
  let count = 0;
  for (const entry of readdirSync(root, { withFileTypes: true })) {
    if (entry.isSymbolicLink()) continue;
    const file = join(root, entry.name);
    if (entry.isDirectory()) count += countSkills(file);
    else if (entry.isFile() && entry.name === 'SKILL.md') count++;
  }
  return count;
}
try {
  for (const [i, fixture] of fixtures.entries()) {
    const sourceDir = join(runRoot, String(i));
    const project = join(sourceDir, 'project');
    const home = join(sourceDir, 'home');
    const config = join(sourceDir, 'config');
    const data = join(sourceDir, 'data');
    const cache = join(sourceDir, 'cache');
    for (const path of [project, home, config, data, cache]) mkdirSync(path, { recursive: true });
    const command = skillInstallCommand(listing(fixture));
    assert.equal(command, 'npx universal-agent-plugins@latest skills add ' + fixture.source +
      ' --skill ' + fixture.skill, 'The UI must select exactly the tested source');
    const env = { ...process.env, HOME: home, USERPROFILE: home, XDG_CONFIG_HOME: config,
      XDG_DATA_HOME: data, XDG_CACHE_HOME: cache, GIT_CONFIG_NOSYSTEM: '1',
      GIT_CONFIG_GLOBAL: join(config, 'gitconfig'), GIT_TERMINAL_PROMPT: '0',
      GCM_INTERACTIVE: 'never', CI: 'true', DO_NOT_TRACK: '1' };
    const args = ['skills', 'add', fixture.source, '--skill', fixture.skill, '--target', 'codex',
      '--scope', 'project', '--project-root', project, '--no-ui', '--yes', '--format', 'json'];
    const output = spawnSync(binary, args, { env, cwd: project, encoding: 'utf8',
      timeout: 120_000, maxBuffer: 1024 * 1024 * 2 });
    const lockWritten = existsSync(join(project, 'uap.lock')) && existsSync(join(project, 'uap.toml'));
    const nativeSkillsRoot = join(project, '.agents', 'skills');
    // A cached publisher copy under .uap is not native installation evidence.
    const nativeSkillPresent = existsSync(join(nativeSkillsRoot, fixture.skill, 'SKILL.md'));
    const resources = countSkills(nativeSkillsRoot);
    const evidence: Evidence = {
      id: fixture.id, source: fixture.source, skill: fixture.skill, command,
      exit_code: output.status,
      installed: output.status === 0 && lockWritten && nativeSkillPresent && resources > 0,
      lock_written: lockWritten, native_skill_present: nativeSkillPresent, resources,
      stderr_tail: (output.stderr || '').slice(-1600),
      error: output.error?.message || null,
    };
    report.results.push(evidence);
    console.log(JSON.stringify(evidence));
    assert.ok(evidence.installed,
      'Native public Skill install failed for ' + fixture.id + '; inspect evidence');
    assert.ok(readFileSync(join(project, 'uap.lock'), 'utf8').includes(fixture.skill),
      'Lock must preserve the selected Skill');
  }
} finally {
  writeFileSync(join(reportRoot, 'native-install.json'), JSON.stringify(report, null, 2) + '\n');
  rmSync(runRoot, { recursive: true, force: true });
}
