# Twice

Twice is a small programming language project.

Current active implementation:

- Go frontend + evaluator
- x86-64 codegen pipeline (`as` + `gcc`/`ld`)

## System Requirements

### Build From Source

Required:

- Go (current stable release recommended)
- GNU assembler: `as`
- One linker toolchain:
  - `gcc` (recommended), or
  - `ld`

Used by the build/run flow:

- `go build` compiles the `twice` CLI
- `as` assembles generated x86-64 assembly
- `gcc`/`ld` links the final executable

### Run an Already Compiled `twice` Binary

Required:

- 64-bit Linux (x86-64)
- No Go toolchain required

Notes:

- The produced programs are native Linux executables.
- If your precompiled `twice` binary is dynamically linked, system libc must be present (typical on Linux systems).

## What Works Today

- Lexer, Pratt parser, AST
- Evaluator (interpreter semantics)
- Code generation to x86-64 assembly and executable output
- CLI compiler/runner (`cmd/twice`)

## Quick Start

### 1. Build the CLI

```bash
go build -o twice ./cmd/twice
```

### 2. Compile a source file

```bash
./twice -o output test.tw
```

### 3. Compile and run immediately

```bash
./twice -run -o output test.tw
```

### 4. Run from stdin

```bash
echo 'print(123);' | ./twice -run -
```

Note: `-run` executes `./<output>`, so prefer a relative `-o` value when using `-run`.

## Language Guide

### Primitive Types

- `int`
- `bool`
- `float`
- `string`
- `char`
- `null`

### Declarations

```tw
let x = 10;
let name: string = "twice";
let maybe: int;      // initialized to null

const limit = 100;
const label: string = "prod";
```

### Assignment

```tw
let x = 1;
x = 2;
```

- Reassigning `const` is an error.
- Type constraints are enforced.

### Operators

Prefix:

- `!`
- `-`

Infix:

- Arithmetic: `+`, `-`, `*`, `/`
- Comparisons: `<`, `>`, `==`, `!=`
- Boolean: `&&`, `||`, `^^`

Current mixed-type behavior includes:

- `int` with `float` arithmetic -> `float`
- `char + int` -> `char`
- `char + char` -> `char`
- `string + int/float/char` -> string concatenation

### Control Flow

```tw
if (x > 10) {
  print("big");
} elif (x == 10) {
  print("equal");
} else {
  print("small");
}
```

### Builtins

- `print(expr)` supports: `int`, `bool`, `float`, `string`, `char`, `null`, `type`
- `typeof(expr)` returns the type name
- Casts:
  - `int(...)`
  - `float(...)`
  - `string(...)`
  - `char(...)`
  - `bool(...)`

### Comments

```tw
// line comment
/* block comment */
```

### Statement Terminators

- Most statements require `;`
- Newlines are not statement terminators
- An expression without trailing `;` becomes an implicit `return`

Example:

```tw
let x = 41;
x + 1
```

The last line is treated as `return x + 1`.

## Example Program

```tw
const banner: string = "Twice";
let n1 = 1 + 2.5;
let next = 'A' + 1;

print(banner);
print(n1);
print(next);
print("type(next): " + typeof(next));
print(true && false);
print(true || false);
print(true ^^ false);

n1
```

## Project Layout

- `cmd/twice` - CLI entrypoint
- `internal/lexer` - tokenization
- `internal/parser` - Pratt parser
- `internal/ast` - AST nodes
- `internal/evaluator` - interpreter semantics
- `internal/codegen` - x86-64 codegen
- `internal/object` - runtime object model
- `internal/token` - token definitions

## Testing

Run all tests:

```bash
go test ./...
```

## Roadmap

See `TODO.md` for planned work (bitwise ops, custom libraries, function codegen expansion).
