import assert from 'node:assert/strict';
import { describe, it } from 'node:test';
import {
  canonicalPath,
  productSoftwareSchema,
  seoDescription,
  spdxLicenseUrl,
} from '../utils/seo.ts';

describe('SEO foundations', () => {
  it('normalizes every indexable page to its GitHub Pages directory URL', () => {
    assert.equal(canonicalPath('/'), '/');
    assert.equal(canonicalPath('/plugins'), '/plugins/');
    assert.equal(canonicalPath('plugins/gitlab/'), '/plugins/gitlab/');
    assert.equal(canonicalPath('//plugins//gitlab//'), '/plugins/gitlab/');
  });

  it('keeps generated plugin descriptions complete and within snippet guidance', () => {
    const description = seoDescription([
      'Install the Example Agent Plugin for Codex, Claude Code, and Cursor.',
      'A deliberately long explanation '.repeat(8),
    ]);
    assert.ok(description.length <= 160);
    assert.ok(description.endsWith('…'));
    assert.equal(description.includes('  '), false);
  });

  it('publishes complete free software application data', () => {
    const schema = productSoftwareSchema({
      siteUrl: 'https://example.com/product/',
      githubUrl: 'https://github.com/example/product',
      releasesUrl: 'https://github.com/example/product/releases',
      docsUrl: 'https://example.com/product/docs/',
      description: 'One CLI for Agent Plugins 1.0.',
    });
    assert.equal(schema.url, 'https://example.com/product/');
    assert.deepEqual(schema.offers, {
      '@type': 'Offer',
      price: '0',
      priceCurrency: 'USD',
    });
    assert.equal(schema.isAccessibleForFree, true);
    assert.equal(schema.installUrl, 'https://www.npmjs.com/package/universal-agent-plugins');
    assert.equal(schema.screenshot, 'https://example.com/product/og-image.png');
    assert.deepEqual(schema.publisher, { '@id': 'https://example.com/product/#organization' });
    assert.equal(spdxLicenseUrl('Apache-2.0'), 'https://spdx.org/licenses/Apache-2.0.html');
    assert.equal(spdxLicenseUrl('invalid license'), undefined);
  });
});
