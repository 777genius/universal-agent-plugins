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
const published = process.env.UAP_E2E_PUBLISHED_CLI === '1';
const binaryPath = process.env.UAP_E2E_SKILLS_BIN;
if (!published) assert.ok(binaryPath, 'UAP_E2E_SKILLS_BIN must point to the built CLI');
const binary = resolve(binaryPath || '');
if (!published) assert.ok(existsSync(binary), 'Build the actual CLI before this test');
const reportRoot = process.env.UAP_SKILLS_INSTALL_REPORT_DIR || join(tmpdir(), 'uap-native-skills-report');
mkdirSync(reportRoot, { recursive: true });

// Exact base strings observed in the public Install dialogs on 2026-11-22 UTC.
// This assertion ties modeled commands to that historical capture; CI does not
// recapture the deployed site. Target/scope/JSON arguments are test additions.
const capturedBaseCommands = {
  'skills-sh:anthropics/skills:frontend-design': 'npx -y universal-agent-plugins@latest skills add https://github.com/anthropics/skills --skill frontend-design',
  'skills-sh:uizze.com:ui-radar': 'npx -y universal-agent-plugins@latest skills add https://uizze.com --skill ui-radar',
} as const;
const fixtures = [
  { source: 'https://github.com/anthropics/skills',
    skill: 'frontend-design', id: 'skills-sh:anthropics/skills:frontend-design',
    providerUrl: 'https://www.skills.sh/anthropics/skills/frontend-design' },
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
    providerUrl: 'https://www.skills.sh/site/uizze.sh/ui-taste' },
  { source: 'https://uizze.com', skill: 'ui-radar',
    id: 'skills-sh:uizze.com:ui-radar',
    providerUrl: 'https://www.skills.sh/site/uizze.com/ui-radar' },
  { source: 'https://open.feishu.cn', skill: 'lark-mail',
    id: 'skills-sh:open.feishu.cn:lark-mail',
    providerUrl: 'https://www.skills.sh/site/open.feishu.cn/lark-mail' },
  { source: 'https://open.feishu.cn/lark-cli/skills/regular', skill: 'lark-mail',
    id: 'skills-sh:open.feishu.cn/lark-cli/skills/regular:lark-mail',
    providerUrl: 'https://www.skills.sh/' },
] as const;
type Evidence = { id: string; source: string; skill: string; command: string; exit_code: number | null;
  installed: boolean; lock_written: boolean; native_skill_present: boolean; resources: number;
  stdout_tail: string; stderr_tail: string; error: string | null;
  lifecycle?: Record<string, unknown> };
