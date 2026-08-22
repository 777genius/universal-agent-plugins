import type { RegistryPlugin } from '../types/registry'

export interface CatalogFilters {
  query?: string
  category?: string
  component?: RegistryPlugin['components'][number]
  source?: 'all' | 'upstream' | 'community_bridge' | 'community' | 'direct'
}

export function filterPlugins(plugins: RegistryPlugin[], filters: CatalogFilters): RegistryPlugin[] {
  const query = filters.query?.trim().toLocaleLowerCase() ?? ''
  return plugins.filter((plugin) => {
    const searchable = [
      plugin.name,
      plugin.display_name,
      plugin.description,
      plugin.author.name,
      ...plugin.categories,
      ...plugin.keywords,
      ...plugin.components,
    ].join(' ').toLocaleLowerCase()
    return (!query || searchable.includes(query))
      && (!filters.category || plugin.categories.includes(filters.category))
      && (!filters.component || plugin.components.includes(filters.component))
      && (!filters.source || filters.source === 'all'
        || plugin.distributions.find(item => item.id === plugin.default_distribution)?.kind === filters.source)
  })
}

export function availableFilters(plugins: RegistryPlugin[]) {
  const categories = [...new Set(plugins.flatMap(plugin => plugin.categories))].sort()
  const components = [...new Set(plugins.flatMap(plugin => plugin.components))].sort()
  return { categories, components }
}
