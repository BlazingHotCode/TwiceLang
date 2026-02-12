package object

// Environment stores variable bindings
// It's a map with a link to an outer scope (for nested functions)
type Environment struct {
	store      map[string]Object
	constStore map[string]bool
	outer      *Environment // Parent scope, nil for global scope
}

// NewEnvironment creates a new global environment
func NewEnvironment() *Environment {
	s := make(map[string]Object)
	return &Environment{store: s, constStore: make(map[string]bool), outer: nil}
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

// SetConst creates a constant binding in the current scope.
func (e *Environment) SetConst(name string, val Object) Object {
	e.store[name] = val
	e.constStore[name] = true
	return val
}

// Has looks up whether a name exists in the current scope chain.
func (e *Environment) Has(name string) bool {
	_, ok := e.store[name]
	if ok {
		return true
	}
	if e.outer != nil {
		return e.outer.Has(name)
	}
	return false
}

// IsConst reports whether a name is constant in the current scope chain.
func (e *Environment) IsConst(name string) bool {
	_, ok := e.store[name]
	if ok {
		return e.constStore[name]
	}
	if e.outer != nil {
		return e.outer.IsConst(name)
	}
	return false
}

// Assign updates an existing binding in the current scope chain.
// Returns false if the name does not exist.
func (e *Environment) Assign(name string, val Object) bool {
	if _, ok := e.store[name]; ok {
		e.store[name] = val
		return true
	}
	if e.outer != nil {
		return e.outer.Assign(name, val)
	}
	return false
}

// HasInCurrentScope reports whether the name is already declared in this scope.
func (e *Environment) HasInCurrentScope(name string) bool {
	_, ok := e.store[name]
	return ok
}

// IsConstInCurrentScope reports whether the current-scope binding is const.
func (e *Environment) IsConstInCurrentScope(name string) bool {
	return e.constStore[name]
}
