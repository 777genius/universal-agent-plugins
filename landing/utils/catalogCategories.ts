/** Editorial task categories; these do not confer review or runtime evidence. */
const marketingSources = new Set([
  '777genius/universal-agent-plugins/examples/plugins/marketing-kit',
  '777genius/universal-agent-plugins/plugins/hubspot-crm',
  'wondelai/skills/plugins/marketing-cro',
  'sickn33/agentic-awesome-skills/plugins/agentic-bundle-aas-marketing-seo-growth',
]);

export function catalogCategories(
  source: { repository: string; path: string },
  declared: string[] = [],
): string[] {
  const identity = `${source.repository.toLowerCase()}/${source.path}`;
  return marketingSources.has(identity)
    ? [...new Set([...declared, 'marketing'])]
    : declared;
}
