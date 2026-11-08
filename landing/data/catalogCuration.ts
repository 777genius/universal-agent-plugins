import type { RegistryPlugin } from '../types/registry.ts';
import type { SkillListing } from '../types/skillsCatalog.ts';

/** Presentation-only suggestions. Never use these IDs as a source of install authority. */
export const recommendedPluginNames = [
  'github',
  'playwright',
  'context7',
  'chrome-devtools',
  'figma',
  'notion',
  'linear',
  'firecrawl',
  'sentry',
  'supabase',
  'vercel',
  'neon',
  'stripe',
  'firebase',
  'docker-hub',
  'gitlab',
  'cloudflare-docs',
] as const;

const pluginRank = new Map<string, number>(
  recommendedPluginNames.map((name, index) => [name, index] as const),
);

export function recommendedPluginRank(name: string): number {
  return pluginRank.get(name) ?? Number.MAX_SAFE_INTEGER;
}

/** Benefit-led copy for reviewed products; raw signed metadata remains authoritative. */
const reviewedSummaries: Readonly<Record<string, string>> = {
  'agent-code-navigator': 'Map a codebase with focused search, language-server navigation, and architecture guidance.',
  'agent-notifications': 'Get desktop alerts when an AI coding agent finishes a task, needs input, or fails.',
  'atlassian': 'Bring Jira issues and Confluence knowledge into planning and research with your agent.',
  'chrome-devtools': 'Inspect browser pages, diagnose errors, and investigate performance from your coding agent.',
  'cloudflare': 'Work with Cloudflare APIs and platform guidance while building and deploying applications.',
  'cloudflare-bindings': 'Explore Cloudflare Workers bindings for storage, AI, and compute services.',
  'cloudflare-docs': 'Look up current Cloudflare documentation while building and debugging Workers apps.',
  'cloudflare-observability': 'Investigate Cloudflare logs and analytics to understand production issues.',
  'cloudflare-radar': 'Explore internet traffic trends and network telemetry through Cloudflare Radar.',
  'context7': 'Find current, version-specific library documentation and code examples for your agent.',
  'docker-hub': 'Explore Docker Hub images and repositories without leaving your development workflow.',
  'figma': 'Bring Figma design context into implementation and interface review workflows.',
  'firebase': 'Work with Firestore, authentication, hosting, and other Firebase backend services.',
  'firecrawl': 'Search public websites and extract usable page content for research and analysis.',
  'github': 'Explore repositories, review pull requests, and work with issues from your coding agent.',
  'gitlab': 'Inspect GitLab projects, merge requests, and issues without switching tools.',
  'greptile': 'Search repository code and investigate how a project works with Greptile.',
  'heroku': 'Inspect Heroku apps, add-ons, logs, and Postgres during development and debugging.',
  'hubspot-crm': 'Read HubSpot CRM records to add customer context to sales and marketing work.',
  'hubspot-developer': 'Build and troubleshoot HubSpot projects, CMS assets, and app workflows using developer tools.',
  'linear': 'Find and manage Linear issues and project work from your agent.',
  'marketing-kit': 'Draft a campaign plan and landing-page review from Notion and HubSpot context.',
  'neon': 'Work with Neon Postgres projects and database branches during development.',
  'notion': 'Find workspace knowledge, read documents, and update Notion pages with your permission.',
  'playwright': 'Automate browser interactions, test user journeys, and inspect web pages.',
  'sentry': 'Investigate Sentry errors and issues while debugging an application.',
  'statsig': 'Explore Statsig experiments, feature flags, and configuration from your agent.',
  'stripe': 'Work with Stripe billing, payments, and customer information through an authorized connection.',
  'supabase': 'Work with Supabase projects, databases, and backend services during development.',
  'tinyfish': 'Collect information from live websites with hosted browser automation.',
  'vercel': 'Inspect Vercel projects, deployments, and logs while shipping and debugging applications.',
};

export function pluginBenefit(
  plugin: Pick<RegistryPlugin, 'name' | 'description' | 'trust_state'>,
): string {
  if (plugin.trust_state === 'conformant_unreviewed') return plugin.description;
  return reviewedSummaries[plugin.name] ?? plugin.description;
}

/** Exact upstream identity, not a skills.sh ranking slot, guessed folder or security verdict. */
export const recommendedSkills: ReadonlyArray<{ repository: string; slug: string }> = [
  { repository: 'vercel-labs/agent-browser', slug: 'agent-browser' },
  { repository: 'anthropics/skills', slug: 'frontend-design' },
  { repository: 'vercel-labs/skills', slug: 'find-skills' },
  { repository: 'mattpocock/skills', slug: 'tdd' },
  { repository: 'vercel-labs/agent-skills', slug: 'vercel-react-best-practices' },
  { repository: 'vercel-labs/agent-skills', slug: 'web-design-guidelines' },
];

/** Avoid assigning editorial descriptions to another publisher's similarly named Skill. */
export function recommendedSkillKey(
  skill: Pick<SkillListing, 'id' | 'skillNameHint' | 'source'>,
): string | undefined {
  if (skill.source.kind !== 'github') return undefined;
  return recommendedSkills.find(
    (item) =>
      item.repository === skill.source.repository &&
      item.slug === skill.skillNameHint &&
      skill.id === `skills-sh:${item.repository}:${item.slug}`,
  )?.slug;
}
