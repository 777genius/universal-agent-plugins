import { readFileSync } from 'node:fs'
import type { RegistryIndex } from '../types/registry'
import { parseDirectoryData } from '../utils/registry'

export function loadRegistryIndex(path: string, mode?: 'published_snapshot' | 'review_preview'): RegistryIndex {
  let sourceText: string
  try {
    sourceText = readFileSync(path, 'utf8')
  } catch (error) {
    throw new Error(`Unable to read registry index at ${path}: ${String(error)}`)
  }
  try {
    return parseDirectoryData(JSON.parse(sourceText) as unknown, mode)
  } catch (error) {
    throw new Error(`Invalid registry index at ${path}: ${String(error)}`)
  }
}
