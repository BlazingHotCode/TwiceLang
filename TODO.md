# TODO

## Completion Criteria

- A TODO item is only done when it has:
- parser tests
- evaluator tests
- codegen tests
- CLI/runtime tests
- adding snippet to [test.tw](./test.tw) and running using binary to test the code
- adding guide/points to show the feature in [README](./README.md)

## 1. Lambda Functions

- Add lambda function syntax: `(var1: type1) returnType => { ... }`
- Allow assigning lambdas to constants/variables:
- `const mylambda = (a: int) int => { return a * a; };`
- Allow direct invocation of lambda values:
- `mylambda(2)`
- Support parser/evaluator/codegen parity for lambda declaration, typing, and invocation.

## 2. OOP and Type System Expansion

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
- Generic classes are supported by syntax plan, but are implemented together with class support.

## 3. Inheritance

- Add inheritance as a separate feature from base class/object support.
- Define overriding and method resolution rules.
- Generic constraints are postponed until after inheritance.
