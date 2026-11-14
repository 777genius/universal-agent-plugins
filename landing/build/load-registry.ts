import { readFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import type { RegistryIndex } from '../types/registry';
import { parseDirectoryData } from '../utils/registry.ts';
import { record } from '../utils/registry-codecs/common.ts';
import { parseModernSnapshot } from '../utils/registry-codecs/signed.ts';

export function loadRegistryIndex(
  path: string,
  mode?: 'published_snapshot' | 'review_preview',
): RegistryIndex {
  let sourceText: string;
  try {
    sourceText = readFileSync(path, 'utf8');
  } catch (error) {
    throw new Error(`Unable to read registry index at ${path}: ${String(error)}`);
  }
  try {
    return parseDirectoryData(JSON.parse(sourceText) as unknown, mode);
  } catch (error) {
    throw new Error(`Invalid registry index at ${path}: ${String(error)}`);
  }
}

// Only reviewed build composition supplies expectedFeedID. The generated mirror
// marker binds the already verified input; it is not a new signing authority or
// a way for a candidate snapshot to choose its reader. Production uses the
// historical loader above until the independent issuer is activated.
export function loadModernRegistryIndex(
  path: string,
  markerPath: string,
  expectedFeedID: string,
): RegistryIndex {
  if (!expectedFeedID || expectedFeedID !== expectedFeedID.trim() || /[/\\\s]/.test(expectedFeedID))
    throw new Error('modern registry requires a reviewed feed identity');
  const original = readFileSync(path);
  const marker: unknown = JSON.parse(readFileSync(markerPath, 'utf8'));
  const digest = `sha256:${createHash('sha256').update(original).digest('hex')}`;
  if (!record(marker) || marker.schema_version !== 1 ||
    marker.directory_feed_id !== expectedFeedID || marker.directory_digest !== digest)
    throw new Error('modern registry differs from its verified mirror lineage or bytes');
  const input: unknown = JSON.parse(original.toString('utf8'));
  if (!record(input) || input.sequence !== marker.directory_sequence ||
    input.source_commit !== marker.directory_source_commit)
    throw new Error('modern registry differs from its verified mirror identity');
  return parseModernSnapshot(input);
}
