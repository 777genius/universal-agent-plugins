import type { RegistryPlugin } from '../types/registry';

const CLI_INVOCATION = 'npx universal-agent-plugins';

export function pluginCommands(plugin: RegistryPlugin, targets?: string | readonly string[]) {
  const values = (targets === undefined ? [] : Array.isArray(targets) ? targets : [targets])
    .map((target) => target.trim())
    .filter((target, index, all) => target && all.indexOf(target) === index);
  if (targets !== undefined && !values.length) throw new Error('At least one target is required');
  const targetFlag = values.length ? ` --target ${values.join(',')}` : '';
  return {
    add: `${CLI_INVOCATION} add ${plugin.install_source}${targetFlag}`,
    update: `${CLI_INVOCATION} update ${plugin.name}${targetFlag}`,
    repair: `${CLI_INVOCATION} repair ${plugin.name}${targetFlag}`,
    switch: `${CLI_INVOCATION} switch ${plugin.name} --to <distribution-id>`,
    remove: `${CLI_INVOCATION} remove ${plugin.name}${targetFlag}`,
  };
}
