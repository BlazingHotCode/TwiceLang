# twice

A small language/compiler project built twice:
- Phase 1 in Go (current implementation)
- Phase 2 in OCaml (in progress)

## Current Status (Go)

The Go pipeline is wired end-to-end:
- Lexer
- Pratt parser
- AST
- x86-64 code generator
- Assembler/linker integration (`as` + `gcc`/`ld`)

## Language Features (Current)

- Integers and booleans
- Prefix operators: `!`, `-`
- Infix operators: `+`, `-`, `*`, `/`, `<`, `>`, `==`, `!=`
- `let` bindings
- `if / elif / else`
- `return`
- Function call syntax parsing
- Builtin codegen support for `print(<expr>)` (single argument)

## Statement Terminators

- `let` and explicit `return` statements require `;`
- Expression statements with `;` are normal statements
- Expression statements without `;` are treated as implicit `return`
  - Example: `x` behaves like `return x`
- Newlines are not statement terminators by themselves

## Build And Run

Build the CLI:

```bash
go build -o twice ./cmd/twice
```

Compile a `.tw` file:

```bash
./twice -o output test.tw
```

Compile and run immediately:

```bash
./twice -run -o output test.tw
```

Read source from stdin:

```bash
echo 'print(123);' | ./twice -run -
```

Note: the CLI currently runs outputs as `./<name>`, so use a relative `-o` path when using `-run`.

## Example

```tw
print(123);
let x = 7;
print(x + 5);
x
```

The last line (`x`) is an implicit return (no semicolon).

## Next Work

- Expand callable function codegen beyond builtin `print`
- Improve diagnostics and parser recovery
- Complete the OCaml implementation track
