import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { test } from 'node:test';

const root = new URL('../../../', import.meta.url);
const read = (path) => readFileSync(new URL(path, root), 'utf8');
// Use the site's installed VitePress, or a read-only matching cache in isolated reviews.
// VITEPRESS_TEST_MODULE is an absolute path to vitepress/dist/node/index.js.
const websiteRequire = createRequire(new URL('website/package.json', root));
const { createMarkdownRenderer } = await import(pathToFileURL(
  process.env.VITEPRESS_TEST_MODULE || websiteRequire.resolve('vitepress'),
).href);
const md = await createMarkdownRenderer(fileURLToPath(new URL('website/', root)));

// Exact heading sequence extracted from original main
// 01f02cb51cfe5f664d4d5f52b295c59c7ea03495:website/source/{locale}/guide/quickstart.md.
// Render with VitePress's actual slugger: accents, Cyrillic and punctuation matter.
const oldHeadings = {
  en: `# Quickstart
## Recommended Default
## Start With The Job
## Optional First Proof
## If You Only Read One Thing
## Legacy Compatibility Path
## Install The CLI For Daily Use
## Why This Path Still Exists
## What You Get
## Supported Node And Python Paths
## If You Are Intentionally Starting On Node Or Python
## What To Do Next
## Expand Later
## What Expands Later
## After Quickstart`,
  ru: `# Быстрый старт
## Начните с задачи
## Опциональная быстрая проверка
## Если читать только одно
## Legacy path для совместимости
## Установите CLI для ежедневной работы
## Почему этот путь всё ещё существует
## Что вы получите
## Поддерживаемые пути для Node и Python
## Если вы осознанно начинаете с Node или Python
## Что делать дальше
## Что добавлять потом
## Что расширяется потом
## Что читать дальше`,
  es: `# Inicio rápido
## Si solo lees una cosa
## Valor predeterminado recomendado
## Por qué este es el valor predeterminado
## Lo que obtienes
## Rutas Node y Python admitidas
## Si está comenzando intencionalmente en Node o Python
## Qué hacer a continuación
## Ampliar más tarde
## Lo que se expande más tarde
## Después del inicio rápido`,
  fr: `# Démarrage rapide
## Si vous ne lisez qu'une chose
## Valeur par défaut recommandée
## Pourquoi c'est la valeur par défaut
## Ce que vous obtenez
## Chemins Node et Python pris en charge
## Si vous commencez intentionnellement le Node ou Python
## Que faire ensuite
## Développer plus tard
## Ce qui se développera plus tard
## Après le démarrage rapide`,
  zh: `# 快速入门
## 如果你只读一件事
## 推荐默认值
## 为什么这是默认值
## 你得到什么
## 支持 Node 和 Python 路径
## 如果您有意从 Node 或 Python 开始
## 接下来做什么
## 稍后展开
## 稍后扩展的内容
## 快速入门后`,
};
const ids = (html) => [...html.matchAll(/\bid="([^"]+)"/g)].map(m => m[1]);
const headingIds = (html) => [...html.matchAll(/<h[1-6] id="([^"]+)"/g)].map(m => m[1]);

assert.equal(Object.values(oldHeadings).reduce((total, headings) => total + headings.split('\n').length, 0), 62);

for (const [locale, headings] of Object.entries(oldHeadings)) {
  test(`${locale}: every original quickstart fragment resolves exactly once`, async () => {
    const original = headingIds(await md.renderAsync(headings));
    const source = read(`website/source/${locale}/guide/quickstart.md`);
    const html = await md.renderAsync(source);
    const current = ids(html);
    assert.equal(original.length, headings.split('\n').length);
    for (const id of [...original, 'use-plugins', 'build-plugins', 'historical-v1']) {
      assert.equal(current.filter(value => value === id).length, 1, `${locale}: #${id}`);
    }
    // Removed headings must have explicit aliases, not incidental strings in prose/code.
    const retained = new Set(headingIds(html));
    for (const id of original.filter(id => !retained.has(id))) {
      assert.ok(html.includes(`<a id="${id}"></a>`), `${locale}: alias #${id}`);
    }
    assert.ok(html.indexOf(`id="${original[0]}"`) < html.indexOf('<h1 '));
    const history = html.indexOf('<h2 id="historical-v1"');
    const legacyIndexes = locale === 'en' ? [2, 5, 6, 7] : locale === 'ru' ? [1, 4, 5, 6] : [1, 2, 3];
    for (const index of legacyIndexes) {
      const position = html.indexOf(`id="${original[index]}"`);
      assert.ok(position > history, `${locale}: historical destination #${original[index]}`);
      assert.ok(position < html.indexOf('<h3 '), `${locale}: alias near historical instructions`);
    }
    if (locale === 'en' || locale === 'ru') {
      const proof = original[locale === 'en' ? 3 : 2];
      assert.ok(html.indexOf(`id="${proof}"`) > html.indexOf('<h2 id="use-plugins"'));
      assert.ok(html.indexOf(`id="${proof}"`) < html.indexOf('<h2 id="build-plugins"'));
    }
  });
}

test('README promotes the canonical quickstart and preserves entry fragments', () => {
  const source = read('README.md');
  const canonical = 'https://777genius.github.io/universal-agent-plugins/docs/en/guide/quickstart.html';
  const links = [...source.matchAll(/\[([^\]]+)\]\(([^)]+)\)/g)];
  for (const label of ['Use / Build quickstart', 'Quickstart']) {
    assert.deepEqual(links.filter(m => m[1] === label).map(m => m[2]), [canonical]);
  }
  assert.match(source, /^### Quick start$/m);
  assert.match(source, /<a id="authoring-and-development"><\/a>/);
});
