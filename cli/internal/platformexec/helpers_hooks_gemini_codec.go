package platformexec

import (
	"encoding/json"
	"fmt"
)

func parseGeminiHooks(body []byte) (geminiHooksFile, error) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return geminiHooksFile{}, err
	}
	hooksValue, ok := raw["hooks"]
	if !ok {
		return geminiHooksFile{}, fmt.Errorf("Gemini hooks file must define a top-level hooks object")
	}
	if _, ok := hooksValue.(map[string]any); !ok {
		return geminiHooksFile{}, fmt.Errorf("Gemini hooks file must define a top-level hooks object")
	}
	var hooks geminiHooksFile
	if err := json.Unmarshal(body, &hooks); err != nil {
		return geminiHooksFile{}, err
	}
	if hooks.Hooks == nil {
		return geminiHooksFile{}, fmt.Errorf("Gemini hooks file must define a top-level hooks object")
	}
	return hooks, nil
}
