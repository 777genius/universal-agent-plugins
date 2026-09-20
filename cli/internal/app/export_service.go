package app

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/validate"
	"slices"
)

func (PluginService) Export(opts PluginExportOptions) (PluginExportResult, error) {
	return runExportService(opts)
}

func exportBlockingFailures(failures []validate.Failure) []validate.Failure {
	return slices.DeleteFunc(slices.Clone(failures), func(f validate.Failure) bool {
		return f.Kind == validate.FailureGeneratedContractInvalid
	})
}
