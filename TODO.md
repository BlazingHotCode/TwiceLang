# TODO

## 1. Null-Safe Access and Coalescing

- Add TypeScript-style null-safe member/method access with `?.`.
- `a?.b` or `a?.fn(...)` evaluates to `null` when `a` is `null`, otherwise performs normal access/call.
- Add TypeScript-style null-coalescing operator `??`.
- `x ?? y` evaluates to `y` only when `x` is `null`, otherwise evaluates to `x`.
- Support chaining like `a?.b?.c ?? fallback`.
- Do not add null-safe index/call forms for bounds/type safety bypass:
- Keep normal behavior for index/function errors (for example, index out of range or invalid function input should still error).
- Use TypeScript-like precedence/behavior for `??` with `||`/`&&` (require parentheses in mixed cases).
- Keep `?.` as read-access only (no null-safe assignment form).
- For field existence checks before assignment, add helper:
- `hasField(obj, field)`

## 2. Structs

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

## 3. Pointers

- Pointer types are nullable only when explicitly declared nullable.
- Add pointer operations (`&value`, `*ptr`, pointer assignment).
- Pointer method calls auto-deref: allow `p.method()` without requiring `(*p).method()`.
- Pointer + struct design should be combined, so references and mutation go through pointers.

## 4. Custom Libraries

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

## 5. OOP and Type System Expansion

- Add class declarations and object instantiation.
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

## 6. Inheritance

- Add inheritance as a separate feature from base class/object support.
- Define overriding and method resolution rules.
