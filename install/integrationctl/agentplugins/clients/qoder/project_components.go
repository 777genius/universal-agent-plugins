package qoder

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "sort"

    "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
    "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
    "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
    "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const (
    qoderProjectSkillKind = "qoder_project_skill_directory"
    qoderProjectMCPKind = "qoder_project_mcp_server"
)

func qoderComponents(project string) shared.ProjectComponents {
    return shared.ProjectComponents{
        ProjectRoot: project,
        SkillsRoot: filepath.Join(project, ".qoder", "skills"),
        ConfigPaths: nativeconfig.Paths{JSON: filepath.Join(project, ".mcp.json")},
        Codec: nativeconfig.CodecQoder,
        SkillKind: qoderProjectSkillKind,
        MCPKind: qoderProjectMCPKind,
    }
}

func qoderNativeOwned(objects []domain.NativeObjectOwnership) []domain.NativeObjectOwnership {
    result := make([]domain.NativeObjectOwnership, 0, len(objects))
    for _, object := range objects {
        if object.Kind != "managed_package_directory" {
            result = append(result, object)
        }
    }
    return result
}

func qoderProjectObjects(root string, plan domain.DeliveryPlan) ([]domain.NativeObjectOwnership, error) {
    layout := qoderComponents(plan.TargetContext.ProjectRoot)
    objects := []domain.NativeObjectOwnership{}
    for _, component := range plan.Components {
        if component.Support == domain.SupportUnsupported {
            continue
        }
        var object domain.NativeObjectOwnership
        var err error
        switch component.Kind {
        case domain.ComponentSkill:
            relative := "skills/" + component.Name
            if component.Support == domain.SupportProjected && component.Source == domain.ComponentSourceCommand {
                relative = projectNativeView + "/skills/" + component.Name
            }
            if err = pathpolicy.ValidateLeafID(component.Name); err == nil {
                if _, statErr := os.Lstat(filepath.Join(root, relative, "SKILL.md")); statErr != nil {
                    err = statErr
                }
            }
            if err == nil {
                object, err = layout.SkillObject(component.Name, relative, root)
            }
        case domain.ComponentMCPServer:
            var server nativeconfig.Server
            server, err = qoderProjectServer(root, component.Name)
            if err == nil {
                object, err = layout.MCPObject(component.Name, server, layout.ConfigPaths.JSON)
            }
        default:
            return nil, fmt.Errorf("unqualified Qoder project component %q", component.Kind)
        }
        if err != nil { return nil, err }
        objects = append(objects, object)
    }
    sort.Slice(objects, func(i, j int) bool { return objects[i].ObjectID < objects[j].ObjectID })
    return objects, layout.ValidateObjects(objects)
}

// The staged .mcp.json uses the existing OpenAI-shaped projection, whose
// stdio type is implicit and HTTP type is "http". Decode without adopting
// unknown metadata or native trust fields into an ownership receipt.
func qoderProjectServer(root, name string) (nativeconfig.Server, error) {
    path := filepath.Join(root, ".mcp.json")
    if err := pathpolicy.RequireContainedChild(root, path); err != nil {
        return nativeconfig.Server{}, err
    }
    body, err := os.ReadFile(path)
    if err != nil { return nativeconfig.Server{}, err }
    document, err := shared.DecodeStrictJSONObject(body)
    if err != nil { return nativeconfig.Server{}, err }
    servers, ok := document["mcpServers"].(map[string]any)
    if !ok { return nativeconfig.Server{}, fmt.Errorf("Qoder projected mcpServers is not an object") }
    entry, ok := servers[name].(map[string]any)
    if !ok { return nativeconfig.Server{}, fmt.Errorf("Qoder projected MCP server %q is absent", name) }
    kind, _ := entry["type"].(string)
    var allowed map[string]bool
    switch kind {
    case "", "stdio":
        allowed = map[string]bool{"type":true, "command":true, "args":true, "env":true, "cwd":true}
    case "http":
        allowed = map[string]bool{"type":true, "url":true, "headers":true}
    default:
        return nativeconfig.Server{}, fmt.Errorf("Qoder MCP server %q uses an unqualified transport", name)
    }
    for key := range entry {
        if !allowed[key] { return nativeconfig.Server{}, fmt.Errorf("Qoder MCP server %q has unqualified field %q", name, key) }
    }
    encoded, err := json.Marshal(entry)
    if err != nil { return nativeconfig.Server{}, err }
    var server nativeconfig.Server
    if err := json.Unmarshal(encoded, &server); err != nil { return server, err }
    if kind == "http" { server.Type = "remote" } else { server.Type = "stdio" }
    server.StdioValuesResolved, server.CWDResolved = true, true
    return server, nil
}

func qoderDesiredServers(root string, objects []domain.NativeObjectOwnership) (map[string]nativeconfig.Server, error) {
    servers := map[string]nativeconfig.Server{}
    for _, object := range objects {
        if object.Kind != qoderProjectMCPKind { continue }
        server, err := qoderProjectServer(root, object.LogicalName)
        if err != nil { return nil, err }
        servers[object.LogicalName] = server
    }
    return servers, nil
}

func validateQoderProjectSelection(plan domain.DeliveryPlan, objects []domain.NativeObjectOwnership) error {
    selected := map[string]bool{}
    for _, component := range plan.Components {
        if component.Support == domain.SupportUnsupported { continue }
        kind := qoderProjectSkillKind
        if component.Kind == domain.ComponentMCPServer { kind = qoderProjectMCPKind } else if component.Kind != domain.ComponentSkill {
            return fmt.Errorf("Qoder project selection includes unsupported component")
        }
        id := kind + ":" + component.Name
        if selected[id] { return fmt.Errorf("duplicate Qoder project component") }
        selected[id] = true
    }
    if len(selected) != len(objects) { return fmt.Errorf("Qoder project component receipts differ from selected components") }
    for _, object := range objects {
        if !selected[object.ObjectID] { return fmt.Errorf("Qoder project component receipt is unselected") }
    }
    return qoderComponents(plan.TargetContext.ProjectRoot).ValidateObjects(objects)
}
