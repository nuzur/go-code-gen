package project

// ValidationConfig controls whether the generated code emits per-entity data
// validation (a Validate() method on each entity plus a shared validation
// helper package) and calls it on the core write path.
//
// Validation is emitted by DEFAULT (opt-out): existing projects gain it
// automatically. Set Disabled to turn it off. This mirrors the change-request
// data validation enforced server-side in product/module/datavalidation, so
// data written through a generated API is held to the same schema rules.
type ValidationConfig struct {
	Disabled bool `json:"disabled"`
}

// ValidationEnabled reports whether generated data validation should be emitted
// and invoked. Default-on: enabled unless explicitly disabled.
func (p Project) ValidationEnabled() bool {
	return !p.ValidationConfig.Disabled
}
