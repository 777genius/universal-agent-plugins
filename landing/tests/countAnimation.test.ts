import assert from 'node:assert/strict';
import { describe, it } from 'node:test';
import { countAtElapsed, PLUGIN_COUNT_ANIMATION_MS } from '../utils/countAnimation.ts';

describe('plugin count animation', () => {
  it('counts linearly from zero to the exact total in two seconds', () => {
    assert.equal(PLUGIN_COUNT_ANIMATION_MS, 2_000);
    assert.equal(countAtElapsed(2_875, 0), 0);
    assert.equal(countAtElapsed(2_875, 1_000), 1_437);
    assert.equal(countAtElapsed(2_875, 2_000), 2_875);
    assert.equal(countAtElapsed(2_875, 3_000), 2_875);
  });

  it('stays monotonic and handles invalid totals safely', () => {
    const values = Array.from({ length: 121 }, (_, index) =>
      countAtElapsed(2_875, (PLUGIN_COUNT_ANIMATION_MS * index) / 120),
    );
    assert.ok(values.every((value, index) => index === 0 || value >= values[index - 1]!));
    assert.equal(countAtElapsed(-10, 1_000), 0);
  });
});
