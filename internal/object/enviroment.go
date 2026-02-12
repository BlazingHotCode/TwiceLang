package object

// Environment stores variable bindings
// It's a map with a link to an outer scope (for nested functions)
type Environment struct {
	store map[string]Object
	outer *Environment // Parent scope, nil for global scope
}

// NewEnvironment creates a new global environment
func NewEnvironment() *Environment {
	s := make(map[string]Object)
	return &Environment{store: s, outer: nil}
}

// NewEnclosedEnvironment creates a new scope enclosed by outer
// Used when calling functions - they get their own scope
func NewEnclosedEnvironment(outer *Environment) *Environment {
	env := NewEnvironment()
	env.outer = outer
	return env
}

// Get looks up a variable by name
// Checks current scope, then outer scopes recursively
func (e *Environment) Get(name string) (Object, bool) {
	obj, ok := e.store[name]
	if !ok && e.outer != nil {
		obj, ok = e.outer.Get(name)
	}
	return obj, ok
}

// Set creates or updates a variable in the current scope
func (e *Environment) Set(name string, val Object) Object {
	e.store[name] = val
	return val
}