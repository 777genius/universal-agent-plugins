// Stable catalog API: client eligibility, resolution and presentation.
import { parseLegacyIndex } from './registryDecodeLegacy.ts';
import { parseSnapshot } from './registryDecodeSigned.ts';
import type {
  RegistryDiagnostic,
  ClientEvidence,
  ClientID,
  DistributionReleaseView,
  DistributionResolution,
  DistributionKind,
  DistributionView,
  PackageEvidence,
  ReleaseTarget,
  TargetAuthentication,
  RegistryIndex,
  RegistryPlugin,
  InstallScope,
} from '../types/registry';

export function parseDirectoryData(
  input: unknown,
  mode?: 'published_snapshot' | 'review_preview',
): RegistryIndex {
  if (!record(input)) throw new Error('Directory data must be an object');
  if ('plugins' in input) {
    if (mode === 'published_snapshot')
      throw new Error(
        'published snapshot mode requires signed snapshot products and distributions',
      );
    return parseLegacyIndex(input);
  }
  return parseSnapshot(input, mode ?? 'review_preview');
}

export const parseRegistryIndex = parseDirectoryData;

export function isPinnedExternalSource(value: string): boolean {
  const match = /^([^@]+)@([a-f0-9]{40})\/\/(.+)$/.exec(value);
  return Boolean(match && REPOSITORY.test(match[1]!) && match[3]!.length > 0);
}

export function evidenceLabel(value: ClientEvidence | PackageEvidence): string {
  const level =
    value.level === 'oauth' ? 'OAuth' : value.level[0]!.toUpperCase() + value.level.slice(1);
  const outcome = value.outcome === 'not_tested' ? 'not tested' : value.outcome.replace('_', ' ');
  return `${level} ${outcome}`;
}

export function validationLabel(
  view: Pick<RegistryPlugin, 'evidence' | 'package_evidence'>,
): string {
  const passed = view.evidence.filter((item) => item.outcome === 'passed');
  const hasEnvironment = (item: ClientEvidence) =>
    Boolean(item.client_version && item.os && item.architecture && item.tested_at);
  if (passed.some((item) => item.level === 'oauth' && hasEnvironment(item))) return 'OAuth tested';
  if (passed.some((item) => item.level === 'runtime' && hasEnvironment(item)))
    return 'Runtime tested';
  if (passed.some((item) => item.level === 'discovery' && hasEnvironment(item)))
    return 'Discovery tested';
  if (passed.some((item) => item.level === 'materialization' && hasEnvironment(item)))
    return 'Materialization tested';
  const schema = view.package_evidence[0];
  return schema ? evidenceLabel(schema) : 'Package reviewed';
}

export function defaultDistribution(plugin: RegistryPlugin): DistributionView {
  const distribution = plugin.distributions.find((item) => item.id === plugin.default_distribution);
  if (!distribution) throw new Error(`${plugin.name}: default distribution is unavailable`);
  return distribution;
}

function supportsInstallScope(target: ReleaseTarget | undefined, scope: InstallScope): boolean {
  // A remote app connection has no project filesystem installation. Missing
  // scope facts cannot authorize a project command on older projections.
  return Boolean(target?.scopes?.includes(scope) && (scope !== 'project' || !target.app_binding));
}

