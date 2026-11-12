/** Parse only LF-complete live records; closed evidence must have no unfinished tail. */
export function parseNativeRows(text: string, closed = false): Record<string, any>[] {
 const complete = text.slice(0, text.lastIndexOf('\n') + 1);
 if (closed && complete.length !== text.length) throw new Error('Native evidence has an unterminated JSONL record');
 const lines = complete.trim().split('\n');
 return (closed ? lines : lines.filter(Boolean)).map((line) => JSON.parse(line));
}
