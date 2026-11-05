// Trusted release target and evidence decoding.
import {
  REPOSITORY, REVISION, DIGEST, CLIENTS, EVIDENCE_WORKFLOW, EVIDENCE_SOURCE_REF,
  record, requiredString, optionalString, stringArray,
} from './common.ts';
import type {
  ClientEvidence,
  ClientID,
  DistributionKind,
  DistributionReleaseView,
  EvidenceLevel,
  PackageEvidence,
  ReleaseTarget,
  TargetAuthentication,
} from '../../types/registry';

export function releaseTargets(
  value: unknown,
  context: string,
  schemaVersion: 1 | 2 = 1,
): ReleaseTarget[] {
  if (!Array.isArray(value) || !value.length) throw new Error(`${context}: targets are required`);
  const seen = new Set<string>();
  return value.map((raw, index): ReleaseTarget => {
    if (!record(raw)) throw new Error(`${context}: target ${index} must be an object`);
    const client = requiredString(raw, 'client', `${context} target ${index}`) as ClientID;
    const scope = schemaVersion === 2
      ? requiredString(raw, 'scope', `${context} target ${index}`)
      : undefined;
    if (schemaVersion === 2 && (raw.scopes !== undefined || (scope !== 'user' && scope !== 'project')))
      throw new Error(`${context}: v2 grant cannot borrow legacy scopes`);
    const key = schemaVersion === 2 ? `${client}:${scope}` : client;
    if (!CLIENTS.has(client) || seen.has(key))
      throw new Error(`${context}: target ${index} has an invalid or duplicate client/scope`);
    seen.add(key);
    const delivery = requiredString(raw, 'delivery', `${context} target ${client}`);
    if (!['managed', 'prepared', 'manual_activation'].includes(delivery))
      throw new Error(`${context}: target ${client} has invalid delivery`);
    const authentication = requiredString(
      raw,
      'authentication',
      `${context} target ${client}`,
    ) as TargetAuthentication;
    if (!['not_required', 'required', 'unknown'].includes(authentication))
      throw new Error(`${context}: target ${client} has invalid authentication`);
    const scopes = schemaVersion === 2
      ? [scope!]
      : stringArray(raw.scopes, 'scopes', `${context} target ${client}`);
    if (schemaVersion === 2 &&
      (!Array.isArray(raw.current_evidence) ||
        raw.current_evidence.some((id) => typeof id !== 'string')))
      throw new Error(`${context}: v2 grant requires an evidence array`);
    let closure: ReleaseTarget['canonical_closure'];
    if (raw.canonical_closure !== undefined) {
      const value = raw.canonical_closure;
      if (schemaVersion !== 2 || !record(value) || typeof value.eligible !== 'boolean')
        throw new Error(`${context}: invalid v2 closure`);
      if (value.eligible) {
        const digest = requiredString(value, 'closure_digest', `${context} closure`);
        const floor = requiredString(value, 'minimum_installer_version', `${context} closure`);
        if (!DIGEST.test(digest) || !/^\\d+\\.\\d+\\.\\d+(?:-[0-9A-Za-z.-]+)?$/.test(floor))
          throw new Error(`${context}: invalid eligible v2 closure digest or floor`);
        closure = { eligible: true, closure_digest: digest, minimum_installer_version: floor };
      } else {
        closure = { eligible: false };
      }
    }
    let appBinding: ReleaseTarget['app_binding'];
    if (record(raw.app_binding)) {
      appBinding = {
        app_key: requiredString(
          raw.app_binding,
          'app_key',
          `${context} target ${client} app binding`,
        ),
        id: requiredString(raw.app_binding, 'id', `${context} target ${client} app binding`),
        mcp_server: requiredString(
          raw.app_binding,
          'mcp_server',
          `${context} target ${client} app binding`,
        ),
      };
    }
    if (client === 'chatgpt' && !appBinding)
      throw new Error(`${context}: ChatGPT target requires app_binding`);
    if (client !== 'chatgpt' && appBinding)
      throw new Error(`${context}: app_binding is valid only for ChatGPT`);
    return {
      client,
      authentication,
      delivery: delivery as ReleaseTarget['delivery'],
      scopes,
      ...(appBinding ? { app_binding: appBinding } : {}),
      ...(closure ? { canonical_closure: closure } : {}),
    };
  });
}

