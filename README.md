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
- If your precompiled `twice` binary is dynamically linked, system libs must be present (typical on Linux systems).

## What Works Today

- Lexer, Pratt parser, AST
- Evaluator (interpreter semantics)
- Code generation to x86-64 assembly and executable output
- CLI compiler/runner (`cmd/twice`)
- Typed declarations with inference and null-default initialization
- Numeric, string, char, boolean, modulo, and bitwise operators
- Control flow with `if`/`elif`/`else`, `while`, `for`, `loop`, `break`, and `continue`
- String indexing (`str[i]`) returning `char`
- String escapes (`\n`, `\t`, `\r`, `\\`, `\"`, etc.) and template strings using backticks with `${...}`
- Named functions with typed/default parameters and typed returns
- Function literals/anonymous functions in codegen and runtime
- Function calls with positional, named, and mixed arguments
- Function calls before declaration (resolved by codegen)
- Arrays with typed declarations, literals, indexing, mutation, `length()`, and `.length`
- Null-safe access/coalescing with `?.` and `??`
- Union types (`type1||type2`) including array forms like `(int||string)[3]`
- Tuple types and values: `(type1, type2, ...)` with tuple access `value.0`, `value.1`, ...

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
echo 'println(123);' | ./twice -run -
```

Note: `-run` executes `./<output>`, so prefer a relative `-o` value when using `-run`.

Codegen diagnostics include exact source line and column when available.

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

Union type declarations are supported:

```tw
let value: int||string = 1;
value = "twice";
value = 3;
if (value == 3) {
  println("union if works");
};
if (typeofValue(value) == int) {
  println("runtime value is int");
};

let mixed: (int||string)[3] = {1, "two", 3};

let tri: int||string||bool = 1;
tri = "ok";
tri = true;
println(typeof(tri)); // int||string||bool
```

Type declarations (aliases) are also supported:

```tw
type NumOrText = int||string;
type Row = int[2];
type Grid = Row[2];

let value: NumOrText = "ok";
let g: Grid = {{1, 2}, {3, 4}};
println(typeof(value)); // NumOrText
```

### Assignment

```tw
let x = 1;
x = 2;
```

- Reassigning `const` is an error.
- Type constraints are enforced.

### Scope Rules

Twice uses lexical scopes for blocks and loops:

- `let`/`const` declarations are local to the current block.
- Redeclaration checks are scope-local (shadowing outer variables is allowed).
- Assigning an existing outer variable inside a nested block updates that outer variable.
- `for (let i = ...; ...)` keeps `i` local to the `for` scope.
- Standalone blocks `{ ... }` create a temporary scope outside loops/functions.

Example:

```tw
let x = 1;
if (true) {
  let x = 2;
  println(x); // 2
};
println(x);   // 1

if (true) {
  x = 5;
};
println(x);   // 5
```

Temporary scope example:

```tw
{
  let temp = 123;
  fn tempFn(x: int) int { return x + 1; }
  println(tempFn(temp));
}
// temp and tempFn are not visible here
```

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
- `int + string` -> string concatenation
- `string[index]` -> `char`
- `%` with numeric types uses modulo semantics

String indexing example:

```tw
println("Twice"[2]); // 'i'
```

Escapes and template strings:

```tw
let who = "Twice";
println("line1\nline2\tend");
println(`Hello ${who}\n`);
```

Null-safe and coalescing examples:

```tw
let arr: int[3];
println(arr?.length);   // 3
println(arr?.length()); // 3

let fallback2 = arr?.length ?? 0;
println(fallback2); // 3

let fallback = arr?.length() ?? 0;
println(fallback); // 3

let arr2 = {1, 2, 3};
println(arr2.length);    // 3
println(arr2?.length);   // 3
println(arr2?.length()); // 3

println(arr2?.missing);          // null
println(arr2?.missing());        // null
println(arr2?.missing ?? "n/a"); // "n/a"

println(hasField(arr2, "length")); // true
let f: string = "length";
println(hasField(arr2, f));        // true
f = "missing";
println(hasField(arr2, f));        // false
println(hasField("abc", "length"));// true
```

Notes:

- `?.` supports method calls and field reads.
- Missing/unsupported members or methods accessed through `?.` evaluate to `null` instead of erroring.
- `hasField(obj, field)` accepts runtime string values for `field` (not only compile-time string literals).
- `??` only falls back when the left side is `null`.
- Mixing `??` with `&&`/`||` requires parentheses.

### Control Flow

```tw
if (x > 10) {
  println("big");
} elif (x == 10) {
  println("equal");
} else {
  println("small");
};
```

Loops:

```tw
let i = 0;
while (i < 3) {
  i++;
};

let sum = 0;
for (let j = 0; j < 4; j++) {
  sum = sum + j;
};

let control = 0;
for (let k = 0; k < 6; k++) {
  if (k == 2) { continue; };
  if (k == 5) { break; };
  control = control + k;
};

// loop is while(true), use break to exit
loop {
  if (control > 0) { break; };
};
```

Notes:

- `for` form is `for (<init>; <check>; <periodic>) {}`.
- `while` form is `while (<bool>) {}`.
- `loop {}` is equivalent to `while (true) {}`.
- `break;` exits the nearest loop.
- `continue;` skips to the next iteration of the nearest loop.
- Current codegen uses fixed frame-slot allocation for loop-local values and array literals, so loop iterations reuse storage instead of growing stack usage per iteration.

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

Function literals are supported in codegen too:

```tw
let y = 3;
let getY = fn() int { return y; };
println((fn(a: int) int { return a + 1; })(2)); // 3
println(getY()); // 3
```

Higher-arity functions and capture-heavy closures are supported:

```tw
fn sum7(a: int, b: int, c: int, d: int, e: int, f: int, g: int) int {
  return a + b + c + d + e + f + g;
}
println(sum7(1, 2, 3, 4, 5, 6, 7)); // 28

