//go:build linux

package pluginkitairepo_test

import "fmt"

func copilotCycleStartupIndex(value any) (int, error) {
	row, ok := value.(map[string]any)
	if !ok {
		return 0, fmt.Errorf("Copilot cycle startup object")
	}
	switch row["type"] {
	case "session.mcp_server_status_changed":
		data, ok := row["data"].(map[string]any)
		if !ok {
			return 0, fmt.Errorf("Copilot cycle server state object")
		}
		if data["status"] == "pending" {
			return 0, nil
		}
		if data["status"] == "connected" {
			return 1, nil
		}
	case "session.mcp_server_tools_loaded":
		return 2, nil
	case "session.skills_loaded":
		return 3, nil
	case "session.info":
		return 4, nil
	case "session.mcp_servers_loaded":
		return 5, nil
	case "session.tools_updated":
		return 6, nil
	}
	return 0, fmt.Errorf("Copilot cycle unknown startup event/state")
}

func (s *copilotReadinessStream) cycleStartup(order [7]int) error {
	positions := map[int]int{}
	canonical := make([]map[string]any, 7)
	for i, kind := range order {
		if _, exists := positions[kind]; exists {
			return fmt.Errorf("Copilot cycle duplicate startup observation")
		}
		positions[kind] = i
		canonical[kind] = s.data[i]
	}
	if len(positions) != 7 || positions[0] >= positions[1] || positions[5] <= positions[1] || positions[5] <= positions[2] {
		return fmt.Errorf("Copilot cycle incomplete per-server startup state")
	}
	copy(s.data[:7], canonical)
	return nil
}
