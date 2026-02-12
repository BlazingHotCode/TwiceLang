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
- Typed declarations with inference and null-default initialization
- Numeric, string, char, boolean, modulo, and bitwise operators
- Named functions with typed/default parameters and typed returns
- Function calls with positional, named, and mixed arguments
- Function calls before declaration (resolved by codegen)
- Arrays with typed declarations, literals, indexing, mutation, and `length()`

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

- Arithmetic: `+`, `-`, `*`, `/`, `%` (`%` is modulo)
- Comparisons: `<`, `>`, `==`, `!=`
- Boolean: `&&`, `||`, `^^`
- Bitwise (int): `&`, `|`, `^`, `<<`, `>>`

Assignment forms:

- `=`
- `+=`, `-=`, `*=`, `/=`, `%=` (compound assignment)
- `++`, `--` (int variables only)

Current mixed-type behavior includes:

- `int` with `float` arithmetic -> `float`
- `char + int` -> `char`
- `char + char` -> `char`
- `string + int/float/char` -> string concatenation
- `%` with numeric types uses modulo semantics

### Control Flow

```tw
if (x > 10) {
  print("big");
} elif (x == 10) {
  print("equal");
} else {
  print("small");
};
```

### Functions

Supported declaration syntax:

```tw
fn add(a: int, b: int = 2) int {
  return a + b;
}
```

Supported call styles:

```tw
add(3);             // positional + default
add(a = 3, b = 4);  // named arguments
add(3, b = 10);     // mixed positional + named
```

Call order is declaration-order independent:

```tw
print(add(3));

fn add(a: int, b: int = 2) int {
  return a + b;
}
```

Notes:

- Parameters may be typed and may have default values.
- Function return type can be declared and is validated.
- Codegen resolves named functions independently of source order (you can call before declaration).

### Arrays

Supported declarations:

```tw
let a: int[3];
let b: int[] = {1, 2, 3};
let grid: int[][2] = {{1}, {2, 3}};
```

Indexing and mutation:

```tw
let arr = {1, 2, 3};
print(arr[1]); // 2
arr[1] = 99;
print(arr[1]); // 99
```

Array length method:

```tw
print(arr.length()); // 3
```

Note: in current codegen, `length()` requires a compile-time-known array size (for example `int[3]` or `int[][2]`).

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
let s: string = "hello";
let c = 'A';
let f = 3.14;
let n: string;

print(s);
print(c);
print(f);
print(n);
print(typeof(n));
print(int(true));
print(char(66));

print("x:" + 7);
print(7 % 4);
print(7.5 % 2.0);
print(true ^^ false);
print(5 << 1);
print('A' + 1);

print(add(5)); // call before declaration

fn add(a: int, b: int = 2) int {
  return a + b;
}

print(add(3));
print(add(a = 3, b = 4));
print(add(3, b = 10));

let arr = {1, 2, 3};
print(arr.length());
print(arr[1]);
arr[1] = 99;
print(arr[1]);

let grid: int[][2] = {{1}, {2, 3}};
print(typeof(grid));
print(grid.length());
```

See `test.tw` for a fuller feature walkthrough.

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

See `TODO.md` for planned work, currently focused on:

- Custom libraries (for example, a `math` library)
- Continued evaluator/codegen parity polish