function evidenceTrustedForEligibility(
  item: Record<string, unknown>,
  level: EvidenceLevel,
  signerVouched: boolean,
): boolean {
  // On the published_snapshot call path, production receives only snapshot
  // bytes verified by the publication workflow. The Directory signer therefore
  // vouches that github_actions trust matched the repository's approved
  // workflow and protected-ref policy. Eligibility trust mirrors the signed
  // resolver's HasTrustedEligibilityProvenance: schema gates need reproducible
  // workflow provenance, while materialization and client runtime gates may
  // also use exact evidence reviewed by the Directory publisher.
  // Review previews never enable commands.
  if (!signerVouched || !record(item.trust)) return false;
  const trust = item.trust;
  if (trust.kind === 'github_actions') {
    const workflow = optionalString(trust, 'workflow');
    const sourceRef = optionalString(trust, 'source_ref');
    const sourceDigest = optionalString(trust, 'source_digest');
    const artifact = item.artifact;
    if (
      !workflow ||
      !EVIDENCE_WORKFLOW.test(workflow) ||
      !sourceRef ||
      !EVIDENCE_SOURCE_REF.test(sourceRef) ||
      !sourceDigest ||
      !REVISION.test(sourceDigest) ||
      !record(artifact) ||
      workflow.slice(0, workflow.indexOf('/.github/workflows/')) !== artifact.repository ||
      artifact.revision !== sourceDigest
    )
      return false;
    return true;
  }
  return (
    trust.kind === 'reviewed_external' &&
    ['materialization', 'discovery', 'runtime', 'oauth'].includes(level)
  );
}

export function evidenceFromSnapshot(
  input: unknown,
  distributionID: string,
  releaseSequence: number,
  treeDigest: string,
  selectedIDs: readonly string[],
  signerVouched: boolean,
): { client: ClientEvidence[]; package: PackageEvidence[] } {
  const client: ClientEvidence[] = [];
  const packageEvidence: PackageEvidence[] = [];
  if (!Array.isArray(input)) return { client, package: packageEvidence };
  for (const item of input) {
    if (!record(item)) continue;
    if (item.distribution_id !== distributionID || item.release_sequence !== releaseSequence)
      continue;
    if (item.package_tree_digest !== treeDigest) continue;
    if (!selectedIDs.includes(String(item.id))) continue;
    const level = item.level;
    const outcome = item.outcome;
    if (!['schema', 'materialization', 'discovery', 'runtime', 'oauth'].includes(String(level)))
      continue;
    if (
      !['passed', 'failed', 'inconclusive', 'not_tested', 'not_applicable'].includes(
        String(outcome),
      )
    )
      continue;
    if (
      !record(item.artifact) ||
      typeof item.artifact.repository !== 'string' ||
      !REPOSITORY.test(item.artifact.repository) ||
      typeof item.artifact.revision !== 'string' ||
      !REVISION.test(item.artifact.revision) ||
      typeof item.artifact.path !== 'string' ||
      !item.artifact.path ||
      typeof item.artifact.digest !== 'string' ||
      !DIGEST.test(item.artifact.digest)
    )
      continue;
    const artifact = {
      repository: item.artifact.repository,
      revision: item.artifact.revision,
      path: item.artifact.path,
      digest: item.artifact.digest,
      url: `https://github.com/${item.artifact.repository}/blob/${item.artifact.revision}/${item.artifact.path}`,
    };
    const common = {
      id: String(item.id),
      outcome: outcome as ClientEvidence['outcome'],
      package_tree_digest: treeDigest,
      trusted_for_eligibility: evidenceTrustedForEligibility(
        item,
        level as EvidenceLevel,
        signerVouched,
      ),
      ...(optionalString(item, 'observed_at')
        ? { tested_at: optionalString(item, 'observed_at') }
        : {}),
      artifact,
    };
    if (level === 'schema') {
      if (item.client !== undefined) continue;
      packageEvidence.push({ ...common, level: 'schema' });
      continue;
    }
    const evidenceClient = item.client as ClientID;
    const clientVersion = optionalString(item, 'client_version');
    const installerVersion = optionalString(item, 'installer_version');
    const os = optionalString(item, 'os');
    const architecture = optionalString(item, 'architecture');
    const testedAt = optionalString(item, 'observed_at');
    if (
      !CLIENTS.has(evidenceClient) ||
      !clientVersion ||
      !installerVersion ||
      !os ||
      !architecture ||
      !testedAt
    )
      continue;
    client.push({
      ...common,
      client: evidenceClient,
      level: level as ClientEvidence['level'],
      client_version: clientVersion,
      installer_version: installerVersion,
      os,
      architecture,
      tested_at: testedAt,
      ...(optionalString(item, 'dependency_identity')
        ? { dependency_identity: optionalString(item, 'dependency_identity') }
        : {}),
    });
  }
  return { client, package: packageEvidence };
}


export function eligibleTargetPolicies(
  kind: DistributionKind,
  release: Pick<DistributionReleaseView, 'targets' | 'blocking_clients' | 'materialized_clients'>,
): ReleaseTarget[] {
  return release.targets.filter(
    (target) =>
      target.canonical_closure?.eligible !== false &&
      !release.blocking_clients.includes(target.client) &&
      (kind !== 'upstream' || release.materialized_clients.includes(target.client)),
  );
}

