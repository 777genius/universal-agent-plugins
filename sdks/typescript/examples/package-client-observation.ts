// Pinned native package discovery. This never substitutes copied SDK metadata
// for the client's own fresh plugin and Skill inventories.
import assert from 'node:assert/strict';
import { join, resolve } from 'node:path';
import { command, contained, parseAgentSkills, type NativeObservation, type Options } from './native-client-observation.ts';
const name = 'uap-commands-fixture';
type Row = Record<string, unknown>;
function array(value: unknown): Row[] { assert.ok(Array.isArray(value)); for (const row of value) assert.ok(row && typeof row === 'object' && !Array.isArray(row)); return value as Row[]; }
const args = (o: Options, tail: string[]) => o.client === 'qoder' ? ['--config-dir', o.nativeRoot, ...tail] : tail;
const run = (o: Options, tail: string[]) => command(o, args(o, tail));
async function add(o: Options, skills: Map<string, string>, skill: string, path: string, base: string) {
  assert.equal(skills.has(skill), false, 'Duplicate native package Skill');
  assert.equal(path, join(base, 'skills', skill, 'SKILL.md'), 'Unexpected native Skill path');
  await contained(o.root, base); await contained(base, path); skills.set(skill, path);
}
export async function inspectPackageNative(o: Options): Promise<NativeObservation> {
  const skills = new Map<string, string>();
  if (o.client === 'copilot') {
    const text = await run(o, ['plugin', 'list']);
    // Built-in rows are permitted only in their own preamble. Every fixture
    // live row must have the complete identity/version/enabled/path grammar.
    const fixtureLines = text.split(/\r?\n/).filter((line) => /^\s*•\s/.test(line) && line.includes(name));
    const matches = [...text.matchAll(/^  • (uap-commands-fixture@[^\s]+) \(v1\.0\.0\) \(enabled\)\r?\n      from ([^\r\n]+)(?:\r?\n|$)/gm)];
    assert.equal(matches.length, fixtureLines.length, 'Malformed, disabled or truncated Copilot fixture row');
    assert.ok(matches.length <= 1, 'Duplicate Copilot fixture');
    const entry = matches[0]; const pluginId = entry?.[1] ?? o.expectedPluginId;
    if (entry) { assert.ok(text.indexOf('Live Plugins (') < entry.index! && text.indexOf('Live Plugins (') >= 0); assert.ok(o.managedArtifact); assert.equal(entry[2], o.managedArtifact); await contained(o.root, entry[2]!); }
    if (o.expectedPluginId && entry) assert.equal(pluginId, o.expectedPluginId);
    const rows = array(JSON.parse(await run(o, ['skill', 'list', '--json'])));
    for (const row of rows) {
      if (row.source !== 'plugin') continue;
      assert.equal(typeof row.name, 'string'); assert.equal(typeof row.path, 'string');
      assert.ok(entry, 'Native plugin Skill remains after plugin disappearance'); assert.equal(row.enabled, true);
      await add(o, skills, row.name as string, join(row.path as string, 'SKILL.md'), entry[2]!);
    }
    assert.equal(Boolean(entry), skills.size > 0, 'Copilot plugin/Skill inventory disagreement');
    return { skills, ...(pluginId ? { pluginId } : {}) };
  }
  if (o.client === 'grok') {
    const entries = array(JSON.parse(await run(o, ['plugin', 'list', '--json']))).filter((r) => r.name === name);
    assert.ok(entries.length <= 1, 'Duplicate Grok fixture'); const entry = entries[0];
    if (entry) { assert.equal(entry.status, 'installed'); assert.equal(entry.version, '1.0.0'); assert.ok(o.managedArtifact); assert.equal(entry.source, o.managedArtifact); assert.equal(typeof entry.path, 'string'); await contained(o.root, entry.path as string); }
    const inspect = JSON.parse(await run(o, ['inspect', '--json'])) as Row;
    assert.equal(inspect.grokVersion, '1.0.67'); assert.equal(resolve(inspect.cwd as string), o.project);
    const plugins = array(inspect.plugins).filter((r) => r.name === name);
    assert.equal(plugins.length, entries.length);
    if (entry) {
      assert.equal(plugins[0]!.enabled, true); assert.equal(plugins[0]!.scope, 'user');
      assert.ok(plugins[0]!.path === o.managedArtifact || plugins[0]!.path === entry.path, 'Grok discovered another plugin artifact');
      await contained(o.root, plugins[0]!.path as string);
    }
    for (const row of array(inspect.skills)) {
      const source = row.source as Row | undefined;
      if (source?.type !== 'plugin' || source.plugin_name !== name) continue;
      assert.ok(entry, 'Grok Skill remains after uninstall'); assert.equal(typeof row.name, 'string'); assert.equal(typeof source.path, 'string');
      await add(o, skills, row.name as string, source.path as string, plugins[0]!.path as string);
    }
    assert.equal(Boolean(entry), skills.size > 0, 'Grok plugin/Skill inventory disagreement');
    return { skills, pluginId: name };
  }
  assert.equal(o.client, 'qoder');
  const entries = array(JSON.parse(await run(o, ['plugins', 'list', '--json']))).filter((r) => r.name === name || r.id === `${name}@local`);
  assert.ok(entries.length <= 1, 'Duplicate Qoder fixture'); const entry = entries[0];
  if (entry) { assert.equal(entry.id, `${name}@local`); assert.equal(entry.version, '1.0.0'); assert.equal(entry.enabled, true); assert.equal(entry.scope, 'user'); assert.equal(entry.installPath, join(o.nativeRoot, 'plugins', 'cache', 'local', name, '1.0.0')); await contained(o.root, entry.installPath as string); }
  for (const [qualified, path] of parseAgentSkills(await run(o, ['skills', 'list']))) {
    if (!qualified.startsWith(`${name}:`)) continue;
    assert.ok(entry, 'Qoder Skill remains after uninstall');
    await add(o, skills, qualified.slice(name.length + 1), path, entry.installPath as string);
  }
  assert.equal(Boolean(entry), skills.size > 0, 'Qoder plugin/Skill inventory disagreement');
  return { skills, pluginId: `${name}@local` };
}

export async function actualUninstallPackageNative(o: Options & { expectedPluginId: string }): Promise<void> {
  const before = await inspectPackageNative(o);
  assert.equal(before.pluginId, o.expectedPluginId); assert.ok(before.skills.size > 0);
  if (o.client === 'copilot') {
    assert.ok(o.expectedPluginId.startsWith(`${name}@`));
    const marketplace = o.expectedPluginId.slice(name.length + 1);
    assert.match(marketplace, /^agentplugins-[a-f0-9]+$/);
    await run(o, ['plugin', 'uninstall', o.expectedPluginId]);
    await run(o, ['plugin', 'marketplace', 'remove', marketplace]);
  } else if (o.client === 'grok') {
    assert.equal(o.expectedPluginId, name); await run(o, ['plugin', 'uninstall', '--confirm', name]);
  } else {
    assert.equal(o.client, 'qoder'); assert.equal(o.expectedPluginId, `${name}@local`);
    await run(o, ['plugins', 'uninstall', name, '--scope', 'user']);
  }
  const after = await inspectPackageNative(o);
  assert.equal(after.skills.size, 0, 'Fresh native plugin registry and Skill discovery must prove absence');
}
