package schemas

import (
	"embed"
	_ "embed"
	"slices"
)

//go:embed *
var Schemas embed.FS
var SchemaTypes = []string{"authz", "event-queue", "feature-flags", "key-value-store", "ledger", "llm-vector", "page-views", "search-engine", "tinyurl", "transactional-outbox"}
var supported = []string{"postgres", "mysql", "sqlite"}

func IsSupported(engine string) bool {
	return slices.Contains(supported, engine)
}

func ProvidedSchema(s string) bool {
	return slices.Contains(SchemaTypes, s)
}