let a1 = 1;
let a2 = 2;
let a3 = 3;
let a4 = 4;
let a5 = 5;
let addCaps = fn(x: int, y: int) int { return a1 + a2 + a3 + a4 + a5 + x + y; };
println(addCaps(6, 7)); // 28
```

Call order is declaration-order independent:

```tw
println(add(3));

fn add(a: int, b: int = 2) int {
  return a + b;
}
```

Notes:

- Parameters may be typed and may have default values.
- Function return type can be declared and is validated.
- Return type can be omitted for no-return functions.
- `return;` returns `null`.
- `return;` is valid only when return type is omitted or explicitly allows `null` (for example `int||null`).
- If return type is `int`, `float`, `bool`, `string`, `char`, etc., use `return <value>;` of a compatible type.
- Codegen resolves named functions independently of source order (you can call before declaration).
- If there is a top-level `fn main()` with zero parameters, it is used as the entrypoint automatically.
- If `main` returns `int`, that value is used as the process exit code.
- If `main` has no return type, process exits with code `0`.
- If `main` has return type `int||null`, returning `null` also exits with code `0`.
- User-defined function calls are not limited to six combined parameters/captures.

Example no-return function:

```tw
fn logValue(x: int) {
  println(x);
  return;
}

fn maybeName(flag: bool) string||null {
  if (flag) {
    return "twice";
  };
  return;
}
```

Example auto-entrypoint `main`:

```tw
fn main() int {
  println("hello from main");
  return 7; // process exit code
}
```

### Arrays

Supported declarations:

```tw
let a: int[3];
let b: int[3] = {1, 2, 3};
let c: int[3] = {};
let grid: int[2][2] = {{1, 2}, {2, 3}};
let pair: (int, string) = ();
```

Indexing and mutation:

```tw
let arr = {1, 2, 3};
println(arr[1]); // 2
arr[1] = 99;
println(arr[1]); // 99

let mixed = {1, "two", 3}; // inferred as (int||string)[3]
println(typeof(mixed));

println(c.length());      // 3
println(pair.0 ?? 7);     // 7
```

Array length method:

```tw
println(arr.length()); // 3
```

Note: array type annotations require explicit sizes in every dimension (for example `int[3]`, `int[2][2]`, `(int||string)[3]`). Empty literals (`{}` / `()`) require type context.

### Tuples

Tuple declaration and literal syntax:

```tw
let a: (int, string, bool) = (1, "x", true);
```

Tuple element access uses numeric dot indices:

```tw
println(a.0); // 1
println(a.1); // "x"
println(a.2); // true
```

Tuples also work with unions and aliases:

```tw
type Pair = (int, string);
type MaybePair = Pair||string;

let v: MaybePair = (1, "x");
println(v.0);      // 1
v = "fallback";
println(typeof(v)); // MaybePair
```

### Builtins

- `print(expr)` writes without a trailing newline
- `println(expr)` writes with a trailing newline
- `print(expr)` and `println(expr)` both support: `int`, `bool`, `float`, `string`, `char`, `null`, `type`
- `typeof(expr)` returns the type name
- `typeofValue(expr)` returns the current value type name (useful for union-typed variables)
- Casts:
  - `int(...)`
  - `float(...)`
  - `string(...)`
  - `char(...)`
  - `bool(...)`

### Runtime Errors

- Runtime failures are reported as: `Runtime error: <message>`
- Runtime messages include source location and a short context snippet when available.
- In `-run` mode, runtime failures exit with non-zero status.

Example:

```tw
fn main() {
  let arr: int[3] = {1,2,3};
  let i = 9;
  println(arr[i]); // Runtime error: array index out of bounds ...
}
```

### Any Type

`any` can hold values of any runtime type.

```tw
let x: any = 3;
println(typeof(x));      // any
println(typeofValue(x)); // int
println(x + 4);          // 7

x = "ok";
println(typeof(x));      // any
println(typeofValue(x)); // string
println(x + "!");        // ok!
```

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

println(s);
println(c);
println(f);
println(n);
println(typeof(n));
println(int(true));
println(char(66));

println("x:" + 7);
println(7 % 4);
println(7.5 % 2.0);
println("Twice"[2]);
println(true ^^ false);
println(5 << 1);
println('A' + 1);

let i = 0;
while (i < 3) {
  i++;
};
println(i);

let sum = 0;
for (let j = 0; j < 4; j++) {
  sum = sum + j;
};
println(sum);

let control = 0;
for (let k = 0; k < 6; k++) {
  if (k == 2) { continue; };
  if (k == 5) { break; };
  control = control + k;
};
println(control);

println(add(5)); // call before declaration

fn add(a: int, b: int = 2) int {
  return a + b;
}

println(add(3));
println(add(a = 3, b = 4));
println(add(3, b = 10));

let arr = {1, 2, 3};
println(arr.length());
println(arr[1]);
arr[1] = 99;
println(arr[1]);

let value: int||string = 1;
value = "twice";
value = 3;
if (value == 3) {
  println("union if works");
};
if (typeofValue(value) == int) {
  println("runtime value is int");
};

let grid: int[2][2] = {{1, 2}, {2, 3}};
println(typeof(grid));
println(grid.length());
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
