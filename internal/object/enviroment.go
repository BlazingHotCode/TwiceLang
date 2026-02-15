package object

import "twice/internal/ast"

// Environment stores variable bindings
// It's a map with a link to an outer scope (for nested functions)
type Environment struct {
	store            map[string]Object
	constStore       map[string]bool
	typeStore        map[string]string
	typeAlias        map[string]string
	genericTypeAlias map[string]GenericTypeAlias
	structDefs       map[string]*ast.StructStatement
	structMethods    map[string]Object
	importNamespaces map[string]string
	importMembers    map[string]string
	outer            *Environment // Parent scope, nil for global scope
}

type GenericTypeAlias struct {
	TypeParams []string
	TypeName   string
}

// NewEnvironment creates a new global environment
func NewEnvironment() *Environment {
	s := make(map[string]Object)
	return &Environment{
		store:            s,
		constStore:       make(map[string]bool),
		typeStore:        make(map[string]string),
		typeAlias:        make(map[string]string),
		genericTypeAlias: make(map[string]GenericTypeAlias),
		structDefs:       make(map[string]*ast.StructStatement),
		structMethods:    make(map[string]Object),
		importNamespaces: make(map[string]string),
		importMembers:    make(map[string]string),
		outer:            nil,
	}
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

func (e *Environment) SetType(name string, t string) {
	e.typeStore[name] = t
}

func (e *Environment) TypeOf(name string) (string, bool) {
	t, ok := e.typeStore[name]
	if ok {
		return t, true
	}
	if e.outer != nil {
		return e.outer.TypeOf(name)
	}
	return "", false
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

func (e *Environment) SetTypeAlias(name, target string) {
	e.typeAlias[name] = target
}

func (e *Environment) TypeAlias(name string) (string, bool) {
	if t, ok := e.typeAlias[name]; ok {
		return t, true
	}
	if e.outer != nil {
		return e.outer.TypeAlias(name)
	}
	return "", false
}

func (e *Environment) HasTypeAliasInCurrentScope(name string) bool {
	_, ok := e.typeAlias[name]
	return ok
}

func (e *Environment) SetGenericTypeAlias(name string, alias GenericTypeAlias) {
	e.genericTypeAlias[name] = alias
}

func (e *Environment) GenericTypeAlias(name string) (GenericTypeAlias, bool) {
	if t, ok := e.genericTypeAlias[name]; ok {
		return t, true
	}
	if e.outer != nil {
		return e.outer.GenericTypeAlias(name)
	}
	return GenericTypeAlias{}, false
}

func (e *Environment) HasGenericTypeAliasInCurrentScope(name string) bool {
	_, ok := e.genericTypeAlias[name]
	return ok
}

func (e *Environment) SetStruct(name string, st *ast.StructStatement) {
	e.structDefs[name] = st
}

func (e *Environment) Struct(name string) (*ast.StructStatement, bool) {
	if st, ok := e.structDefs[name]; ok {
		return st, true
	}
	if e.outer != nil {
		return e.outer.Struct(name)
	}
	return nil, false
}

func (e *Environment) HasStructInCurrentScope(name string) bool {
	_, ok := e.structDefs[name]
	return ok
}

func structMethodKey(receiverType, methodName string) string {
	return receiverType + "::" + methodName
}

func (e *Environment) SetStructMethod(receiverType, methodName string, fn Object) {
	e.structMethods[structMethodKey(receiverType, methodName)] = fn
}

func (e *Environment) StructMethod(receiverType, methodName string) (Object, bool) {
	if fn, ok := e.structMethods[structMethodKey(receiverType, methodName)]; ok {
		return fn, true
	}
	if e.outer != nil {
		return e.outer.StructMethod(receiverType, methodName)
	}
	return nil, false
}

func (e *Environment) SetImportNamespace(alias, path string) {
	e.importNamespaces[alias] = path
}

func (e *Environment) ImportNamespace(alias string) (string, bool) {
	if p, ok := e.importNamespaces[alias]; ok {
		return p, true
	}
	if e.outer != nil {
		return e.outer.ImportNamespace(alias)
	}
	return "", false
}

func (e *Environment) SetImportMember(alias, target string) {
	e.importMembers[alias] = target
}

func (e *Environment) ImportMember(alias string) (string, bool) {
	if p, ok := e.importMembers[alias]; ok {
		return p, true
	}
	if e.outer != nil {
		return e.outer.ImportMember(alias)
	}
	return "", false
}
