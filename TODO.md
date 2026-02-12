# TODO

## 1. Function Codegen

- Implement codegen for user-defined function literals (`fn(...) { ... }`).
- Implement codegen for non-builtin function calls.
- Support parameter passing and local function stack frames.
- Align function return behavior between evaluator and codegen.

## 2. Complete `const` Support

- Add parser support for `const` statements.
- Add AST node(s) for constants if needed.
- Add evaluator behavior enforcing immutability.
- Add codegen behavior for constant bindings.

## 3. Real Codegen Error Handling

- Replace assembly comment errors with real Go errors (e.g. undefined variable, unknown function, invalid call target).
- Propagate codegen failures up to CLI and stop compilation on error.
- Improve user-facing error messages with source context when possible.

## 4. Explicit Variable Types And Typed Declarations

- Add support for explicit variable type annotations with syntax: `let name: type = value`.
- Add support for typed declarations without an initial value (declaration-only form), e.g. `let name: type;`.
- Extend lexer/parser/AST to represent `:` and declared types.
- Enforce declared-type compatibility in evaluator/codegen.

## 5. Automatic Variable Type Inference

- Infer variable type automatically from assigned value when no explicit type is provided.
- Store inferred type metadata for later validations and codegen decisions.
- Validate reassignment/update operations against inferred type rules (once reassignment exists).
