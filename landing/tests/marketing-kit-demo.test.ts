import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  marketingKitDependency,
  marketingKitInstallCommand,
  marketingKitName,
  marketingKitRestoreCommand,
  marketingKitUserManifest,
  marketingKitVersion,
} from '../data/marketingKitDemo.ts';

test('Marketing Kit command names the actual published package instead of an empty install', () => {
  assert.equal(marketingKitName, 'marketing-kit');
  assert.equal(marketingKitVersion, '0.1.0');
  assert.equal(
    marketingKitInstallCommand,
    'npx -y universal-agent-plugins install marketing-kit --scope user',
  );
  assert.match(marketingKitInstallCommand, / install marketing-kit --scope user$/);
  assert.doesNotMatch(marketingKitInstallCommand, /marketing-kit@0\.1\.0/);
  assert.match(marketingKitUserManifest, /"777genius\/marketing-kit" = "0\.1\.0"/);
  assert.ok(marketingKitUserManifest.includes(marketingKitDependency));
});

test('short teammate command converges the imported user-scope lock', () => {
  assert.equal(marketingKitRestoreCommand, 'npx -y universal-agent-plugins install --scope user');
  assert.match(marketingKitUserManifest, /claude = \["777genius\/marketing-kit"\]/);
  assert.match(marketingKitUserManifest, /codex = \["777genius\/marketing-kit"\]/);
  assert.match(marketingKitUserManifest, /gemini = \["777genius\/marketing-kit"\]/);
});
