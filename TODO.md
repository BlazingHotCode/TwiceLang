# TODO

## Completion Criteria

- A TODO item is only done when it has:
- parser tests
- evaluator tests
- codegen tests
- CLI/runtime tests
- adding snippet to [test.tw](./test.tw) and running using binary to test the code
- adding guide/points to show the feature in [README](./README.md)

## 1. Generics/Templates (Java-Style)

- Add generic type syntax using angle brackets, like Java: `Type<T>`.
- Support generic declarations for user-defined types/functions in v1.
- Support generic type aliases in v1 (for example `type Pair<T, U> = (T, U);`).
- Support generic call syntax in expression context (not just type positions).
- Use explicit generic type arguments by default.
- For generic methods, allow inference only when mapping is unambiguous; otherwise require explicit type arguments.
- Generic constraints are postponed until after inheritance.
- Support nested generics in v1 (for example `List<List<int>>`).
- In type context, parse `>>` as two generic closers when appropriate (not right-shift).
- Implement generics with monomorphization.
- Generate specializations on-demand for used instantiations only.
- Support cross-module specialization in v1 (imports included).
- Use readable mangled specialization names that include concrete type arguments.
- Missing/wrong generic type-argument arity must be compile/codegen errors (not runtime).
- Generic classes are supported by syntax plan, but are implemented together with class support.
- Keep parser, evaluator, and codegen behavior aligned for generics.

## 2. Lists (Dynamic Arrays)

- Add generic list types as dynamic arrays without fixed compile-time length.
- Use generic syntax for declarations, for example `let xs: List<int>;`.
- Keep fixed-size arrays (`type[len]`) as a separate type from `List<T>`.
- `List<T>` element typing is enforced.
- Mixed-type lists should use `List<any>`.
- List construction syntax is constructor-only:
- `new List<T>()`
- `new List<T>(val1, val2, ...)`
- List API is methods-only (for example `list.append(x)`), not global helper functions.
- `length` should work as both property and method (`list.length` and `list.length()`).
- v1 operations:
- `append(value)` -> returns `null`
- `remove(index)` -> returns `T||null`; invalid index throws runtime error
- `insert(index, value)` with valid index range `0..length`
- `pop()` -> returns `T||null`; empty list returns `null`
- `contains(value)` -> returns `bool||null` (null when incomparable)
- `clear()` -> returns `null`
- `contains(value)` argument typing:
- strict for concrete `List<T>`
- allow broader runtime-checked values for `List<any>` / union element lists
- List index get/set out-of-range throws runtime error.
- Appending `null` to `List<T>` where `T` excludes null throws runtime error.
- Fixed array/list interoperability requires explicit conversion both ways.
- Ensure parser, evaluator, and codegen all support lists consistently.

## 3. Structs

- Add struct declarations and typed struct values.
- Use literal construction syntax for structs.
- Struct fields are `public` by default.
- Keep struct methods tied to pointer behavior and references.
- Struct declaration shape:
- `struct Name { var1: type1, var2: type2, ... }`
- Constructor-style creation syntax:
- `let a: Name = new Name(val1, val2)`
- `let a: Name = new Name(var1 = val1, var2 = val2)`
- Allow type inference from constructor call:
- `let a = new Name(...)`
- Add nullable/optional struct fields with `?:` in struct declarations.
- Add default values in field declarations (`field: type = value`), which imply optional/nullable-style constructor behavior so caller can omit them.
- Explicit struct function declaration syntax:
- `fn (self: StructName) methodName(args...) ReturnType { ... }`
- Explicit mutating struct function declaration syntax (with pointers):
- `fn (self: *StructName) methodName(args...) { ... }`
- Struct function usage syntax:
- `value.methodName(args...)`
- Pointer struct function usage syntax (auto-deref):
- `ptr.methodName(args...)` (no `(*ptr).methodName(...)` required)

## 4. Pointers

- Pointer types are nullable only when explicitly declared nullable.
- Add pointer operations (`&value`, `*ptr`, pointer assignment).
- Pointer method calls auto-deref: allow `p.method()` without requiring `(*p).method()`.
- Pointer + struct design should be combined, so references and mutation go through pointers.

## 5. Custom Libraries

- Add `import ...` syntax.
- Built-in libraries are imported with `twice.<lib>`.
- Local libraries use non-`twice.` import paths.
- Namespace behavior depends on import form:
- Importing only the library exposes the library namespace.
- Importing `.<something>` exposes that specific member/namespace path.
- Add alias import syntax like:
- `import twice.math as math`
- Add member import alias syntax like:
- `import twice.math.sqrt as sqrt`

## 6. OOP and Type System Expansion

- Add class declarations and object instantiation.o
- Structs stay public-focused; classes support explicit `public`/`private`.
- Class members default to `private` when no modifier is provided.
- Classes support `static` functions and fields.
- Add `main` function for what to run.
- Put class functions directly inside class declarations (not implemented outside).
- Add instance methods and method dispatch on objects.
- Add constructors and initialization hooks.
- Constructors must initialize or receive all fields that do not have defaults.
- Constructor overloads are allowed.
- Expand type naming and introspection for class/instance/union/array types.
- Add member access (`obj.field`, `Class.staticField`) with visibility enforcement.
- Add tests and codegen parity for all OOP and visibility features.
- Use Java-style class syntax (fields/modifiers/constructors/methods declared inside class body).
- Class syntax target (example shape):
- `class Shape { private x: int; public Shape(...) { this.x = ...; } public fn area() int { return this.x; } public static fn make() Shape { ... } }`
- Constructor lives inside class declaration as `public ClassName(...) { ... }`.
- Use `this.var` style for instance field access inside class methods/constructors.
- `this` is valid in instance methods and constructors, and invalid in `static` methods.
- Typed declarations without `new` remain `null` until initialized (for example `let s: Shape;`).

## 7. Inheritance

- Add inheritance as a separate feature from base class/object support.
- Define overriding and method resolution rules.
