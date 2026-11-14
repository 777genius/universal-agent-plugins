package gemini

import (
	"encoding/json"
)

type settingDoc struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	EnvVar      string `yaml:"env_var"`
	Sensitive   bool   `yaml:"sensitive"`
}

func marshalJSON(value any) ([]byte, error) {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(body, '\n'), nil
}
