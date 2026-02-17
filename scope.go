package gioc

// Scope controls how many instances of a provider are created.
type Scope uint8

const (
	// Singleton creates the instance once and returns the same pointer on every
	// subsequent request. All consumers within the container share the same object.
	Singleton Scope = iota

	// Prototype creates a new instance on every request. Each consumer receives
	// its own independent copy of the object.
	Prototype
)
