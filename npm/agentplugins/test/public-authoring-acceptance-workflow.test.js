'use strict';
// Source contracts only. These tests never dispatch, sign, or qualify E.
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const a = require('../scripts/public-authoring-acceptance');
const text = fs.readFileSync(path.resolve(__dirname, '../../../.github/workflows/authoring-public-packed.yml'), 'utf8');
const jobs = Object.fromEntries([...text.matchAll(/^  (public_[a-z_]+):\n([\s\S]*?)(?=^  public_[a-z_]+:|$(?![\s\S]))/gm)]
  .map(m => [m[1], m[2]]));

test('C3 workflow four invocations retain all cells and exact completed jobs', () => {
  assert.deepEqual(Object.keys(jobs), ['public_inputs', 'public_cell', 'public_producer_complete', 'public_assemble',
    'public_evidence_intake', 'public_evidence_attestation', 'public_check']);
  assert.match(text, /options: \[produce, assemble, attest, check\]/);
  assert.match(jobs.public_cell, /fail-fast: false/);
  assert.match(jobs.public_cell, /needs: public_inputs/);
  assert.match(jobs.public_cell, /name: public_cell \(\$\{\{ matrix.cell \}\}\)/);
  assert.match(jobs.public_cell, /matrix: \$\{\{ fromJSON\(needs.public_inputs.outputs.matrix\) \}\}/);
  assert.match(jobs.public_producer_complete, /needs: \[public_inputs, public_cell\]/);
  assert.match(jobs.public_producer_complete, /== os.environ\['CELLS_RESULT'\] == 'success'/);
  assert.equal(a.PUBLIC_JOBS.produce.length, 20);
  assert.equal(new Set(a.matrix.map(r => r.key)).size, 18);
  for (const [mode, names] of Object.entries(a.PUBLIC_JOBS)) {
    const producer = { workflow: a.WORKFLOW, source: 'a'.repeat(40), ref: 'refs/tags/agentplugins-v2.0.0', run_id: 50, run_attempt: 2 };
    const selected = { tag: 'agentplugins-v2.0.0', ref: producer.ref, source: producer.source, versions: {} };
    const attempt = { producer, status: 'completed', conclusion: 'success', jobs: names.map((name, i) => ({
      id: i + 1, name, run_id: 50, run_attempt: 2, source: producer.source, ref: producer.ref, status: 'completed', conclusion: 'success'
    })) };
    assert.deepEqual(a.completedAttempt(attempt, selected, mode), producer);
    for (let i = 0; i < names.length; i++) for (const field of ['run_attempt', 'source', 'status', 'conclusion']) {
      const bad = structuredClone(attempt);
      bad.jobs[i][field] = { run_attempt: 1, source: 'b'.repeat(40), status: 'in_progress', conclusion: 'skipped' }[field];
      assert.throws(() => a.completedAttempt(bad, selected, mode), `${mode}/${names[i]}/${field}`);
    }
  }
});

test('C3 workflow isolates E2 signing and rechecks closure after signing', () => {
  for (const [name, body] of Object.entries(jobs)) {
    if (name === 'public_evidence_attestation') continue;
    assert.doesNotMatch(body, /id-token: write|attestations: write|actions\/attest@/);
  }
  const signer = jobs.public_evidence_attestation;
  assert.match(signer, /needs: public_evidence_intake/);
  assert.match(signer, /runs-on: ubuntu-24.04/);
  assert.match(signer, /id-token: write\n      attestations: write/);
  assert.match(signer, /subject-path: \$\{\{ steps.evidence.outputs.subjects \}\}/);
  assert.ok(signer.indexOf("packed.public_workflow('attest')") < signer.indexOf('uses: actions/attest@'));
  assert.ok(signer.indexOf('uses: actions/attest@') < signer.indexOf("checked = packed.public_workflow('attest')"));
  assert.ok(signer.indexOf('assert closure(') < signer.indexOf('uses: actions/upload-artifact@'));
  assert.match(signer, /assert closure\(os.environ\['PUBLIC_SIGNED_ROOT'\]\) == closure\(checked\['root'\]\)/);
  assert.doesNotMatch(text, /contents: write|packages: write|gh release|npm publish|workflow_call:|push:/);
});

test('C3 workflow pins source actions and independent controller bootstrap', () => {
  const uses = [...text.matchAll(/uses: ([^\s]+)/g)].map(m => m[1]);
  assert.ok(uses.length > 0);
  for (const use of uses) assert.match(use, /^actions\/(checkout|upload-artifact|attest)@[0-9a-f]{40}$/);
  for (const [name, body] of Object.entries(jobs)) {
    if (name === 'public_producer_complete') continue;
    assert.match(body, /persist-credentials: false/);
    assert.match(body, /s\['source'\] == os.environ\['GITHUB_SHA'\] == os.environ\['GITHUB_WORKFLOW_SHA'\]/);
    assert.match(body, /shell: python -I -B \{0\}/);
    assert.match(body, /packed.public_workflow\(/);
    assert.doesNotMatch(body, /setup-node|node-version|node npm\//);
  }
  assert.doesNotMatch(jobs.public_assemble, /needs:/);
  assert.doesNotMatch(jobs.public_evidence_intake, /needs:/);
  assert.doesNotMatch(jobs.public_check, /needs:/);
});
