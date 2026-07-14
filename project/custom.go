package project

// CustomConfig controls the optional "custom application" zone: a once-only
// scaffold (a hand-written server that wraps the generated one, plus a
// user-owned main.go) that lets a generated app add custom endpoints / custom
// logic on top of the generated CRUD service.
//
// It is OFF by default. When disabled, generation is byte-identical to before
// (existing consumers like nem and sfapi are unaffected). When enabled, the
// scaffold files are emitted ONCE and never overwritten on regeneration.
type CustomConfig struct {
	Enabled bool
	// Dir is the package/directory for the custom application layer, relative
	// to the project root. Defaults to "app".
	Dir string
}
