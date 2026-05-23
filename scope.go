package gioc

// Scope controls provider instance reuse.
type Scope uint8

const (
	// Singleton reuses one instance.
	Singleton Scope = iota

	// Prototype creates a new instance per request.
	Prototype
)
