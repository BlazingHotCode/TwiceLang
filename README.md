# twice

A small language/compiler project built twice:

- Phase 1 in Go (current implementation)

## Current Status (Go)

The Go pipeline is wired end-to-end:

- Lexer
- Pratt parser
- AST
- x86-64 code generator
- Assembler/linker integration (`as` + `gcc`/`ld`)

## Language Features (Current)

- Primitive values: `int`, `bool`, `float`, `string`, `char`, `null`
- Prefix operators: `!`, `-`
- Infix operators: `+`, `-`, `*`, `/`, `<`, `>`, `==`, `!=`
- `let` and `const` bindings
- Optional type annotations:
  - `let name = value;`
  - `let name: type = value;`
  - `let name: type;` (initialized as `null`)
  - `const name = value;`
  - `const name: type = value;`
- Variable reassignment (`x = value;`) with const protection and type checks
- `if / elif / else`
- `return`
- Builtins:
  - `print(<expr>)` for `int`, `bool`, `float`, `string`, `char`, `null`, and `type`
  - `typeof(<expr>)` returns the static/runtime type name
  - Casts: `int(...)`, `float(...)`, `string(...)`, `char(...)`, `bool(...)`
- Line and block comments:
  - `// comment`
  - `/* block comment */`

## Statement Terminators

- Most statements require `;` (including `let`, `const`, assignment, `return`, and expression statements)
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
// Single-line and block comments work.
/* test program */
const banner: string = "Twice";
let n: int;
n = 41;
print(banner);
print(n + 1);
print(typeof(n));
print(float(3));
print(char(65));
n
```

The last line (`n`) is an implicit return (no semicolon), so process exit code is `41`.

## Next Work

- Expand callable function codegen beyond builtin `print`
- Improve parser recovery
- Complete the OCaml implementation track
