# TODO

## 1. Function Codegen

- Implement codegen for user-defined function literals (`fn(...) { ... }`).
- Implement codegen for non-builtin function calls.
- Support parameter passing and local function stack frames.
- Align function return behavior between evaluator and codegen.

## 2. Add More Variable Types

- Add additional language/runtime types beyond `int` and `bool`.
- Define parser/evaluator/codegen behavior for each new type.
- Extend type validation rules for declarations, reassignment, and function calls.

## 3. Explicit Variable Types And Typed Declarations

- Add support for explicit variable type annotations with syntax: `let name: type = value`.
- Add support for typed declarations without an initial value (declaration-only form), e.g. `let name: type;`.
- Extend lexer/parser/AST to represent `:` and declared types.
- Enforce declared-type compatibility in evaluator/codegen.

## 4. Automatic Variable Type Inference

- Infer variable type automatically from assigned value when no explicit type is provided.
- Store inferred type metadata for later validations and codegen decisions.
- Validate reassignment/update operations against inferred type rules (once reassignment exists).

## 5. `typeof` Builtin

- Add a `typeof(value)` builtin that returns the runtime type of its argument.
- Define output format for known types (e.g. `int`, `bool`, future types).
- Support `typeof` in evaluator and codegen paths consistently.
- Add parser/evaluator/codegen/CLI tests for `typeof`.