function eligibleRelease(
  distribution: DistributionView,
  targets: ReadonlySet<ClientID>,
  scope?: InstallScope,
): { release?: DistributionReleaseView; reason?: string; diagnostic?: RegistryDiagnostic } {
  if (distribution.status !== 'active')
    return {
      reason: `distribution is ${distribution.status}`,
      diagnostic: { code: 'distributionStatus', params: { status: distribution.status } },
    };
  const reasons: string[] = [];
  const diagnostics: RegistryDiagnostic[] = [];
  for (const release of [...distribution.releases].sort(
    (a, b) => b.release_sequence - a.release_sequence,
  )) {
    const supported = new Set(release.targets.map((target) => target.client));
    if (release.release_status !== 'active') {
      reasons.push(`release ${release.release_sequence} is ${release.release_status}`);
      diagnostics.push({
        code: 'releaseStatus',
        params: { sequence: release.release_sequence, status: release.release_status },
      });
    } else if (!release.meets_minimum_capabilities) {
      reasons.push(`release ${release.release_sequence} misses required components`);
      diagnostics.push({
        code: 'missingComponents',
        params: { sequence: release.release_sequence },
      });
    } else if ([...targets].some((target) => !supported.has(target))) {
      const missing = [...targets].filter((target) => !supported.has(target)).sort();
      reasons.push(`release ${release.release_sequence} does not support ${missing.join(',')}`);
      diagnostics.push({
        code: 'unsupportedTargets',
        params: { sequence: release.release_sequence, targets: missing.join(',') },
      });
    } else {
      const missingScope = scope
        ? [...targets].filter(
            (client) =>
              !supportsInstallScope(
                release.targets.find((target) => target.client === client),
                scope,
              ),
          )
        : [];
      if (scope && missingScope.length) {
        reasons.push(
          `release ${release.release_sequence} does not support ${scope} scope for ${missingScope.sort().join(',')}`,
        );
        diagnostics.push({
          code: 'unsupportedScope',
          params: { sequence: release.release_sequence, scope, targets: missingScope.join(',') },
        });
        continue;
      }
      const failures = [...targets]
        .filter((target) => release.blocking_clients.includes(target))
        .sort();
      if (failures.length) {
        reasons.push(
          `release ${release.release_sequence} has blocking trusted failure for ${failures.join(',')}`,
        );
        diagnostics.push({
          code: 'blockingFailure',
          params: { sequence: release.release_sequence, targets: failures.join(',') },
        });
        continue;
      }
      if (distribution.kind === 'upstream') {
        const missing = [...targets]
          .filter((target) => !release.materialized_clients.includes(target))
          .sort();
        if (missing.length) {
          reasons.push(
            `release ${release.release_sequence} lacks current positive package compatibility evidence (passed materialization) for ${missing.join(',')}`,
          );
          diagnostics.push({
            code: 'missingEvidence',
            params: { sequence: release.release_sequence, targets: missing.join(',') },
          });
          continue;
        }
      }
      return { release };
    }
  }
  return {
    reason: reasons.join('; ') || 'no releases',
    diagnostic: diagnostics.length
      ? { code: 'reasons', reasons: diagnostics }
      : { code: 'noReleases' },
  };
}

export function resolveDistribution(
  plugin: RegistryPlugin,
  targets: readonly ClientID[],
  scope?: InstallScope,
): DistributionResolution {
  if (!targets.length || new Set(targets).size !== targets.length)
    return {
      unavailable_reason: 'targets must be unique supported client IDs',
      unavailable_diagnostic: { code: 'invalidTargets' },
    };
  const selectedTargets = new Set(targets);
  const resolved = (distribution: DistributionView): DistributionView | undefined => {
    const release = eligibleRelease(distribution, selectedTargets, scope).release;
    return release
      ? {
          ...distribution,
          source: release.source,
          release_sequence: release.release_sequence,
          version: release.version,
          compatible_clients: release.targets
            .filter((target) => !release.blocking_clients.includes(target.client))
            .map((target) => target.client),
          evidence: release.evidence,
          package_evidence: release.package_evidence,
          release_status: release.release_status,
          selectable: true,
          targets: release.targets,
          components: release.components,
        }
      : undefined;
  };
  const declared = plugin.distributions.find(
    (distribution) => distribution.id === plugin.declared_default_distribution,
  );
  if (!declared) throw new Error(`${plugin.name}: declared default distribution is unavailable`);
  const selectedDefault = resolved(declared);
  if (selectedDefault) return { distribution: selectedDefault };
  const defaultEligibility = eligibleRelease(declared, selectedTargets, scope);
  const defaultReason = defaultEligibility.reason ?? 'no releases';
  const defaultDiagnostic = defaultEligibility.diagnostic ?? { code: 'noReleases' as const };
  const priority: Record<DistributionKind, number> = {
    upstream: 0,
    community_bridge: 1,
    community: 2,
    direct: 3,
  };
  const alternatives = plugin.distributions
    .filter((item) => item.id !== declared.id)
    .sort((a, b) => priority[a.kind] - priority[b.kind] || a.id.localeCompare(b.id));
  const ineligibleReasons = [{ distribution_id: declared.id, reason: defaultReason }];
  for (const distribution of alternatives) {
    const release = resolved(distribution);
    if (release) {
      const fallbackReason = `declared default ${declared.id} was ineligible: ${defaultReason}`;
      return {
        distribution: { ...release, fallback_reason: fallbackReason },
        fallback_reason: fallbackReason,
        fallback_diagnostic: {
          code: 'defaultIneligible',
          params: { distribution: declared.id },
          reasons: [defaultDiagnostic],
        },
      };
    }
    ineligibleReasons.push({
      distribution_id: distribution.id,
      reason: eligibleRelease(distribution, selectedTargets, scope).reason ?? 'no releases',
    });
  }
  const targetList = [...selectedTargets].sort().join(',');
  return {
    unavailable_reason: `${plugin.name}: no eligible distribution supports the complete target set ${targetList}; ${ineligibleReasons.map((item) => `${item.distribution_id}: ${item.reason}`).join('; ')}`,
    ineligible_reasons: ineligibleReasons,
    unavailable_diagnostic: {
      code: 'noEligibleDistribution',
      params: { name: plugin.name, targets: targetList },
      reasons: [declared, ...alternatives].map((distribution) => ({
        code: 'distributionReason',
        params: { distribution: distribution.id },
        reasons: [
          eligibleRelease(distribution, selectedTargets, scope).diagnostic ?? {
            code: 'noReleases',
          },
        ],
      })),
    },
  };
}

