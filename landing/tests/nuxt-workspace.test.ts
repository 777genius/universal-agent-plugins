import assert from 'node:assert/strict';
import { mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { test } from 'node:test';
import { NuxtConfigSchema } from 'nuxt/schema';
import ts from 'typescript';

function configuredWorkspaceDir(): string {
  // Read the real config without evaluating its signed-registry build inputs.
  const source = ts.createSourceFile(
    'nuxt.config.ts',
    readFileSync(new URL('../nuxt.config.ts', import.meta.url), 'utf8'),
    ts.ScriptTarget.Latest,
    true,
    ts.ScriptKind.TS,
  );
  const exported = source.statements.find(ts.isExportAssignment);
  assert.ok(exported && ts.isCallExpression(exported.expression));
  assert.equal(exported.expression.expression.getText(source), 'defineNuxtConfig');
  const config = exported.expression.arguments[0];
  assert.ok(config && ts.isObjectLiteralExpression(config));
  const properties = config.properties.filter(
    (property) =>
      ts.isPropertyAssignment(property) && property.name.getText(source) === 'workspaceDir',
  );
  assert.equal(properties.length, 1, 'Nuxt must explicitly configure workspaceDir');
  const property = properties[0];
  assert.ok(ts.isPropertyAssignment(property) && ts.isStringLiteral(property.initializer));
  return property.initializer.text;
}

test('configured Nuxt workspace stays inside the project despite a parent workspace marker', async (t) => {
  const parent = mkdtempSync(join(tmpdir(), 'landing-nuxt-workspace-'));
  t.after(() => rmSync(parent, { recursive: true, force: true }));
  const project = join(parent, 'landing');
  mkdirSync(project);
  // A stray pnpm settings file does not declare this project as a workspace.
  writeFileSync(join(parent, 'pnpm-workspace.yaml'), 'onlyBuiltDependencies: []\n');
  writeFileSync(join(parent, 'pnpm-lock.yaml'), "lockfileVersion: '9.0'\n");
  writeFileSync(join(project, 'package.json'), '{"private":true}\n');

  // Exercise the installed, lockfile-selected Nuxt resolver, including its
  // vulnerable automatic discovery, without starting a dev server.
  const workspaceSchema = NuxtConfigSchema.workspaceDir;
  assert.ok(workspaceSchema !== null && typeof workspaceSchema === 'object');
  assert.ok('$resolve' in workspaceSchema);
  const resolveWorkspace = workspaceSchema.$resolve;
  assert.ok(typeof resolveWorkspace === 'function');
  const get = async (key: string) => {
    assert.equal(key, 'rootDir');
    return project;
  };
  assert.equal(
    await resolveWorkspace(undefined, get),
    parent,
    'fixture must reproduce parent discovery',
  );
  assert.equal(await resolveWorkspace(configuredWorkspaceDir(), get), project);
});
