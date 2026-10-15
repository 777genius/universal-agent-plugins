// Enable only after the public CLI release includes default saving and --no-save.
export const SAVED_DEPENDENCIES_CLI_RELEASED = false;

export type DependencySaveMode = 'saved' | 'noSave' | 'local' | 'discovery' | 'chatgpt';

export function dependencySaveMode(source: string, targets: readonly string[]): DependencySaveMode {
  if (/^(?:\.{1,2}[\\/]|[\\/]|~[\\/]|[A-Za-z]:[\\/])/.test(source)) return 'local';
  if (source.startsWith('discovery:')) return 'discovery';
  if (targets.length === 1 && targets[0] === 'chatgpt') return 'chatgpt';
  // Community selectors are already validated and may be shell-quoted paths.
  const github = isPinnedGitSource(source);
  const directory = /^[A-Za-z0-9][A-Za-z0-9._/-]*$/.test(source) && !source.startsWith('.');
  if (!github && !directory) return 'noSave';
  return targets.every((target) => target === 'claude' || target === 'codex') ? 'saved' : 'noSave';
}

export function dependencyAddFlag(mode: DependencySaveMode, released: boolean): string {
  return released && mode === 'noSave' ? ' --no-save' : '';
}

export function isPinnedGitSource(source: string): boolean {
  return /^'?((github:)?[A-Za-z0-9][A-Za-z0-9-]*\/[A-Za-z0-9][A-Za-z0-9._-]*@[0-9a-f]{40})(?:\/\/|$|')/.test(
    source,
  );
}
