/** Website presentation is maintained separately from the generated technical contract. */
export function assertClientCoverage(
  expected: readonly string[],
  rows: readonly { id: string }[],
  context: string,
): void {
  const known = new Set(expected);
  const seen = new Set<string>();
  for (const { id } of rows) {
    if (!known.has(id)) throw new Error(`${context}: unknown client ${id}`);
    if (seen.has(id)) throw new Error(`${context}: duplicate client ${id}`);
    seen.add(id);
  }
  for (const id of expected) {
    if (!seen.has(id)) throw new Error(`${context}: missing client ${id}`);
  }
}
