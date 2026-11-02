package directoryv1

import (
	"encoding/json"
	"fmt"
	"strings"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func requireRootFields(body []byte, fields ...string) error {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(body, &root); err != nil {
		return err
	}
	return requiredMap(root, fields...)
}
func requireSnapshotFields(body []byte) error {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(body, &root); err != nil {
		return err
	}
	if err := requiredMap(root, "snapshot_schema_version", "sequence", "publication_id", "source_commit", "generated_at", "expires_at", "products", "distributions", "evidence", "revocations"); err != nil {
		return err
	}
	var sequence uint64
	if err := json.Unmarshal(root["sequence"], &sequence); err != nil {
		return err
	}
	legacyEvidence := sequence < domain.DirectoryEvidenceTrustCutoverSequence
	var products []map[string]json.RawMessage
	if err := json.Unmarshal(root["products"], &products); err != nil {
		return err
	}
	for _, p := range products {
		if err := requiredMap(p, "schema_version", "id", "display_name", "description", "manifest_name", "aliases", "reserved_aliases", "categories", "minimum_capabilities", "default_distribution", "distributions"); err != nil {
			return err
		}
		var minimum map[string]json.RawMessage
		if err := json.Unmarshal(p["minimum_capabilities"], &minimum); err != nil {
			return err
		}
		if err := requiredMap(minimum, "skills", "mcp"); err != nil {
			return err
		}
		if raw, ok := p["icon"]; ok {
			var icon map[string]json.RawMessage
			if err := json.Unmarshal(raw, &icon); err != nil {
				return err
			}
			if err := requiredMap(icon, "path", "digest"); err != nil {
				return err
			}
		}
	}
	var distributions []map[string]json.RawMessage
	if err := json.Unmarshal(root["distributions"], &distributions); err != nil {
		return err
	}
	for _, d := range distributions {
		if err := requiredMap(d, "schema_version", "id", "product_id", "kind", "status", "packager", "releases", "release_policies"); err != nil {
			return err
		}
		var releases []map[string]json.RawMessage
		if err := json.Unmarshal(d["releases"], &releases); err != nil {
			return err
		}
		for _, r := range releases {
			if err := requiredMap(r, "sequence", "package_version", "manifest_name", "agent_plugins_schema", "package_source", "tree_digest_algorithm", "tree_digest", "manifest_digest", "components", "published_at"); err != nil {
				return err
			}
			var source map[string]json.RawMessage
			if err := json.Unmarshal(r["package_source"], &source); err != nil {
				return err
			}
			if err := requiredMap(source, "repository", "revision", "path"); err != nil {
				return err
			}
			if raw, ok := r["build_provenance"]; ok {
				var provenance map[string]json.RawMessage
				if err := json.Unmarshal(raw, &provenance); err != nil {
					return err
				}
				if err := requiredMap(provenance, "upstream_repository", "upstream_revision"); err != nil {
					return err
				}
			}
		}
		var policies []map[string]json.RawMessage
		if err := json.Unmarshal(d["release_policies"], &policies); err != nil {
			return err
		}
		for _, p := range policies {
			if err := requiredMap(p, "release_sequence", "status", "minimum_installer_version", "targets", "current_evidence"); err != nil {
				return err
			}
			var targets []map[string]json.RawMessage
			if err := json.Unmarshal(p["targets"], &targets); err != nil {
				return err
			}
			for _, target := range targets {
				if err := requiredMap(target, "client", "scopes", "delivery", "authentication"); err != nil {
					return err
				}
				if raw, ok := target["app_binding"]; ok {
					var binding map[string]json.RawMessage
					if err := json.Unmarshal(raw, &binding); err != nil {
						return err
					}
					if err := requiredMap(binding, "app_key", "id", "mcp_server"); err != nil {
						return err
					}
				}
			}
		}
	}
	var evidence []map[string]json.RawMessage
	if err := json.Unmarshal(root["evidence"], &evidence); err != nil {
		return err
	}
	for _, e := range evidence {
		required := []string{"schema_version", "id", "distribution_id", "release_sequence", "package_tree_digest", "level", "outcome", "artifact"}
		if legacyEvidence {
			if _, present := e["trust"]; present {
				return fmt.Errorf("legacy evidence cannot contain trust")
			}
			required = append(required, "product_id", "manifest_digest", "source_repository", "source_revision", "source_path")
		} else {
			for _, field := range []string{"product_id", "manifest_digest", "source_repository", "source_revision", "source_path", "adapter_version"} {
				if _, present := e[field]; present {
					return fmt.Errorf("wire evidence cannot contain legacy field %q", field)
				}
			}
			required = append(required, "trust")
		}
		if err := requiredMap(e, required...); err != nil {
			return err
		}
		var artifact map[string]json.RawMessage
		if err := json.Unmarshal(e["artifact"], &artifact); err != nil {
			return err
		}
		if err := requiredMap(artifact, "repository", "revision", "path", "digest"); err != nil {
			return err
		}
		if raw, ok := e["trust"]; ok {
			var trust map[string]json.RawMessage
			if err := json.Unmarshal(raw, &trust); err != nil {
				return err
			}
			if err := requiredMap(trust, "kind"); err != nil {
				return err
			}
			for _, field := range []string{"bundle_manifest", "launch_artifact", "observer_artifact", "evidence_index"} {
				if artifactRaw, present := trust[field]; present {
					var trustArtifact map[string]json.RawMessage
					if err := json.Unmarshal(artifactRaw, &trustArtifact); err != nil {
						return err
					}
					if err := requiredMap(trustArtifact, "repository", "revision", "path", "digest"); err != nil {
						return err
					}
				}
			}
		}
	}
	var revocations []map[string]json.RawMessage
	if err := json.Unmarshal(root["revocations"], &revocations); err != nil {
		return err
	}
	for _, r := range revocations {
		if err := requiredMap(r, "distribution_id", "release_sequence"); err != nil {
			return err
		}
	}
	return nil
}
func requiredMap(value map[string]json.RawMessage, fields ...string) error {
	if value == nil {
		return fmt.Errorf("expected object")
	}
	for _, f := range fields {
		raw, ok := value[f]
		if !ok {
			return fmt.Errorf("missing required field %q", f)
		}
		if string(raw) == "null" {
			return fmt.Errorf("required field %q cannot be null", f)
		}
	}
	return nil
}
func isJSONNull(value json.RawMessage) bool { return strings.TrimSpace(string(value)) == "null" }
