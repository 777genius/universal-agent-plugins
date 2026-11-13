'use strict';
// Execute only the actual pure diagnostic functions, never driver main or Playwright.
const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const { stripTypeScriptTypes } = require('node:module');
const { join } = require('node:path');
const { test } = require('node:test');
const vm = require('node:vm');
const source = readFileSync(join(__dirname, 'driver.ts'), 'utf8');
const start = '// BEGIN CLOSED FAILURE DIAGNOSTICS:';
const end = '// END CLOSED FAILURE DIAGNOSTICS';
const begin = source.indexOf(start), finish = source.indexOf(end);
assert(begin > 0 && finish > begin && source.indexOf(start, begin + 1) === -1);
const pure = source.slice(begin, finish);
assert(!/require\(|spawn\(|process\.|chromium\.|writeFile/u.test(pure));
const sandbox = { Buffer, Error, LIMIT: 1 << 20 };
vm.createContext(sandbox);
vm.runInContext(stripTypeScriptTypes(pure, { mode: 'strip' }) + '\nglobalThis.facts = failureFacts; globalThis.append = appendDiagnostic;', sandbox);
const facts = (...args) => JSON.parse(JSON.stringify(sandbox.facts(...args)));
const normal = (error, capture, body = '') => facts(error, capture, null, null, false, false, Buffer.from(body), Buffer.byteLength(body), 0);

test('actual diagnostic append stays capped through repeated native flood chunks', () => {
  const chunk = Buffer.alloc(256 << 10, 0x61);
  let retained = Buffer.alloc(0), total = 0;
  for (let index = 0; index < 256; index++) {
    retained = sandbox.append(retained, chunk);
    total += chunk.length;
    assert(retained.length <= (1 << 20) + 1);
  }
  assert.equal(retained.length, (1 << 20) + 1);
  assert.equal(retained[retained.length - 1], 0x61);
  const row = facts(new Error('native merged stream overflow'), undefined, null, null, false, false, retained, total, 0);
  assert.equal(row.reason, 'publisher-stream-overflow');
  assert.equal(row.publisherMergedBytes, (1 << 20) + 1);
  assert.equal(row.mergedBytesCapped, true);
  const giant = sandbox.append(Buffer.alloc(0), Buffer.alloc(4 << 20, 0x62));
  assert.equal(giant.length, (1 << 20) + 1);
  assert(source.includes('diagnostic = appendDiagnostic(diagnostic, chunk)'));
  assert(!source.includes('diagnostic = Buffer.concat([diagnostic, chunk])'));
});

test('known own reasons and capture failures retain only the closed vocabulary', () => {
  const row = normal(new Error('publisher startup failed'), new Error('publisher proof missing'));
  assert.equal(row.reason, 'publisher-startup-failed');
  assert.equal(row.captureReason, 'publisher-proof-missing');
  assert.equal(row.proofSeen, false);
  assert.equal(row.endpointSeen, false);
  assert.equal(normal(new Error('stage deadline exceeded')).reason, 'stage-deadline');
});

test('arbitrary exception, endpoint, cookie, profile and native buffer bytes never escape', () => {
  const privateValues = ['ws://127.0.0.1:52345/devtools/browser/private-endpoint', 'cookie-private-0123456789', '/private/user/profile/config-secret'];
  const privateText = privateValues.join(' ');
  for (const error of [new Error(privateText), { message: privateText }, privateText]) {
    const value = normal(error, new Error(privateText), privateText);
    assert.equal(value.reason, 'unclassified');
    assert.equal(value.captureReason, 'unclassified');
    assert(!JSON.stringify(value).includes('private'));
    assert(!JSON.stringify(value).includes('127.0.0.1'));
  }
});

test('publisher terminal facts and byte counts have fixed safe bounds', () => {
  const row = facts(new Error('publisher startup failed'), undefined, 127, 'SIGABRT', true, true, Buffer.alloc(0), 2 ** 40, 2 ** 40);
  assert.equal(row.publisherExitCode, 127);
  assert.equal(row.publisherSignal, 'SIGABRT');
  assert.equal(row.proofSeen, true);
  assert.equal(row.endpointSeen, true);
  assert.equal(row.publisherMergedBytes, (1 << 20) + 1);
  assert.equal(row.preVerifierStderrBytes, (1 << 20) + 1);
  assert.equal(row.mergedBytesCapped, true);
  const invalid = facts(undefined, undefined, 'private-code', 'private-signal', false, false, Buffer.alloc(0), NaN, -1);
  assert.equal(invalid.publisherExitCode, null);
  assert.equal(invalid.publisherSignal, 'other');
  assert.equal(invalid.publisherMergedBytes, 0);
  assert(!JSON.stringify(invalid).includes('private'));
  const opaque = facts(undefined, undefined, {}, { toString: () => 'SIGTERM', private: 'must-not-escape' }, false, false, Buffer.alloc(0), 0, 0);
  assert.equal(opaque.publisherSignal, 'other');
  assert(!JSON.stringify(opaque).includes('must-not-escape'));
});

test('native startup signatures are fixed booleans and do not expose matched text', () => {
  for (const [text, field] of [
    ['No usable sandbox!', 'sandboxUnavailable'],
    ['Failed to move to new namespace', 'namespaceDenied'],
    ['error while loading shared libraries: private.so', 'sharedLibraryMissing'],
    ["version GLIBC_2.39 not found in /private/library", 'glibcVersionMissing'],
    ['Missing X server or $DISPLAY', 'displayUnavailable']
  ]) {
    const row = normal(new Error('publisher startup failed'), undefined, text);
    assert.equal(row.startupSignatures[field], true);
    assert(Object.values(row.startupSignatures).every(value => typeof value === 'boolean'));
    assert(!JSON.stringify(row).includes(text));
  }
});

test('failure reporting has no free-form native UI text or exception serialization', () => {
  const ui = source.slice(source.indexOf('async function failureUI'), source.indexOf('async function main'));
  assert(!/textContent|getAttribute|\.slice\(/u.test(ui));
  const reporting = source.slice(source.indexOf('  } catch (error) {\n    const metadata ='));
  assert(reporting.includes('diagnostics: failureFacts('));
  assert(!/String\(error\)|error\.message|diagnostic\.toString|endpoint\s*[,}]/u.test(reporting));
  assert(reporting.includes('diagnostic.fill(0)'));
});
