// Translator contract: ONE | FEW | MANY (or ZERO | ONE | FEW | MANY).
export function slavicPluralRule(choice: number, choicesLength: number): number {
  const count = Math.abs(choice);
  if (choicesLength === 2) return count === 1 ? 0 : 1;
  const form = count % 10 === 1 && count % 100 !== 11 ? 0
    : count % 10 >= 2 && count % 10 <= 4 && (count % 100 < 12 || count % 100 > 14) ? 1 : 2;
  return choicesLength === 4 ? (count === 0 ? 0 : form + 1) : form;
}
