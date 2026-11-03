/** Highlight the small, read-only TOML example; never produce executable HTML. */
export interface TomlExampleToken {
  text: string;
  kind: 'plain' | 'section' | 'key' | 'string' | 'number' | 'comment';
}
export function tomlExampleTokens(source: string): TomlExampleToken[] {
  const tokens: TomlExampleToken[] = [];
  const pattern = /#[^\n]*|"(?:\\.|[^"\\])*"|\[[A-Za-z_][\w.-]*\]|\b[A-Za-z_][\w-]*(?=\s*=)|\b\d+(?:\.\d+)*\b/g;
  let offset = 0;
  for (const match of source.matchAll(pattern)) {
    const index = match.index!;
    if (index > offset) tokens.push({ text: source.slice(offset, index), kind: 'plain' });
    const text = match[0];
    const kind: TomlExampleToken['kind'] = text.startsWith('#') ? 'comment'
      : text.startsWith('[') ? 'section'
      : /^"\d+(?:\.\d+)*"$/.test(text) || /^\d/.test(text) ? 'number'
      : text.startsWith('"') ? 'string' : 'key';
    tokens.push({ text, kind });
    offset = index + text.length;
  }
  if (offset < source.length) tokens.push({ text: source.slice(offset), kind: 'plain' });
  return tokens;
}
