package installer

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (e *Engine) knownTargetIndex(facts []TargetFacts) (map[string]TargetFacts, error) {
	indexed := make(map[string]TargetFacts, len(facts))
	for _, fact := range facts {
		if !e.SupportsClient(fact.ClientID) {
			return nil, fmt.Errorf("%w: client %q is not supported", ErrTargetFactsUnavailable, fact.ClientID)
		}
		if fact.BindingID == "" {
			return nil, fmt.Errorf("%w: BindingID is required for %s", ErrTargetFactsUnavailable, fact.ClientID)
		}
		if !validRoot(fact.ConfigRoot) {
			return nil, fmt.Errorf("%w: ConfigRoot must be an explicit absolute clean path for %s", ErrTargetFactsUnavailable, fact.ClientID)
		}
		if !validRoot(fact.Executable) {
			return nil, fmt.Errorf("%w: Executable must be an explicit absolute clean path for %s", ErrTargetFactsUnavailable, fact.ClientID)
		}
		if _, exists := indexed[fact.ClientID]; exists {
			return nil, fmt.Errorf("%w: duplicate facts for %s", ErrTargetFactsUnavailable, fact.ClientID)
		}
		indexed[fact.ClientID] = fact
	}
	return indexed, nil
}

func (e *Engine) detectedClients(req Request) (map[domain.ClientID]domain.DetectedClient, error) {
	detected := map[domain.ClientID]domain.DetectedClient{}
	add := func(client domain.DetectedClient) error {
		if current, exists := detected[client.ClientID]; exists {
			if current.ConfigRoot != client.ConfigRoot || current.ExecutablePath != client.ExecutablePath {
				return fmt.Errorf("%w: conflicting paths for %s", ErrTargetFactsUnavailable, client.ClientID)
			}
			return nil
		}
		detected[client.ClientID] = client
		return nil
	}
	if len(req.Targets) == 0 {
		client, err := e.detectedClient(req)
		if err != nil {
			return nil, err
		}
		if err := add(client); err != nil {
			return nil, err
		}
	} else {
		for _, target := range req.Targets {
			client, err := e.detectedClient(Request{
				Operation: req.Operation, ClientID: target.ClientID,
				ClientConfigRoot: target.ClientConfigRoot,
				ClientExecutable: firstNonEmpty(target.ClientExecutable, req.ClientExecutable),
			})
			if err != nil {
				return nil, err
			}
			if err := add(client); err != nil {
				return nil, err
			}
		}
	}
	known, err := e.knownTargetIndex(req.KnownTargets)
	if err != nil {
		return nil, err
	}
	for _, fact := range known {
		client, err := e.detectedClient(Request{
			Operation: req.Operation, ClientID: fact.ClientID,
			ClientConfigRoot: fact.ConfigRoot, ClientExecutable: fact.Executable,
		})
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrTargetFactsUnavailable, err)
		}
		if err := add(client); err != nil {
			return nil, err
		}
	}
	return detected, nil
}
