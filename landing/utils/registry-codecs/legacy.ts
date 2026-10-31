// Legacy preview decoder; never used for signed published snapshots.
import {
  PLUGIN_NAME, COMPONENTS, record, requiredString, stringArray, author, source, icon,
  clientIDs, legacyDelivery, legacyAuthentication, legacyTargetAuthentication, legacyEvidence,
} from './common.ts';
import type {
  ComponentID,
  DistributionKind,
  DistributionView,
  RegistryIndex,
  RegistryPlugin,
} from '../../types/registry';

export function parseLegacyIndex(input: Record<string, unknown>): RegistryIndex {
  if (input.schema_version !== 1 || !Array.isArray(input.plugins)) {
    throw new Error('registry index must have schema_version 1 and a plugins array');
  }
  const names = new Set<string>();
  const plugins = input.plugins.map((raw, index): RegistryPlugin => {
    const context = `registry plugin ${index}`;
    if (!record(raw)) throw new Error(`${context}: item must be an object`);
    const name = requiredString(raw, 'name', context);
    if (!PLUGIN_NAME.test(name)) throw new Error(`${context}: invalid name ${name}`);
    if (names.has(name)) throw new Error(`${context}: duplicate name ${name}`);
    names.add(name);
    if (typeof raw.built_in !== 'boolean')
      throw new Error(`${context}: built_in must be a boolean`);
    const parsedSource = source(raw.source, context);
    const installSource = requiredString(raw, 'install_source', context);
    const expectedSource = `${parsedSource.repository}@${parsedSource.revision}//${parsedSource.path}`;
    if (raw.built_in ? installSource !== name : installSource !== expectedSource) {
      throw new Error(
        `${context}: ${raw.built_in ? 'built-in install_source must equal its name' : 'external install_source must use source repository@40-char-sha//path'}`,
      );
    }
    if (!record(raw.client_support))
      throw new Error(`${context}: client_support must be an object`);
    const resolution = raw.client_support.resolution;
    if (resolution !== 'catalog' && resolution !== 'install_time')
      throw new Error(`${context}: client support resolution is invalid`);
    if (
      (raw.built_in && resolution !== 'catalog') ||
      (!raw.built_in && resolution !== 'install_time')
    ) {
      throw new Error(`${context}: client support resolution does not match source type`);
    }
    const compatibleClients = clientIDs(raw.client_support.clients, `${context} client support`);
    if (!compatibleClients.length)
      throw new Error(`${context}: at least one compatible client is required`);
    const components = stringArray(raw.components, 'components', context) as ComponentID[];
    if (components.some((component) => !COMPONENTS.has(component)))
      throw new Error(`${context}: unsupported component`);
    const evidence = legacyEvidence(raw.validation, context);
    const distributionID = raw.built_in ? `777genius/${name}` : parsedSource.repository;
    const kind: DistributionKind = raw.built_in ? 'community' : 'direct';
    const version = requiredString(raw, 'version', context);
    const authentication = legacyAuthentication(name);
    const distribution: DistributionView = {
      id: distributionID,
      kind,
      label: kind === 'community' ? 'Community package' : 'Direct source',
      publisher: author(raw.author, context).name,
      source: parsedSource,
      version,
      compatible_clients: compatibleClients,
      evidence,
      package_evidence: [],
      status: 'active',
      release_status: 'active',
      selectable: true,
      targets: compatibleClients.map((client) => ({
        client,
        authentication: legacyTargetAuthentication(authentication),
        delivery: legacyDelivery[client]!,
        scopes: ['user'],
      })),
      components,
      releases: [],
    };
    distribution.releases = [
      {
        release_sequence: 1,
        source: parsedSource,
        version,
        targets: distribution.targets,
        components,
        evidence,
        package_evidence: [],
        release_status: 'active',
        selectable: true,
        blocking_clients: [],
        materialized_clients: compatibleClients,
        meets_minimum_capabilities: true,
      },
    ];
    return {
      name,
      display_name: name,
      version,
      description: requiredString(raw, 'description', context),
      author: author(raw.author, context),
      license: requiredString(raw, 'license', context),
      categories: stringArray(raw.categories, 'categories', context),
      keywords: stringArray(raw.keywords, 'keywords', context),
      source: parsedSource,
      install_source: installSource,
      built_in: raw.built_in,
      installable: true,
      components,
      ...(raw.icon === undefined ? {} : { icon: icon(raw.icon, context) }),
      default_distribution: distributionID,
      declared_default_distribution: distributionID,
      distributions: [distribution],
      evidence,
      package_evidence: [],
      authentication,
      client_support: {
        resolution: raw.built_in ? 'directory' : 'install_time',
        clients: compatibleClients,
        delivery: legacyDelivery,
        scopes: Object.fromEntries(compatibleClients.map((client) => [client, ['user']])),
        app_bindings: {},
      },
    };
  });
  return { schema_version: 1, data_source: 'legacy_compatibility', plugins };
}

