import {
  SAVED_DEPENDENCIES_CLI_RELEASED,
  dependencySaveMode,
  dependencyAddFlag,
  isPinnedGitSource,
} from './savedDependencies.ts';
import type { RegistryPlugin } from '../types/registry';

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
) {
  const values = (targets === undefined ? [] : Array.isArray(targets) ? targets : [targets])
    .map((target) => target.trim())
    .filter((target, index, all) => target && all.indexOf(target) === index);
  if (targets !== undefined && !values.length) throw new Error('At least one target is required');
  const targetFlag = values.length ? ` --target ${values.join(',')}` : '';
  const addSource =
    plugin.trust_state === 'conformant_unreviewed'
      ? communityAddSource(plugin)
      : plugin.install_source;
  const mode = dependencySaveMode(addSource, values);
  const owned = savedDependenciesReleased && mode === 'saved';
  return {
    saveMode: savedDependenciesReleased ? mode : undefined,
    add: `${CLI_INVOCATION} add ${addSource}${targetFlag}${dependencyAddFlag(mode, savedDependenciesReleased)}`,
    update:
      owned && isPinnedGitSource(addSource)
        ? `${CLI_INVOCATION} add ${addSource}${targetFlag}`
        : `${CLI_INVOCATION} update ${plugin.name}${owned ? '' : targetFlag}`,
    repair: `${CLI_INVOCATION} repair ${plugin.name}${targetFlag}`,
    switch: `${CLI_INVOCATION} switch ${plugin.name} --to <distribution-id>`,
    remove: `${CLI_INVOCATION} remove ${plugin.name}${owned ? '' : targetFlag}`,
  };
}
