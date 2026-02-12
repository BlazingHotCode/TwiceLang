# TODO

## 1. Function Codegen

- Implement codegen for user-defined function literals (`fn(...) { ... }`).
- Implement codegen for non-builtin function calls.
- Support parameter passing and local function stack frames.
- Align function return behavior between evaluator and codegen.

## 2. Explicit Variable Types And Typed Declarations

- Add support for explicit variable type annotations with syntax: `let name: type = value`.
- Add support for typed declarations without an initial value (declaration-only form), e.g. `let name: type;`.
- Extend lexer/parser/AST to represent `:` and declared types.
- Enforce declared-type compatibility in evaluator/codegen.

## 3. Automatic Variable Type Inference

- Infer variable type automatically from assigned value when no explicit type is provided.
- Store inferred type metadata for later validations and codegen decisions.
- Validate reassignment/update operations against inferred type rules (once reassignment exists).

## 4. Comment Syntax Support

- Add single-line comment support (e.g. `// comment`).
- Add block comment support (e.g. `/* comment */`), including multi-line blocks.
- Update lexer/parser tests to cover comment skipping behavior.