export function eligibleTargetPolicies(
  kind: DistributionKind,
  release: Pick<DistributionReleaseView, 'targets' | 'blocking_clients' | 'materialized_clients'>,
): ReleaseTarget[] {
  return release.targets.filter(
    (target) =>
      !release.blocking_clients.includes(target.client) &&
      (kind !== 'upstream' || release.materialized_clients.includes(target.client)),
  );
}

/** Every policy a no-`--target` install can resolve: the signed resolver walks
 * distributions in order and installs into whichever agents it detects, so any
 * individually eligible (release, target) pair is reachable. */
export function reachableTargetPolicies(
  distributions: readonly Pick<DistributionView, 'kind' | 'status' | 'releases'>[],
  scope?: InstallScope,
): ReleaseTarget[] {
  const policies: ReleaseTarget[] = [];
  for (const distribution of distributions) {
    if (distribution.status !== 'active') continue;
    for (const release of distribution.releases) {
      if (release.release_status !== 'active' || !release.meets_minimum_capabilities) continue;
      for (const target of eligibleTargetPolicies(distribution.kind, release)) {
        if (scope && !supportsInstallScope(target, scope)) continue;
        if (
          !policies.some(
            (item) =>
              item.client === target.client && item.authentication === target.authentication,
          )
        )
          policies.push(target);
      }
    }
  }
  return policies;
}

export function expectedDistribution(
  plugin: RegistryPlugin,
  targets: readonly ClientID[],
): DistributionView | undefined {
  return resolveDistribution(plugin, targets).distribution;
}

export function directoryIsExpired(
  registry: Pick<RegistryIndex, 'data_source' | 'expires_at'>,
  now = Date.now(),
): boolean {
  if (registry.data_source !== 'published_snapshot' || !registry.expires_at) return false;
  const expiry = parseRFC3339Instant(registry.expires_at, 'Directory expires_at');
  return now >= expiry.epochSeconds * 1000 + Number(`0.${expiry.fraction || '0'}`) * 1000;
}

export function deliveryLabel(delivery: ReleaseTarget['delivery']): string {
  if (delivery === 'managed') return 'Managed install';
  if (delivery === 'prepared') return 'Prepared; client import remains';
  return 'Manual activation required';
}

export function targetAuthenticationLabel(authentication: TargetAuthentication): string {
  if (authentication === 'not_required') return 'No account required';
  if (authentication === 'required') return 'Authentication required';
  return 'Check package requirements';
}

