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
  lifecycleTargets?: readonly string[],
) {
  return installCommands(
    plugin.trust_state === 'conformant_unreviewed'
      ? communityAddSource(plugin)
      : plugin.install_source,
    plugin.name,
    targets,
    savedDependenciesReleased,
    scope,
    CLI_INVOCATION,
    lifecycleTargets,
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
  lifecycleTargets?: readonly string[],
) {
  if (scope !== 'user' && scope !== 'project') throw new Error('Unsupported installation scope');
  const values = (targets === undefined ? [] : Array.isArray(targets) ? targets : [targets])
    .map((target) => target.trim())
    .filter((target, index, all) => target && all.indexOf(target) === index);
  if (targets !== undefined && !values.length) throw new Error('At least one target is required');
  // Guided setup can defer a target without creating its installation receipt.
  // Narrow maintenance to real peers without changing the original add/save mode.
  const lifecycleValues = lifecycleTargets?.map((target) => target.trim())
    .filter((target, index, all) => target && all.indexOf(target) === index) ?? values;
  if (lifecycleTargets !== undefined &&
      (!lifecycleValues.length || lifecycleValues.some((target) => !values.includes(target))))
    throw new Error('Lifecycle targets must be a nonempty subset of installation targets');
  // In an interactive terminal, a targetless project add prompts for the
  // compatible installed agents. Non-interactive CLI calls require --target.
  const targetFlag = values.length ? ` --target ${values.join(',')}` : '';
  const lifecycleTargetFlag = lifecycleValues.length ? ` --target ${lifecycleValues.join(',')}` : '';
  const scopeFlag = scope === 'project' ? ' --scope project' : '';
  const mode = dependencySaveMode(source, values);
  const owned = savedDependenciesReleased && mode === 'saved';
  // Explicit maintenance targets retain the selected saved binding and use
  // the actual updater. Existing Directory maintenance keeps its own policy.
  const maintenanceTargetFlag = owned && lifecycleTargets === undefined ? '' : lifecycleTargetFlag;
  return {
    saveMode: savedDependenciesReleased ? mode : undefined,
    add: `${invocation} add ${source}${targetFlag}${scopeFlag}${dependencyAddFlag(mode, savedDependenciesReleased)}`,
    update:
      owned && isPinnedGitSource(source) && lifecycleTargets === undefined
        ? `${invocation} add ${source}${targetFlag}${scopeFlag}`
        : `${invocation} update ${name}${maintenanceTargetFlag}${scopeFlag}`,
    repair: `${invocation} repair ${name}${lifecycleTargetFlag}${scopeFlag}`,
    switch: `${invocation} switch ${name} --to <distribution-id>${scopeFlag}`,
    remove: `${invocation} remove ${name}${maintenanceTargetFlag}${scopeFlag}`,
  };
}
