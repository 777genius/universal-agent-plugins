// Strict signed directory snapshot projection.
import { catalogCategories } from '../catalogCategories.ts';
import {
  PLUGIN_NAME, COMPONENTS, KINDS, record, requiredString, optionalString, stringArray,
  digestValue, safeSequence, parseRFC3339Instant, compareInstants, author, source, icon,
} from './common.ts';
import { releaseTargets, evidenceFromSnapshot, eligibleTargetPolicies } from './evidence.ts';
import type {
  ComponentID,
  DistributionReleaseView,
  DistributionKind,
  DistributionView,
  RegistryIndex,
  RegistryPlugin,
} from '../../types/registry';

export function parseSnapshot(
  input: Record<string, unknown>,
  mode: 'published_snapshot' | 'review_preview',
): RegistryIndex {
  const isSigned = input.snapshot_schema_version === 1;
  if (
    (!isSigned && input.schema_version !== 1) ||
    !Array.isArray(input.products) ||
    !Array.isArray(input.distributions)
  ) {
    throw new Error('Directory data must have schema version 1, products, and distributions');
  }
  const snapshotSequence = isSigned ? input.sequence : input.snapshot_sequence;
  if (
    mode === 'published_snapshot' &&
    (!isSigned || typeof input.generated_at !== 'string' || typeof input.expires_at !== 'string')
  ) {
    throw new Error(
      'published snapshot requires one signed sequence, generated_at, and expires_at',
    );
  }
  if (isSigned) safeSequence(snapshotSequence, 'Directory snapshot sequence');
  else if (snapshotSequence !== undefined)
    safeSequence(snapshotSequence, 'Directory preview snapshot sequence');

  // Reject unsafe identities before building keys or comparing them. JSON.parse
  // aliases 9007199254740992 and 9007199254740993 in JavaScript, so filtering
  // or stringifying first would make distinct signed identities collide.
  for (const [field, records] of [
    ['evidence', input.evidence],
    ['verification_summaries', input.verification_summaries],
    ['current_verification', input.current_verification],
    ['revocations', input.revocations],
  ] as const) {
    if (!Array.isArray(records)) continue;
    for (const [index, item] of records.entries()) {
      if (record(item) && item.release_sequence !== undefined) {
        safeSequence(item.release_sequence, `Directory ${field}[${index}] release sequence`);
      }
    }
  }
  let generatedAt: string | undefined;
  let expiresAt: string | undefined;
  if (input.generated_at !== undefined || input.expires_at !== undefined || isSigned) {
    const generated = parseRFC3339Instant(input.generated_at, 'Directory generated_at');
    const expires = parseRFC3339Instant(input.expires_at, 'Directory expires_at');
    if (compareInstants(expires, generated) <= 0)
      throw new Error('Directory expires_at must be after generated_at');
    generatedAt = input.generated_at as string;
    expiresAt = input.expires_at as string;
  }
  const distributionRecords = new Map<string, Record<string, unknown>>();
  for (const raw of input.distributions) {
    if (!record(raw)) throw new Error('Directory distribution must be an object');
    const id = requiredString(raw, 'id', 'Directory distribution');
    if (distributionRecords.has(id)) throw new Error(`duplicate distribution ${id}`);
    distributionRecords.set(id, raw);
  }
  const revoked = new Set(
    (Array.isArray(input.revocations) ? input.revocations : [])
      .filter(record)
      .map((item) => `${String(item.distribution_id)}:${String(item.release_sequence)}`),
  );
  const seen = new Set<string>();
  const plugins = input.products.map((raw, index): RegistryPlugin => {
    const context = `Directory product ${index}`;
    if (!record(raw)) throw new Error(`${context} must be an object`);
    const name = requiredString(raw, 'id', context);
    if (!PLUGIN_NAME.test(name) || seen.has(name))
      throw new Error(`${context}: invalid or duplicate id ${name}`);
    seen.add(name);
    const defaultID = requiredString(raw, 'default_distribution', context);
    const listed = stringArray(raw.distributions, 'distributions', context);
    if (!listed.includes(defaultID))
      throw new Error(`${context}: default distribution is not listed`);
    if (!record(raw.minimum_capabilities))
      throw new Error(`${context}: minimum_capabilities must be an object`);
    const minimumCapabilities = raw.minimum_capabilities;
    const requiredComponents = new Set<ComponentID>(
      [...COMPONENTS].filter((component) => minimumCapabilities[component] === 'required'),
    );
    const distributions = listed.map((id): DistributionView => {
      const item = distributionRecords.get(id);
      if (!item || item.product_id !== name)
        throw new Error(`${context}: missing or mismatched distribution ${id}`);
      const kind = requiredString(item, 'kind', `distribution ${id}`) as DistributionKind;
      if (!KINDS.has(kind)) throw new Error(`distribution ${id}: unsupported kind`);
      if (!Array.isArray(item.releases) || !item.releases.length)
        throw new Error(`distribution ${id}: releases are required`);
      const status = requiredString(item, 'status', `distribution ${id}`);
      if (!['candidate', 'active', 'suspended'].includes(status))
        throw new Error(`distribution ${id}: unsupported status`);
      const policies = Array.isArray(item.release_policies)
        ? item.release_policies.filter(record)
        : [];
      for (const [policyIndex, policy] of policies.entries()) {
        safeSequence(
          policy.release_sequence,
          `distribution ${id} policy ${policyIndex} release sequence`,
        );
      }
      const releases = item.releases.filter(record);
      for (const [releaseIndex, release] of releases.entries()) {
        safeSequence(release.sequence, `distribution ${id} release ${releaseIndex} sequence`);
      }
      releases.sort((a, b) => Number(b.sequence) - Number(a.sequence));
      const releaseSequences = releases.map((release) => release.sequence);
      if (new Set(releaseSequences).size !== releaseSequences.length)
        throw new Error(`distribution ${id}: release sequences must be unique`);
      const releaseViews = releases.map((release): DistributionReleaseView => {
        const releaseSequence = safeSequence(
          release.sequence,
          `distribution ${id} release sequence`,
        );
        const packageSource = source(
          {
            ...(record(release.package_source) ? release.package_source : {}),
            manifest_digest: release.manifest_digest,
            tree_digest: release.tree_digest,
          },
          `distribution ${id}`,
          mode === 'review_preview',
        );
        const policy = policies.find((candidate) => candidate.release_sequence === releaseSequence);
        if (!policy)
          throw new Error(
            `distribution ${id}: release ${String(releaseSequence)} has no signed policy`,
          );
        const targets = releaseTargets(
          policy.targets,
          `distribution ${id} release ${String(releaseSequence)}`,
        );
        const policyStatus = requiredString(policy, 'status', `distribution ${id} release policy`);
        if (!['active', 'superseded', 'revoked'].includes(policyStatus))
          throw new Error(`distribution ${id}: unsupported release status`);
        const releaseStatus = revoked.has(`${id}:${String(releaseSequence)}`)
          ? 'revoked'
          : (policyStatus as DistributionView['release_status']);
        if (
          !Array.isArray(policy.current_evidence) ||
          policy.current_evidence.some((value) => typeof value !== 'string')
        )
          throw new Error(`distribution ${id}: current_evidence must be an array of evidence IDs`);
        const treeDigest = digestValue(release.tree_digest, `distribution ${id} tree digest`);
        const evidence = evidenceFromSnapshot(
          input.evidence ?? input.verification_summaries ?? input.current_verification,
          id,
          releaseSequence,
          treeDigest,
          policy.current_evidence as string[],
          mode === 'published_snapshot',
        );
        const components = stringArray(
          release.components ?? [],
          'components',
          `distribution ${id}`,
        ) as ComponentID[];
        if (components.some((component) => !COMPONENTS.has(component)))
          throw new Error(`distribution ${id}: unsupported component`);
        const blockingClients = [
          ...new Set(
            evidence.client
              .filter(
                (observation) =>
                  observation.trusted_for_eligibility &&
                  observation.outcome === 'failed' &&
                  ['materialization', 'discovery', 'runtime'].includes(observation.level),
              )
              .map((observation) => observation.client),
          ),
        ];
        const materializedClients = [
          ...new Set(
            evidence.client
              .filter(
                (observation) =>
                  observation.trusted_for_eligibility &&
                  observation.level === 'materialization' &&
                  observation.outcome === 'passed',
              )
              .map((observation) => observation.client),
          ),
        ];
        const meetsMinimumCapabilities = [...requiredComponents].every((component) =>
          components.includes(component),
        );
        return {
          release_sequence: releaseSequence,
          source: packageSource,
          version:
            optionalString(release, 'version') ??
            optionalString(release, 'package_version') ??
            'unversioned',
          targets,
          components,
          evidence: evidence.client,
          package_evidence: evidence.package,
          release_status: releaseStatus as DistributionView['release_status'],
          selectable: status === 'active' && releaseStatus === 'active' && meetsMinimumCapabilities,
          blocking_clients: blockingClients,
          materialized_clients: materializedClients,
          meets_minimum_capabilities: meetsMinimumCapabilities,
        };
      });
      const eligibleTargets = (release: DistributionReleaseView) =>
        release.selectable ? eligibleTargetPolicies(kind, release) : [];
      const selectedRelease =
        releaseViews.find((release) => eligibleTargets(release).length > 0) ?? releaseViews[0]!;
      const compatible = [
        ...new Set(
          releaseViews.flatMap((release) =>
            eligibleTargets(release).map((target) => target.client),
          ),
        ),
      ];
      return {
        id,
        kind,
        label:
          kind === 'upstream'
            ? 'Upstream package'
            : kind === 'community_bridge'
              ? 'Community bridge'
              : 'Community package',
        publisher:
          optionalString(item, 'publisher') ??
          optionalString(item, 'packager') ??
          id.split('/')[0]!,
        source: selectedRelease.source,
        release_sequence: selectedRelease.release_sequence,
        version: selectedRelease.version,
        compatible_clients: compatible,
        evidence: selectedRelease.evidence,
        package_evidence: selectedRelease.package_evidence,
        status: status as DistributionView['status'],
        release_status: selectedRelease.release_status,
        selectable: compatible.length > 0,
        targets: selectedRelease.targets,
        components: selectedRelease.components,
        releases: releaseViews,
      };
    });
    const declared = distributions.find((item) => item.id === defaultID)!;
    const priority: Record<DistributionKind, number> = {
      upstream: 0,
      community_bridge: 1,
      community: 2,
      direct: 3,
    };
    const selected = declared.selectable
      ? declared
      : (distributions
          .filter((item) => item.selectable)
          .sort((a, b) => priority[a.kind] - priority[b.kind] || a.id.localeCompare(b.id))[0] ??
        declared);
    const components = selected.components;
    const productAuthor = record(raw.author)
      ? author(raw.author, context)
      : { name: selected.publisher };
    return {
      name,
      display_name: optionalString(raw, 'display_name') ?? name,
      version: selected.version,
      description: requiredString(raw, 'description', context),
      author: productAuthor,
      license: optionalString(raw, 'license') ?? 'See source',
      categories: catalogCategories(
        selected.source,
        stringArray(raw.categories ?? [], 'categories', context),
      ),
      keywords: stringArray(raw.keywords ?? [], 'keywords', context),
      source: selected.source,
      install_source: name,
      built_in: true,
      installable: selected.selectable,
      components,
      ...(raw.icon === undefined
        ? {}
        : {
            icon: icon(
              record(raw.icon) && raw.icon.sha256 === undefined
                ? { path: raw.icon.path, sha256: raw.icon.digest }
                : raw.icon,
              context,
            ),
          }),
      default_distribution: selected.id,
      declared_default_distribution: defaultID,
      ...(selected.id === defaultID
        ? {}
        : {
            default_fallback_reason:
              declared.status !== 'active'
                ? `Declared default is ${declared.status}`
                : `Declared default release is ${declared.release_status}`,
          }),
      distributions,
      evidence: selected.evidence,
      package_evidence: selected.package_evidence,
      authentication:
        selected.targets.length > 0 &&
        selected.targets.every((target) => target.authentication === 'not_required')
          ? 'none'
          : 'unknown',
      client_support: {
        resolution: 'directory',
        clients: [
          ...new Set(
            distributions
              .filter((item) => item.selectable)
              .flatMap((item) => item.compatible_clients),
          ),
        ],
        delivery: Object.fromEntries(
          selected.targets.map((target) => [target.client, target.delivery]),
        ),
        scopes: Object.fromEntries(
          selected.targets.map((target) => [target.client, target.scopes]),
        ),
        app_bindings: Object.fromEntries(
          selected.targets
            .filter((target) => target.app_binding)
            .map((target) => [target.client, target.app_binding!]),
        ),
      },
    };
  });
  return {
    schema_version: 1,
    data_source: mode,
    ...(typeof snapshotSequence === 'number' ? { snapshot_sequence: snapshotSequence } : {}),
    ...(generatedAt ? { generated_at: generatedAt } : {}),
    ...(expiresAt ? { expires_at: expiresAt } : {}),
    plugins,
  };
}