export function authenticationState(
  distribution: Pick<DistributionView, 'targets'> | undefined,
  selectedClients: readonly ClientID[],
  legacyAuthentication?: RegistryPlugin['authentication'],
): 'not_required' | 'oauth' | 'client_managed' | 'unknown' | 'varies' | 'required' {
  // The flat legacy catalog has only a product-wide value. Keep its established
  // labels, but never infer OAuth from a signed target's generic `required`.
  if (legacyAuthentication === 'none') return 'not_required';
  if (legacyAuthentication === 'oauth') return 'oauth';
  if (legacyAuthentication === 'client_managed') return 'client_managed';

  if (!distribution || !selectedClients.length) return 'unknown';
  const selected = distribution.targets.filter((target) => selectedClients.includes(target.client));
  if (selected.length !== selectedClients.length) return 'unknown';
  const values = new Set(selected.map((target) => target.authentication));
  if (values.size > 1) return 'varies';
  return selected[0]!.authentication;
}

export function authenticationLabel(
  distribution: Pick<DistributionView, 'targets'> | undefined,
  selectedClients: readonly ClientID[],
  legacyAuthentication?: RegistryPlugin['authentication'],
): string {
  const labels = {
    not_required: 'No account required',
    oauth: 'OAuth required',
    client_managed: 'Client-managed authentication',
    unknown: 'Check package requirements',
    varies: 'Authentication varies',
    required: 'Authentication required',
  };
  return labels[authenticationState(distribution, selectedClients, legacyAuthentication)];
}

export function githubSourceUrl(
  plugin: RegistryPlugin,
  distribution = defaultDistribution(plugin),
): string {
  if (distribution.source.revision === null)
    return `https://github.com/${distribution.source.repository}`;
  const path = distribution.source.path.split('/').map(encodeURIComponent).join('/');
  return `https://github.com/${distribution.source.repository}/tree/${distribution.source.revision}/${path}`;
}

export interface VendorInstallSource {
  url: string;
  command: string;
}

// Products that install through their own vendor installer instead of the
// universal-agent-plugins CLI. The command is the vendor's own published one;
// the UI must present it as vendor-supplied, never as vouched for by UAP.
const vendorInstallSources: Record<string, VendorInstallSource> = {
  'agent-notifications': {
    url: 'https://agent-notifications.com/#install',
    command: '(set -o pipefail; curl -fsSL https://agent-notifications.com/install.sh | bash)',
  },
};

export function vendorInstallSource(plugin: RegistryPlugin): VendorInstallSource | undefined {
  return vendorInstallSources[plugin.name];
}

export function mirroredIconPath(plugin: RegistryPlugin): string | undefined {
  if (!plugin.built_in) return undefined;
  // Local presentation assets are owned explicitly; package metadata cannot
  // select arbitrary files or make an external discovery entry load a logo.
  const icons: Record<string, string> = {
    'agent-code-navigator': 'agent-code-navigator',
    'agent-notifications': 'agent-notifications',
    atlassian: 'atlassian',
    'chrome-devtools': 'googlechrome',
    cloudflare: 'cloudflare',
    'cloudflare-bindings': 'cloudflare',
    'cloudflare-docs': 'cloudflare',
    'cloudflare-observability': 'cloudflare',
    'cloudflare-radar': 'cloudflare',
    context7: 'context7',
    'docker-hub': 'docker',
    figma: 'figma',
    firebase: 'firebase',
    firecrawl: 'firecrawl',
    github: 'github',
    gitlab: 'gitlab',
    greptile: 'greptile',
    heroku: 'heroku',
    'hubspot-crm': 'hubspot',
    'hubspot-developer': 'hubspot',
    linear: 'linear',
    'marketing-kit': 'marketing-kit',
    neon: 'neon',
    notion: 'notion',
    playwright: 'playwright',
    sentry: 'sentry',
    statsig: 'statsig',
    stripe: 'stripe',
    supabase: 'supabase',
    tinyfish: 'tinyfish',
    vercel: 'vercel',
  };
  const icon = Object.hasOwn(icons, plugin.name) ? icons[plugin.name] : undefined;
  return icon ? `plugin-icons/${icon}.svg` : undefined;
}

