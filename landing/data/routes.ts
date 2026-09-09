/** HTML families only. APIs, assets and the independently built docs are never expanded. */
export const routeFamilies = [
  { family: 'home', path: '/', indexable: true },
  { family: 'catalog', path: '/plugins/', indexable: true },
  { family: 'download', path: '/download/', indexable: true },
  { family: 'create', path: '/create-plugin/', indexable: false },
  { family: 'community', path: '/plugins/community/', indexable: false },
] as const;
export interface ProductRoute {
  family: (typeof routeFamilies)[number]['family'] | 'plugin' | 'agent';
  path: string;
  indexable: boolean;
}
export function productRoutes(
  pluginSlugs: readonly string[],
  clientSlugs: readonly string[],
): ProductRoute[] {
  const validate = (slugs: readonly string[], grammar: RegExp) => {
    if (new Set(slugs).size !== slugs.length || slugs.some((s) => !grammar.test(s))) {
      throw new Error('Invalid or duplicate route slug');
    }
  };
  // Directory product names permit dots; client URL slugs have a separate contract.
  validate(pluginSlugs, /^(?!.*(?:--|\.\.))[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$/);
  validate(clientSlugs, /^[a-z0-9]+(?:-[a-z0-9]+)*$/);
  if (pluginSlugs.includes('community')) throw new Error('Reserved plugin route: community');
  return [
    ...routeFamilies,
    ...pluginSlugs.map((slug) => ({
      family: 'plugin' as const,
      path: `/plugins/${slug}/`,
      indexable: true,
    })),
    ...clientSlugs.map((slug) => ({
      family: 'agent' as const,
      path: `/agents/${slug}/`,
      indexable: true,
    })),
  ];
}
