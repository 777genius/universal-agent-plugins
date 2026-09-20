package platformexec

import "os"

func readImportedOpenCodeConfigSource(source importedOpenCodeConfigSource) (importedOpenCodeConfig, error) {
	body, err := os.ReadFile(source.path)
	if err != nil {
		return importedOpenCodeConfig{}, err
	}
	return decodeImportedOpenCodeConfig(body)
}
