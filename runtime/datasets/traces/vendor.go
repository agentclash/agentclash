package traces

import (
	datasetadapters "github.com/agentclash/agentclash/runtime/datasets/adapters"
)

func importVendorSpans(platform SourcePlatform, data []byte) (datasetadapters.ImportResult, error) {
	format := string(platform)
	return datasetadapters.Import(format, data, datasetadapters.Mapping{})
}
