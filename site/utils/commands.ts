import type { RegistryPlugin } from '../types/registry'

export function pluginCommands(plugin: RegistryPlugin, targets: string | readonly string[]) {
  const values = (Array.isArray(targets) ? targets : [targets])
    .map(target => target.trim())
    .filter((target, index, all) => target && all.indexOf(target) === index)
  if (!values.length) throw new Error('At least one target is required')
  const target = values.join(',')
  return {
    add: `npx universal-agent-plugins add ${plugin.install_source} --target ${target}`,
    update: `npx universal-agent-plugins update ${plugin.name} --target ${target}`,
    repair: `npx universal-agent-plugins repair ${plugin.name} --target ${target}`,
    switch: `npx universal-agent-plugins switch ${plugin.name} --to <distribution-id>`,
    remove: `npx universal-agent-plugins remove ${plugin.name} --target ${target}`,
  }
}
