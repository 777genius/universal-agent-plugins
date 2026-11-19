import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { lstatSync, readFileSync } from 'node:fs';
import { test } from 'node:test';
import ts from 'typescript';

const fonts = new URL('../public/fonts/', import.meta.url);
const stylesheet = readFileSync(new URL('fonts.css', fonts), 'utf8');
const sources = JSON.parse(readFileSync(new URL('sources.json', fonts), 'utf8')) as {
  files: Array<{
    file: string;
    url: string;
    family: string;
    subset: string;
    bytes: number;
    sha256: string;
  }>;
  licenses: Array<{ file: string }>;
};

test('Nuxt serves the existing typography locally without Google Fonts load dependencies', () => {
  const config = readFileSync(new URL('../nuxt.config.ts', import.meta.url), 'utf8');
  const links = [...config.matchAll(
    /\{ rel: 'stylesheet', href: (`\$\{baseURL\}fonts\/fonts\.css`) \}/g,
  )];
  assert.equal(links.length, 1);
  const hrefForBase = Function('baseURL', `return ${links[0][1]}`) as (base: string) => string;
  assert.doesNotMatch(config, /fonts\.(?:googleapis|gstatic)\.com/);
  const urls = [...stylesheet.matchAll(/url\(([^)]+)\)/g)].map((match) => match[1]);
  assert.equal(urls.length, 40);
  assert.deepEqual(
    new Set(urls),
    new Set(sources.files.map((font) => `./${font.file}`)),
  );
  for (const url of urls) {
    assert.match(url, /^\.\/[a-z0-9-]+\.woff2$/);
    for (const base of ['/', '/universal-agent-plugins/']) {
      const css = new URL(hrefForBase(base), 'https://example.test');
      assert.equal(css.pathname, `${base}fonts/fonts.css`);
      const asset = new URL(url, css);
      assert.equal(asset.origin, css.origin);
      assert.equal(asset.pathname, `${base}fonts/${url.slice(2)}`);
    }
  }
  assert.doesNotMatch(stylesheet, /https?:|@import/);
});

test('standalone fonts leave the existing Nuxt inline-style policy unchanged', () => {
  const source = ts.createSourceFile(
    'nuxt.config.ts',
    readFileSync(new URL('../nuxt.config.ts', import.meta.url), 'utf8'),
    ts.ScriptTarget.Latest,
    true,
    ts.ScriptKind.TS,
  );
  const exported = source.statements.find(ts.isExportAssignment);
  assert.ok(exported && ts.isCallExpression(exported.expression));
  const config = exported.expression.arguments[0];
  assert.ok(config && ts.isObjectLiteralExpression(config));
  const features = config.properties.find(
    (property) => ts.isPropertyAssignment(property) && property.name.getText(source) === 'features',
  );
  assert.ok(features && ts.isPropertyAssignment(features));
  assert.ok(ts.isObjectLiteralExpression(features.initializer));
  const property = features.initializer.properties.find(
    (item) => ts.isPropertyAssignment(item) && item.name.getText(source) === 'inlineStyles',
  );
  assert.ok(property && ts.isPropertyAssignment(property));
  assert.ok(ts.isArrowFunction(property.initializer));
  // Execute only the actual pure predicate, without evaluating registry build inputs.
  const inlineStyles = Function(`return (${property.initializer.getText(source)})`)() as
    (id?: string) => boolean;
  assert.equal(inlineStyles('/repo/landing/assets/styles/fonts.css'), true);
  assert.equal(inlineStyles('C:\\repo\\landing\\assets\\styles\\fonts.css'), true);
  assert.equal(inlineStyles('/repo/landing/layouts/default.vue'), false);
  assert.equal(inlineStyles('/repo/landing/pages/plugins/[slug].vue'), false);
  assert.equal(inlineStyles('/repo/landing/assets/styles/main.scss'), true);
  assert.equal(inlineStyles('/repo/landing/components/shared/CommandSnippetCard.vue'), true);
  assert.equal(inlineStyles(undefined), true);
});

test('every local font is the pinned regular WOFF2 asset with its distribution notice', () => {
  assert.equal(sources.files.length, 13);
  assert.equal(new Set(sources.files.map((font) => font.file)).size, 13);
  for (const font of sources.files) {
    assert.match(font.file, /^[a-z0-9-]+\.woff2$/);
    assert.match(font.url, /^https:\/\/fonts\.gstatic\.com\/s\//);
    const file = new URL(font.file, fonts);
    assert.ok(lstatSync(file).isFile() && !lstatSync(file).isSymbolicLink());
    const body = readFileSync(file);
    assert.equal(body.length, font.bytes);
    assert.equal(body.subarray(0, 4).toString(), 'wOF2');
    assert.equal(body.readUInt32BE(8), body.length);
    assert.equal(createHash('sha256').update(body).digest('hex'), font.sha256);
  }
  assert.deepEqual(
    sources.licenses.map((license) => license.file).sort(),
    ['OFL-Inter.txt', 'OFL-JetBrainsMono.txt'],
  );
  for (const license of sources.licenses) {
    assert.match(readFileSync(new URL(license.file, fonts), 'utf8'), /SIL OPEN FONT LICENSE Version 1\.1/);
  }
});

test('local faces preserve the existing families, weights, script coverage and swap behavior', () => {
  const faces = [...stylesheet.matchAll(/@font-face\s*\{([^}]+)\}/g)].map((match) => match[1]);
  assert.equal(faces.length, 40);
  const weights = new Map<string, Set<string>>();
  for (const face of faces) {
    assert.match(face, /font-style:\s*normal;/);
    assert.match(face, /font-display:\s*swap;/);
    assert.match(face, /unicode-range:\s*U\+/);
    const family = face.match(/font-family:\s*'([^']+)'/)![1];
    const weight = face.match(/font-weight:\s*(\d+)/)![1];
    if (!weights.has(family)) weights.set(family, new Set());
    weights.get(family)!.add(weight);
  }
  assert.deepEqual([...weights.keys()].sort(), ['Inter', 'JetBrains Mono']);
  assert.deepEqual([...weights.get('Inter')!].sort(), ['400', '600', '700', '800']);
  assert.deepEqual([...weights.get('JetBrains Mono')!].sort(), ['400', '600']);
  for (const family of ["'Inter'", "'JetBrains Mono'"]) {
    const subsets = sources.files.filter((font) => font.family === family).map((font) => font.subset);
    for (const subset of ['latin', 'latin-ext', 'cyrillic', 'cyrillic-ext']) {
      assert.ok(subsets.includes(subset), `${family}: missing ${subset}`);
    }
  }
});
