# TODO

## 1. Complete Null-Safe Member Access

- Expand null-safe access beyond partial handling so `?.` works consistently for member reads and method calls.
- Keep array `?.length()` support and generalize to other valid members.
- Ensure parser, evaluator, and codegen behavior matches.

## 2. Expand `hasField`

- Remove current codegen limitation that requires compile-time-only field strings.
- Support broader object/struct/class field checks instead of only array `"length"`.
- Keep behavior aligned between evaluator and codegen.

## 3. Function Literals in Codegen

- Add codegen support for function literals/anonymous functions.
- Match existing parser/evaluator behavior.
- Add tests for calls, captures, and failure cases.

## 4. Empty Literal Support

- Add support for empty array/tuple literals where type information is available.
- Define clear typing rules for empty literals in declarations and assignments.
- Keep evaluator and codegen parity.

## 5. Remove Function Arity/Capture Hard Limit

- Remove/raise the current codegen limit of 6 combined parameters/captures.
- Implement a stable calling convention strategy for larger functions.
- Add regression tests for higher arity and capture counts.

## 6. String Formatting

- Add escape-sequence formatting in strings (for example `\n`, `\t`, and related escapes).
- Add string interpolation with `${...}` expressions inside string literals.

## 7. Structs

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

## 8. Pointers

- Pointer types are nullable only when explicitly declared nullable.
- Add pointer operations (`&value`, `*ptr`, pointer assignment).
- Pointer method calls auto-deref: allow `p.method()` without requiring `(*p).method()`.
- Pointer + struct design should be combined, so references and mutation go through pointers.

## 9. Custom Libraries

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

## 10. OOP and Type System Expansion

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

## 11. Inheritance

- Add inheritance as a separate feature from base class/object support.
- Define overriding and method resolution rules.