const report = {
  checked_at: new Date().toISOString(), production_cli: published ? 'npx -y universal-agent-plugins@latest' : binary,
  profile: 'isolated temporary project',
  command_origin: 'repository DTO model and generator; published base strings compared to historical public browser capture, not recaptured by CI',
  historical_capture: { observed_at: '2026-11-22 UTC', commands: capturedBaseCommands },
  version_attribution: 'Separate version observations do not prove which binary each @latest operation executed',
  live_network: true, results: [] as Evidence[],
};
const runRoot = mkdtempSync(join(tmpdir(), 'uap-live-skills-'));
const failures: string[] = [];
function listing(fixture: typeof fixtures[number]): SkillListing {
  // Model the API's actual website DTOs rather than inventing unsupported
  // source metadata: one current origin form and two legacy listing-page forms.
  const currentPublisher = fixture.source === 'https://uizze.sh';
  const rootGitHub = fixture.source === 'https://github.com/anthropics/skills';
  return {
    id: fixture.id, providerId: 'skills-sh', providerUrl: fixture.providerUrl,
    title: fixture.skill, skillNameHint: fixture.skill,
    availability: rootGitHub || currentPublisher ? 'unverified' : 'unsupported-source',
    source: rootGitHub
      ? { kind: 'github', repository: 'anthropics/skills', url: fixture.source }
      : { kind: 'external', url: currentPublisher ? fixture.source : fixture.providerUrl },
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
function commandEvidence(output: ReturnType<typeof spawnSync>) {
  return { exit_code: output.status, error: output.error?.message || null,
    stdout: String(output.stdout || ''), stderr: String(output.stderr || '') };
}
type CLIOutput = {
  schema_version: number; command: string; result: string; status: string; readiness: string;
  items: { status: string; targetId: string; alias: string }[];
  data: {
    version: string; name: string; source: string; installation_id: string;
    installations: { installation_id: string }[];
    clients: {
      client_id: string;
      package_revision?: { tree_digest: string; manifest_digest: string; resolved_revision?: string };
      native_discovery_reconciled?: boolean; native_discovery_evidence?: unknown;
    }[];
  };
};
function completeJSON(output: ReturnType<typeof spawnSync>, command: string): CLIOutput {
  assert.ifError(output.error);
  assert.equal(output.status, 0, command + ' failed');
  const value = JSON.parse(String(output.stdout));
  assert.equal(value.schema_version, 1);
  assert.equal(value.command, command);
  assert.equal(value.result, 'success');
  return value;
}
const selected = published
  ? fixtures.filter(fixture => [
      'https://github.com/anthropics/skills',
      'https://uizze.com',
    ].includes(fixture.source))
  : fixtures;
try {
  for (const [i, fixture] of selected.entries()) {
    const sourceDir = join(runRoot, String(i));
    const project = join(sourceDir, 'project');
    const home = join(sourceDir, 'home');
    const config = join(sourceDir, 'config');
    const data = join(sourceDir, 'data');
    const cache = join(sourceDir, 'cache');
    const temp = join(sourceDir, 'tmp');
    for (const path of [project, home, config, data, cache, temp]) mkdirSync(path, { recursive: true });
    // A genuine installed Codex CLI must discover its own isolated profile.
    // Never reuse the runner's real user configuration or credentials.
    mkdirSync(join(home, '.codex'), { recursive: true });
    const command = skillInstallCommand(listing(fixture)) ?? '';
    const expectedCommand = 'npx -y universal-agent-plugins@latest skills add ' + fixture.source +
      ' --skill ' + fixture.skill;
    if (published) {
      assert.equal(command, capturedBaseCommands[fixture.id as keyof typeof capturedBaseCommands],
        'Modeled base command differs from historical public browser capture');
    }
    const knownUnavailable = fixture.skill === 'lark-mail' &&
      fixture.source.startsWith('https://open.feishu.cn');
    if (knownUnavailable ? Boolean(command) : command !== expectedCommand) {
      failures.push('Catalog command/source eligibility mismatch for ' + fixture.id);
    }
    const env = { ...process.env, HOME: home, USERPROFILE: home, CODEX_HOME: join(home, '.codex'),
      XDG_CONFIG_HOME: config, XDG_DATA_HOME: data, XDG_CACHE_HOME: cache, GIT_CONFIG_NOSYSTEM: '1',
      GIT_CONFIG_GLOBAL: join(config, 'gitconfig'), GIT_TERMINAL_PROMPT: '0',
      TMPDIR: temp, TMP: temp, TEMP: temp, npm_config_cache: join(cache, 'npm'),
      GCM_INTERACTIVE: 'never', CI: 'true', DO_NOT_TRACK: '1' };
    const foreign = join(project, '.agents', 'skills', 'foreign-sentinel', 'SKILL.md');
    mkdirSync(join(project, '.agents', 'skills', 'foreign-sentinel'), { recursive: true });
    writeFileSync(foreign, 'foreign Skill sentinel\n');
    const projectSentinel = join(project, 'foreign.txt');
    writeFileSync(projectSentinel, 'foreign project sentinel\n');
    const invoke = (args: string[]) => spawnSync(published ? 'npx' : binary,
      published ? ['-y', 'universal-agent-plugins@latest', ...args] : args,
      { env, cwd: project, encoding: 'utf8', timeout: 120_000, maxBuffer: 1024 * 1024 * 2 });
    const versionBefore = invoke(['version', '--format', 'json']);
    const args = ['skills', 'add', fixture.source, '--skill', fixture.skill, '--target', 'codex',
      '--scope', 'project', '--project-root', project, '--no-ui', '--yes', '--format', 'json'];
    const output = spawnSync(published ? 'npx' : binary,
      published ? ['-y', 'universal-agent-plugins@latest', ...args] : args,
      { env, cwd: project, encoding: 'utf8', timeout: 120_000, maxBuffer: 1024 * 1024 * 2 });
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
      stdout_tail: (output.stdout || '').slice(-1600),
      stderr_tail: (output.stderr || '').slice(-1600),
      error: output.error?.message || null,
    };
    evidence.lifecycle = { version_before: commandEvidence(versionBefore), add: commandEvidence(output), removed: false };
    try {
      const observed = completeJSON(versionBefore, 'version').data.version;
      assert.ok(typeof observed === 'string' && observed.length > 0);
      evidence.lifecycle.observed_version_before = observed;
    } catch (error) {
      failures.push('CLI version observation failed before ' + fixture.id + ': ' + String(error));
    }
    report.results.push(evidence);
    console.log(JSON.stringify(evidence));
    if (knownUnavailable) {
      // These two are REAL negative network cases, not skipped fixtures. The
      // target is a genuine installed Codex CLI; a target-detection failure
      // cannot masquerade as successful proof that the source is unusable.
      const diagnostic = evidence.stdout_tail + '\n' + evidence.stderr_tail;
      if (evidence.exit_code === 0 || evidence.lock_written || evidence.native_skill_present ||
          evidence.error || /TARGET_UNAVAILABLE|target codex is not detected/i.test(diagnostic)) {
        failures.push('Feishu source refusal unproven or unexpectedly installable: ' + fixture.id);
      }
    } else if (!evidence.installed) {
      failures.push('Native public Skill install failed for ' + fixture.id);
    }
    if (!knownUnavailable && output.status === 0) {
      try {
        const added = completeJSON(output, 'skills add');
        assert.equal(added.status, 'succeeded');
        evidence.lifecycle.readiness = added.readiness;
        evidence.lifecycle.native_qualified = false;
        assert.equal(added.items?.length, 1);
        assert.equal(added.items[0].status, 'succeeded');
        assert.equal(added.items[0].targetId, 'codex');
        const lock = JSON.parse(readFileSync(join(project, 'uap.lock'), 'utf8'));
        assert.equal(lock.lock_version, 1);
        assert.equal(lock.packages?.length, 1);
        const pin = lock.packages[0];
        assert.equal(pin.alias, fixture.skill);
        assert.equal(added.items[0].alias, pin.alias);
        assert.deepEqual(pin.targets, ['codex']);
        let expectedSource: string;
        let expectedRevision: string | undefined;
        const sha256 = /^sha256:[a-f0-9]{64}$/;
        if (fixture.source.includes('github.com') || fixture.source.includes('githubusercontent.com')) {
          const git = pin.source?.git;
          assert.equal(git?.repository, 'anthropics/skills');
          assert.equal(git.kind, 'skill');
          assert.equal(git.skill, fixture.skill);
          assert.equal(git.path, 'skills/frontend-design');
          assert.equal(git.resolved_skill_path, 'skills/frontend-design');
          assert.match(git.commit, /^[a-f0-9]{40}$/);
          assert.match(git.raw_tree_digest, sha256);
          assert.equal(git.normalizer, 'agent-skill-package/v1');
          assert.equal(pin.source_key, 'github-skill:anthropics/skills:' + fixture.skill + ':skills/frontend-design');
          expectedSource = 'anthropics/skills/skills/frontend-design';
          expectedRevision = git.commit;
        } else {
          const web = pin.source?.web;
          assert.equal(web?.url, fixture.source);
          assert.equal(web.skill, fixture.skill);
          assert.equal(pin.source_key, 'https-skill:' + fixture.source + ':' + fixture.skill);
          assert.match(web.raw_tree_digest, sha256);
          assert.match(web.index_sha256, /^[a-f0-9]{64}$/);
          assert.equal(new URL(web.index_url).origin, new URL(fixture.source).origin);
          assert.equal(web.normalizer, 'agent-skill-package/v1');
          assert.ok(Array.isArray(web.files) && web.files.some((file: { path: string }) => file.path === 'SKILL.md'));
          for (const file of web.files) {
            assert.match(file.sha256, /^[a-f0-9]{64}$/);
            assert.equal(new URL(file.url).origin, new URL(fixture.source).origin);
          }
          expectedSource = fixture.source;
          // Public info exposes only immutable 40-hex Git revisions. HTTPS
          // provenance remains in the verified lock, not a fabricated info field.
          expectedRevision = undefined;
        }
        assert.match(pin.tree_digest, /^sha256:[a-f0-9]{64}$/);
        assert.match(pin.manifest_digest, /^sha256:[a-f0-9]{64}$/);
        const common = ['--target', 'codex', '--scope', 'project', '--project-root', project, '--format', 'json'];
        const infoOutput = invoke(['info', pin.alias, ...common]);
        evidence.lifecycle.info = commandEvidence(infoOutput);
        const info = completeJSON(infoOutput, 'info').data;
        assert.equal(info.name, pin.alias);
        assert.equal(info.source, expectedSource);
        assert.ok(typeof info.installation_id === 'string' && info.installation_id.length > 0);
        assert.equal(info.clients?.length, 1);
        const client = info.clients[0];
        assert.equal(client.client_id, 'codex');
        assert.equal(client.package_revision?.tree_digest, pin.tree_digest);
        assert.equal(client.package_revision?.manifest_digest, pin.manifest_digest);
        assert.equal(client.package_revision?.resolved_revision, expectedRevision);
        // CLI-reported reconciliation is not an independently observed Codex session.
        evidence.lifecycle.native_qualified = false;
        evidence.lifecycle.reported_native_discovery_reconciled = client.native_discovery_reconciled ?? null;
        evidence.lifecycle.native_discovery = client.native_discovery_evidence || null;
        evidence.lifecycle.locked_identity = pin;
        evidence.lifecycle.installation_id = info.installation_id;
        const removedOutput = invoke(['remove', info.installation_id, ...common, '--yes']);
        evidence.lifecycle.remove = commandEvidence(removedOutput);
        assert.ifError(removedOutput.error);
        assert.equal(removedOutput.status, 0);
        // Saved dependencies use the existing raw lifecycle result contract.
        const removed = JSON.parse(String(removedOutput.stdout));
        assert.equal(removed.schema_version, 1);
        assert.equal(removed.command, 'agentplugins remove');
        assert.equal(removed.result, 'success');
        assert.equal(removed.outcome, 'completed');
        assert.equal(existsSync(join(nativeSkillsRoot, fixture.skill)), false, 'Selected native Skill remains');
        const remaining = completeJSON(invoke(['list', '--scope', 'project', '--project-root', project, '--format', 'json']), 'list').data.installations;
        assert.ok(Array.isArray(remaining));
        assert.equal(remaining.some((entry: { installation_id: string }) => entry.installation_id === info.installation_id), false, 'Managed installation remains');
        if (existsSync(join(project, 'uap.lock'))) {
          const after = JSON.parse(readFileSync(join(project, 'uap.lock'), 'utf8'));
          assert.ok(Array.isArray(after.packages));
          assert.equal(after.packages.some((entry: { alias: string }) => entry.alias === pin.alias), false, 'Selected lock identity remains');
        }
        const statusOutput = invoke(['deps', 'status', '--scope', 'project', '--project-root', project, '--format', 'json']);
        evidence.lifecycle.saved_environment = commandEvidence(statusOutput);
        assert.ifError(statusOutput.error);
        assert.equal(statusOutput.status, 0);
        // deps status uses the canonical Go manifest parser and verifies the
        // manifest digest against the remaining lock; no second TOML parser.
        const status = JSON.parse(String(statusOutput.stdout));
        assert.ok(Array.isArray(status.facts));
        assert.equal((status.diagnostics || []).length, 0, 'Saved requirement/lock agreement is invalid');
        assert.match(status.manifest_digest, /^sha256:[a-f0-9]{64}$/);
        assert.equal(status.manifest_digest, status.lock_digest);
        assert.equal(status.pending_recovery, false);
        assert.equal(status.facts.some((fact: { alias?: string }) => fact.alias === pin.alias), false);
        assert.equal(readFileSync(foreign, 'utf8'), 'foreign Skill sentinel\n');
        assert.equal(readFileSync(projectSentinel, 'utf8'), 'foreign project sentinel\n');
        evidence.lifecycle.removed = true;
      } catch (error) {
        evidence.lifecycle.error = error instanceof Error ? error.message : String(error);
        failures.push('Native Skill lifecycle failed for ' + fixture.id + ': ' + evidence.lifecycle.error);
      }
    }
    const versionAfter = invoke(['version', '--format', 'json']);
    evidence.lifecycle.version_after = commandEvidence(versionAfter);
    try {
      const observed = completeJSON(versionAfter, 'version').data.version;
      assert.ok(typeof observed === 'string' && observed.length > 0);
      evidence.lifecycle.observed_version_after = observed;
      evidence.lifecycle.version_observations_match = observed === evidence.lifecycle.observed_version_before;
      assert.equal(evidence.lifecycle.version_observations_match, true, 'Version changed during lifecycle');
    } catch (error) {
      failures.push('CLI version observation failed after ' + fixture.id + ': ' + String(error));
    }
  }
  // Preserve all eight independent live source results, even when the first
  // publisher is broken. The suite still fails closed; no fixture can give
  // another source a false green.
  assert.deepEqual(failures, [], 'Native source acquisition failures; inspect saved evidence');
} finally {
  writeFileSync(join(reportRoot, published ? 'npm-install.json' : 'native-install.json'),
    JSON.stringify(report, null, 2) + '\n');
  rmSync(runRoot, { recursive: true, force: true });
}
