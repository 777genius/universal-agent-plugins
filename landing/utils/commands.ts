import {
  SAVED_DEPENDENCIES_CLI_RELEASED,
  dependencySaveMode,
  dependencyAddFlag,
  isPinnedGitSource,
} from './savedDependencies.ts';
import type { InstallScope, RegistryPlugin } from '../types/registry';

const CLI_INVOCATION = 'npx -y universal-agent-plugins';

/** Encode one shell argument; safe tokens retain the familiar readable command. */
export function shellToken(value: string): string {
  // A comma is excluded: PowerShell treats an unquoted comma as its array operator.
  return /^[A-Za-z0-9_./:@+=-]+$/.test(value) ? value : `'${value.replaceAll("'", "'\\''")}'`;
}

function communityAddSource(plugin: RegistryPlugin): string {
  const source = plugin.source;
  if (
    !source ||
    typeof source.repository !== 'string' ||
    !/^[A-Za-z0-9][A-Za-z0-9-]*\/[A-Za-z0-9][A-Za-z0-9._-]*$/.test(source.repository) ||
    typeof source.revision !== 'string' ||
    !/^[0-9a-f]{40}$/.test(source.revision)
  )
    throw new Error('Community source requires a GitHub repository and exact commit revision');
  if (
    typeof source.path !== 'string' ||
    source.path.includes('\\') ||
    // An apostrophe has no quoting that is correct in both POSIX shells and PowerShell.
    source.path.includes("'") ||
    [...source.path].some((char) => char.charCodeAt(0) < 32 || char.charCodeAt(0) === 127) ||
    (source.path && source.path.split('/').some((part) => !part || part === '.' || part === '..'))
  )
    throw new Error('Community source requires a relative package path');
  const selector = `github:${source.repository}@${source.revision}${source.path ? `//${source.path}` : ''}`;
  return shellToken(selector);
}

export function pluginCommands(
  plugin: RegistryPlugin,
  targets?: string | readonly string[],
  savedDependenciesReleased = SAVED_DEPENDENCIES_CLI_RELEASED,
  scope: InstallScope = 'user',
) {
  return installCommands(
    plugin.trust_state === 'conformant_unreviewed'
      ? communityAddSource(plugin)
      : plugin.install_source,
    plugin.name,
    targets,
    savedDependenciesReleased,
    scope,
  );
}

/** Shared command contract for Directory and the legacy curated install facade. */
export function installCommands(
  source: string,
  name: string,
  targets?: string | readonly string[],
  savedDependenciesReleased = SAVED_DEPENDENCIES_CLI_RELEASED,
  scope: InstallScope = 'user',
  invocation = CLI_INVOCATION,
) {
  if (scope !== 'user' && scope !== 'project') throw new Error('Unsupported installation scope');
  const values = (targets === undefined ? [] : Array.isArray(targets) ? targets : [targets])
    .map((target) => target.trim())
    .filter((target, index, all) => target && all.indexOf(target) === index);
  if (targets !== undefined && !values.length) throw new Error('At least one target is required');
  if (scope === 'project' && !values.length)
    throw new Error('Project installation requires explicit targets');
  const targetFlag = values.length ? ` --target ${values.join(',')}` : '';
  const scopeFlag = scope === 'project' ? ' --scope project' : '';
  const mode = dependencySaveMode(source, values);
  const owned = savedDependenciesReleased && mode === 'saved';
  return {
    saveMode: savedDependenciesReleased ? mode : undefined,
    add: `${invocation} add ${source}${targetFlag}${scopeFlag}${dependencyAddFlag(mode, savedDependenciesReleased)}`,
    update:
      owned && isPinnedGitSource(source)
        ? `${invocation} add ${source}${targetFlag}${scopeFlag}`
        : `${invocation} update ${name}${owned ? '' : targetFlag}${scopeFlag}`,
    repair: `${invocation} repair ${name}${targetFlag}${scopeFlag}`,
    switch: `${invocation} switch ${name} --to <distribution-id>${scopeFlag}`,
    remove: `${invocation} remove ${name}${owned ? '' : targetFlag}${scopeFlag}`,
  };
}
