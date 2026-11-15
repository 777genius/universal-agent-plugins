package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// A confirmed project installation may grant trust to its exact canonical
// folder. This persistent user preference is independent of plugin ownership;
// removal never revokes it or changes another project's trust.
func grantClaudeProjectTrust(env clients.Env, request domain.ActivationRequest) (bool, error) {
	project, err := claudeTrustableProject(request)
	if err != nil {
		return false, err
	}
	if request.VerifyOnly || request.Delivery.ArtifactDigest == "" {
		return false, fmt.Errorf("claude Code project trust requires a confirmed verified delivery")
	}
	return shared.UpdateProjectTrustFile(env.NativeConfig, filepath.Join(request.Client.ConfigRoot, ".claude.json"), func(body []byte) ([]byte, error) { return renderClaudeProjectTrust(body, project) })
}

func claudeTrustableProject(request domain.ActivationRequest) (string, error) {
	root, err := claudePlanProjectRoot(request.Client, request.Plan)
	if err != nil {
		return "", err
	}
	project := filepath.Dir(root)
	_, home, err := boundedClaudeProbeEnvironment()
	if err != nil {
		return "", err
	}
	physicalHome, err := filepath.EvalSymlinks(home)
	if err != nil {
		return "", fmt.Errorf("resolve Claude Code home before project trust: %w", err)
	}
	if project == home || project == physicalHome {
		return "", fmt.Errorf("claude Code project trust cannot grant the entire home directory")
	}
	return project, nil
}

func renderClaudeProjectTrust(body []byte, project string) ([]byte, error) {
	if !exactAbsolute(project) || filepath.Dir(project) == project {
		return nil, fmt.Errorf("claude Code project trust requires an exact absolute project")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	value, err := shared.DecodeUniqueJSONValue(decoder)
	if err != nil {
		return nil, fmt.Errorf("decode Claude Code project trust: %w", err)
	}
	if _, err = decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("claude Code project trust contains trailing data")
	}
	if _, ok := value.(map[string]any); !ok {
		return nil, fmt.Errorf("claude Code project trust must be an object")
	}
	// Keep unrelated native values as raw JSON: the strict duplicate-key
	// decoder's empty-array representation must not turn [] into null.
	doc := map[string]json.RawMessage{}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	projects := map[string]json.RawMessage{}
	if raw, exists := doc["projects"]; exists {
		if err := json.Unmarshal(raw, &projects); err != nil || projects == nil {
			return nil, fmt.Errorf("claude Code projects must be an object")
		}
	}
	entry := map[string]json.RawMessage{}
	if raw, exists := projects[project]; exists {
		if err := json.Unmarshal(raw, &entry); err != nil || entry == nil {
			return nil, fmt.Errorf("claude Code selected project must be an object")
		}
	}
	if raw, exists := entry["hasTrustDialogAccepted"]; exists {
		if string(raw) != "true" && string(raw) != "false" {
			return nil, fmt.Errorf("claude Code selected project trust must be boolean")
		}
		if string(raw) == "true" {
			return body, nil
		}
	}
	entry["hasTrustDialogAccepted"] = json.RawMessage("true")
	raw, err := json.Marshal(entry)
	if err != nil {
		return nil, err
	}
	projects[project] = raw
	raw, err = json.Marshal(projects)
	if err != nil {
		return nil, err
	}
	doc["projects"] = raw
	next, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(next, '\n'), nil
}

func observeClaudeProjectActivation(ctx context.Context, env clients.Env, request domain.ActivationRequest, probe ClaudeActivationProbe) (projectListObservation, error) {
	listed, err := runPreparedClaudeList(ctx, env, probe.Command)
	if err != nil {
		return projectListObservation{}, err
	}
	observe := func(body []byte) projectListObservation {
		return observeClaudeProjectList(body, request.DeclaredName, probe.ActivePath, request.Plan.TargetContext.ProjectRoot, request.Client.ConfigRoot)
	}
	observation := observe(listed.Stdout)
	if observation.state != projectListSuppressed || !observation.untrusted || request.VerifyOnly {
		return observation, nil
	}
	if _, err := grantClaudeProjectTrust(env, request); err != nil {
		return projectListObservation{}, err
	}
	listed, err = runPreparedClaudeList(ctx, env, probe.Command)
	if err != nil {
		return projectListObservation{}, err
	}
	return observe(listed.Stdout), nil
}
