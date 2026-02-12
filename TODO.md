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

## 4. Improve `print` Runtime Support

- Handle signed integers in `print_int` (negative values).
- Decide and enforce argument/type constraints for `print`.
- Add validation for wrong argument counts/types.
