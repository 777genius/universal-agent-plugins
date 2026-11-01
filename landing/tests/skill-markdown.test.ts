import { test } from 'node:test';
import assert from 'node:assert/strict';
import { renderSkillMarkdown, skillDocumentBody } from '../utils/skillMarkdown.ts';

test('skill Markdown renders headings, lists, links, tables, quotes and fenced code', () => {
  const raw = [
    '---', 'name: lark-mail', 'metadata:', '  requires:', '    bins: [lark-cli]', '---',
    '# Mail', '', '**重要**', '', '- Alpha', '- Beta', '',
    '| Key | Value |', '| --- | --- |', '| one | two |', '',
    '> Review the source', '',
    '```bash', 'echo safe', '```', '',
    '[Repository](https://github.com/larksuite/cli)',
  ].join('\n');
  const html = renderSkillMarkdown(raw);
  assert.doesNotMatch(html, /metadata|bins:/);
  assert.match(html, /<h1>Mail<\/h1>/);
  assert.match(html, /<strong>重要<\/strong>/);
  assert.match(html, /<table>/);
  assert.match(html, /<blockquote>/);
  assert.match(html, /<pre>/);
  assert.match(html, /target="_blank"/);
  assert.match(html, /noopener noreferrer nofollow/);
  assert.equal(skillDocumentBody('Plain text'), 'Plain text');
});

test('untrusted Markdown cannot inject HTML, scripts, image tracking, or unsafe links', () => {
  const raw = [
    '<script>alert(1)</script>',
    '<img src="https://attacker.test/tracker" onerror="alert(1)">',
    '<iframe src="https://attacker.test"></iframe>',
    '![Tracker](https://attacker.test/pixel.png)',
    '[Bad](javascript:alert(1))',
    '[Competitor](https://www.skills.sh/site/example)',
    '[Relative](../secrets/SKILL.md)',
    '[Good](https://github.com/larksuite/cli)',
  ].join('\n\n');
  const html = renderSkillMarkdown(raw);
  assert.doesNotMatch(html, /<script|<img|<iframe|href="javascript:|href="\.\.|href="https:\/\/www\.skills\.sh/i);
  assert.match(html, /&lt;script&gt;/);
  assert.match(html, /href="https:\/\/github\.com\/larksuite\/cli"/);
  assert.doesNotMatch(html, /<a[^>]*>Competitor<\/a>/);
});
