package gemini

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tailscale/hujson"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Project trust is the owner's persistent workspace preference, not plugin
// ownership. Removal never revokes it. Verification never grants new trust.
func trustGeminiProject(env clients.Env, request domain.ActivationRequest) (bool, error) {
	if request.VerifyOnly || request.Plan.Scope != domain.ScopeProject {
		return false, nil
	}
	project, err := geminiTrustableProject(request)
	if err != nil {
		return false, err
	}
	return shared.UpdateProjectTrustFile(env.NativeConfig, filepath.Join(request.Client.ConfigRoot, "trustedFolders.json"), func(body []byte) ([]byte, error) { return geminiProjectTrustMapping(body, project) })
}

func geminiTrustableProject(request domain.ActivationRequest) (string, error) {
	if os.Getenv("GEMINI_RESTRICTED_MODE") == "true" || os.Getenv("GEMINI_CLI_TRUST_WORKSPACE") == "false" {
		return "", fmt.Errorf("gemini workspace trust is disabled by the existing environment policy")
	}
	if err := validateProfile(request.Client.ConfigRoot, ""); err != nil {
		return "", err
	}
	project := request.Plan.TargetContext.ProjectRoot
	canonical, err := clients.CanonicalDirectory(project, os.Lstat, filepath.EvalSymlinks)
	if err != nil || canonical != project {
		return "", fmt.Errorf("gemini project trust requires the selected canonical project")
	}
	if project == filepath.Dir(project) {
		return "", fmt.Errorf("gemini project trust cannot grant trust to the filesystem root")
	}
	if home, err := clients.CanonicalDirectory(os.Getenv("HOME"), os.Lstat, filepath.EvalSymlinks); err == nil && home == project {
		return "", fmt.Errorf("gemini project trust cannot grant trust to the entire home directory")
	}
	return project, nil
}

// A preview may offer confirmed setup only for an exact missing, grantable trust preference.
func geminiProjectTrustMissing(env clients.Env, request domain.ActivationRequest) (bool, error) {
	project, err := geminiTrustableProject(request)
	if err != nil {
		return false, err
	}
	file, err := env.NativeConfig.ReadExactFile(filepath.Join(request.Client.ConfigRoot, "trustedFolders.json"))
	if err != nil {
		return false, err
	}
	body := file.Body
	if !file.Exists {
		body = []byte("{}")
	}
	next, err := geminiProjectTrustMapping(body, project)
	return err == nil && !bytes.Equal(body, next), err
}

func geminiProjectTrustMapping(body []byte, project string) ([]byte, error) {
	ast, mapping, err := geminiTrustDocument(body)
	if err != nil {
		return nil, err
	}
	for path, value := range mapping {
		if err := geminiTrustRulePermits(path, value, project); err != nil {
			return nil, err
		}
	}
	if _, exists := mapping[project]; exists {
		return body, nil
	}
	object := ast.Value.(*hujson.Object)
	object.Members = append(object.Members, hujson.ObjectMember{Name: hujson.Value{Value: hujson.String(project)}, Value: hujson.Value{Value: hujson.String("TRUST_FOLDER")}})
	return ast.Pack(), nil
}

func geminiTrustDocument(body []byte) (hujson.Value, map[string]any, error) {
	if len(body) > 1<<20 {
		return hujson.Value{}, nil, fmt.Errorf("gemini workspace trust file exceeds limit")
	}
	ast, err := hujson.Parse(body)
	if err != nil {
		return ast, nil, fmt.Errorf("invalid Gemini workspace trust file")
	}
	// Native trust JSON accepts comments but not trailing commas.
	for value := range ast.All() {
		switch node := value.Value.(type) {
		case *hujson.Object:
			if len(node.Members) > 0 && node.Members[len(node.Members)-1].Value.AfterExtra != nil {
				return ast, nil, fmt.Errorf("invalid Gemini workspace trust trailing comma")
			}
		case *hujson.Array:
			if len(node.Elements) > 0 && node.Elements[len(node.Elements)-1].AfterExtra != nil {
				return ast, nil, fmt.Errorf("invalid Gemini workspace trust trailing comma")
			}
		}
	}
	standard, err := hujson.Standardize(bytes.Clone(body))
	if err != nil {
		return ast, nil, err
	}
	mapping, err := shared.DecodeStrictJSONObject(standard)
	if err != nil {
		return ast, nil, fmt.Errorf("invalid Gemini workspace trust object: %w", err)
	}
	return ast, mapping, nil
}

func geminiTrustRulePermits(path string, value any, project string) error {
	level, ok := value.(string)
	if !ok || level != "TRUST_FOLDER" && level != "TRUST_PARENT" && level != "DO_NOT_TRUST" {
		return fmt.Errorf("unknown Gemini workspace trust rule")
	}
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return fmt.Errorf("ambiguous Gemini workspace trust path")
	}
	if level != "DO_NOT_TRUST" {
		return nil
	}
	// Native getRealPath falls back to stored lexical paths for removed or
	// inaccessible history. Only live aliases of explicit denies need resolving.
	physical := path
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		physical = resolved
	}
	relative, err := filepath.Rel(physical, project)
	if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("preserved explicit Gemini workspace distrust decision")
	}
	return nil
}
