/**
 * Parse committed records while a native host is still appending evidence.
 * An EOF fragment, even valid JSON, is not a record until its LF arrives.
 * Malformed terminated records must throw. Completed release receipts must
 * keep their strict whole-file parser instead of using this live reader.
 */
export function liveEvidenceRows(snapshot: string): Record<string, any>[] {
 const committed = snapshot.slice(0, snapshot.lastIndexOf('\n') + 1);
 return committed.trim().split('\n').filter(Boolean).map((line) => JSON.parse(line));
}
