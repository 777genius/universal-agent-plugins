package gemini

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func geminiMCPRequests(prepared *geminiNativeApply) ([]nativeconfig.Request, error) {
	requests := make([]nativeconfig.Request, 0)
	ids := make([]string, 0, prepared.idsCapacity)
	seen := map[string]bool{}
	for id := range prepared.previousByID {
		ids, seen[id] = append(ids, id), true
	}
	for id := range prepared.desiredByID {
		if !seen[id] {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		request, include, err := geminiMCPRequest(prepared, id)
		if err != nil {
			return nil, err
		}
		if include {
			requests = append(requests, request)
		}
	}
	return requests, nil
}

func geminiMCPRequest(prepared *geminiNativeApply, id string) (nativeconfig.Request, bool, error) {
	prior, hadPrior := prepared.previousByID[id]
	next, hasNext := prepared.desiredByID[id]
	if (hadPrior && prior.Kind != GeminiMCPObjectKind) || (hasNext && next.Kind != GeminiMCPObjectKind) {
		return nativeconfig.Request{}, false, nil
	}
	if hasNext {
		return geminiMCPUpsertRequest(prepared, prior, next, hadPrior)
	}
	if hadPrior {
		return geminiMCPRemoveRequest(prepared.kernel, prepared.configRoot, prior)
	}
	return nativeconfig.Request{}, false, nil
}

func geminiMCPUpsertRequest(prepared *geminiNativeApply, prior, next domain.NativeObjectOwnership, hadPrior bool) (nativeconfig.Request, bool, error) {
	server, err := geminiServerFromPackage(prepared.activePath, next.LogicalName)
	if err != nil {
		return nativeconfig.Request{}, false, err
	}
	server, err = MaterializeGeminiServer(server, prepared.activePath, prepared.descriptor.DataRoot)
	if err != nil {
		return nativeconfig.Request{}, false, err
	}
	present, owned, err := inspectOwnedGeminiMCP(prepared.kernel, prepared.configRoot, prior, hadPrior)
	if err != nil {
		return nativeconfig.Request{}, false, err
	}
	action := nativeconfig.ActionAdd
	var receipt *nativeconfig.Receipt
	if present {
		if !owned {
			return nativeconfig.Request{}, false, fmt.Errorf("the Gemini MCP server %q is no longer owned", prior.LogicalName)
		}
		action, receipt = nativeconfig.ActionUpdate, GeminiReceipt(prior)
	}
	return nativeconfig.Request{
		Paths: GeminiConfigPaths(prepared.configRoot), Codec: nativeconfig.CodecGemini, Action: action,
		Name: next.LogicalName, Server: server,
		Placeholders: nativeconfig.Placeholders{PackageRoot: prepared.activePath, DataRoot: prepared.descriptor.DataRoot},
		Owned:        receipt,
	}, true, nil
}

func inspectOwnedGeminiMCP(kernel nativeconfig.Kernel, configRoot string, prior domain.NativeObjectOwnership, hadPrior bool) (bool, bool, error) {
	if !hadPrior {
		return false, false, nil
	}
	return kernel.Inspect(GeminiConfigPaths(configRoot), nativeconfig.CodecGemini, prior.LogicalName, GeminiReceipt(prior))
}

func geminiMCPRemoveRequest(kernel nativeconfig.Kernel, configRoot string, prior domain.NativeObjectOwnership) (nativeconfig.Request, bool, error) {
	present, owned, err := kernel.Inspect(GeminiConfigPaths(configRoot), nativeconfig.CodecGemini, prior.LogicalName, GeminiReceipt(prior))
	if err != nil {
		return nativeconfig.Request{}, false, err
	}
	if !present {
		return nativeconfig.Request{}, false, nil
	}
	if !owned {
		return nativeconfig.Request{}, false, fmt.Errorf("the Gemini MCP server %q is no longer owned", prior.LogicalName)
	}
	return nativeconfig.Request{
		Paths: GeminiConfigPaths(configRoot), Codec: nativeconfig.CodecGemini,
		Action: nativeconfig.ActionRemove, Name: prior.LogicalName, Owned: GeminiReceipt(prior),
	}, true, nil
}

func verifyGeminiMCPReceipts(prepared *geminiNativeApply, requests []nativeconfig.Request) error {
	for _, request := range requests {
		if request.Action == nativeconfig.ActionRemove {
			continue
		}
		expected := prepared.desiredByID["gemini-mcp:"+request.Name]
		preview, err := nativeconfig.DesiredReceipt(filepath.Join(prepared.configRoot, "settings.json"), nativeconfig.CodecGemini, request.Name, request.Server, nativeconfig.Placeholders{PackageRoot: prepared.activePath, DataRoot: prepared.descriptor.DataRoot})
		if err != nil || preview.Digest != expected.ManagedDigest {
			return fmt.Errorf("the Gemini MCP server %q does not match staged ownership", request.Name)
		}
	}
	return nil
}
