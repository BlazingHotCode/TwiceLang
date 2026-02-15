package codegen

import (
	"fmt"
	"strings"

	"twice/internal/ast"
)

// CodeGen holds the state for code generation
type CodeGen struct {
	output             strings.Builder
	funcDefs           strings.Builder
	labelCount         int
	exitLabel          string
	normalExit         string
	variables          map[string]int // name -> stack offset
	constVars          map[string]bool
	varTypes           map[string]valueType
	varDeclared        map[string]valueType
	varTypeNames       map[string]string
	varDeclaredNames   map[string]string
	varValueTypeName   map[string]string
	varIsNull          map[string]bool
	varArrayLen        map[string]int
	intVals            map[string]int64
	charVals           map[string]rune
	stringVals         map[string]string
	floatVals          map[string]float64
	stringLits         map[string]string
	stackOffset        int // current stack position
	maxStackOffset     int
	functions          map[string]*compiledFunction
	funcByName         map[string]string
	funcStmtKeys       map[*ast.FunctionStatement]string
	funcLitKeys        map[*ast.FunctionLiteral]string
	varFuncs           map[string]string
	typeAliases        map[string]string
	genericTypeAliases map[string]genericTypeAlias
	structDecls        map[string]*ast.StructStatement
	currentFn          string
	nextAnonFn         int
	inFunction         bool
	funcRetLbl         string
	funcRetType        valueType
	funcRetTypeName    string
	loopBreakLabels    []string
	loopContLabels     []string
	arraySlots         map[*ast.ArrayLiteral]int
	tupleSlots         map[*ast.TupleLiteral]int
	scopeDecls         []map[string]struct{}
	typeScopeDecls     []map[string]struct{}
	inferTypeCache     map[ast.Expression]valueType
	inferNameCache     map[ast.Expression]string
	errors             []CodegenError
}

type compiledFunction struct {
	Key              string
	Name             string
	Label            string
	Literal          *ast.FunctionLiteral
	Captures         []string
	CaptureTypeNames []string
	TypeArgMap       map[string]string
}

type genericTypeAlias struct {
	TypeParams []string
	TypeName   string
}

type valueType int

const (
	typeUnknown valueType = iota
	typeInt
	typeBool
	typeFloat
	typeString
	typeChar
	typeNull
	typeType
	typeAny
	typeArray
)

type CodegenError struct {
	Message string
	Context string
	Line    int
	Column  int
}

// New creates a new code generator
func New() *CodeGen {
	return &CodeGen{
		variables:          make(map[string]int),
		constVars:          make(map[string]bool),
		varTypes:           make(map[string]valueType),
		varDeclared:        make(map[string]valueType),
		varTypeNames:       make(map[string]string),
		varDeclaredNames:   make(map[string]string),
		varValueTypeName:   make(map[string]string),
		varIsNull:          make(map[string]bool),
		varArrayLen:        make(map[string]int),
		intVals:            make(map[string]int64),
		charVals:           make(map[string]rune),
		stringVals:         make(map[string]string),
		floatVals:          make(map[string]float64),
		stringLits:         make(map[string]string),
		functions:          make(map[string]*compiledFunction),
		funcByName:         make(map[string]string),
		funcStmtKeys:       make(map[*ast.FunctionStatement]string),
		funcLitKeys:        make(map[*ast.FunctionLiteral]string),
		varFuncs:           make(map[string]string),
		typeAliases:        make(map[string]string),
		genericTypeAliases: make(map[string]genericTypeAlias),
		structDecls:        make(map[string]*ast.StructStatement),
		stackOffset:        0,
		maxStackOffset:     0,
		arraySlots:         make(map[*ast.ArrayLiteral]int),
		tupleSlots:         make(map[*ast.TupleLiteral]int),
		scopeDecls:         []map[string]struct{}{},
		typeScopeDecls:     []map[string]struct{}{},
		inferTypeCache:     make(map[ast.Expression]valueType),
		inferNameCache:     make(map[ast.Expression]string),
		errors:             []CodegenError{},
	}
}

// Generate produces x86-64 assembly from a program
func (cg *CodeGen) Generate(program *ast.Program) string {
	cg.reset()
	cg.exitLabel = cg.newLabel()
	cg.normalExit = cg.newLabel()
	cg.collectFunctions(program)
	cg.semanticCheck(program)
	cg.symbolCheck(program)
	if len(cg.errors) > 0 {
		return ""
	}
	cg.annotateTypesPass(program)
	cg.emitHeader()

	// Generate code for each statement
	for _, stmt := range program.Statements {
		cg.generateStatement(stmt)
	}
	cg.maybeAutoRunMain(program)
	cg.generateFunctionDefinitions()

	cg.emitFooter()
	asm := cg.output.String()
	asm = strings.ReplaceAll(asm, "__STACK_ALLOC_MAIN__", cg.stackAllocLine())
	return asm
}

func (cg *CodeGen) maybeAutoRunMain(program *ast.Program) {
	if program == nil || hasTopLevelMainCall(program) {
		return
	}
	key, ok := cg.funcByName["main"]
	if !ok {
		return
	}
	fn, ok := cg.functions[key]
	if !ok || fn == nil || fn.Literal == nil {
		return
	}
	if len(fn.Literal.Parameters) != 0 {
		return
	}

	cg.emit("    call %s", fn.Label)
	if fn.Literal.ReturnType == "" || fn.Literal.ReturnType == "null" {
		cg.emit("    xor %%rdi, %%rdi")
		cg.emit("    jmp %s", cg.exitLabel)
		return
	}
	if cg.parseTypeName(fn.Literal.ReturnType) == typeInt {
		cg.emit("    mov %%rax, %%rdi")
		cg.emit("    jmp %s", cg.exitLabel)
		return
	}
	// Non-int/non-empty return types are not auto-run entrypoints.
}

func hasTopLevelMainCall(program *ast.Program) bool {
	if program == nil {
		return false
	}
	for _, st := range program.Statements {
		switch s := st.(type) {
		case *ast.ExpressionStatement:
			if isMainCallExpr(s.Expression) {
				return true
			}
		case *ast.ReturnStatement:
			if isMainCallExpr(s.ReturnValue) {
				return true
			}
		}
	}
	return false
}

func isMainCallExpr(expr ast.Expression) bool {
	ce, ok := expr.(*ast.CallExpression)
	if !ok || ce == nil {
		return false
	}
	id, ok := ce.Function.(*ast.Identifier)
	return ok && id != nil && id.Value == "main"
}

// emit adds a line of assembly
func (cg *CodeGen) emit(format string, args ...interface{}) {
	cg.output.WriteString(fmt.Sprintf(format, args...))
	cg.output.WriteString("\n")
}

// emitHeader outputs assembly preamble
func (cg *CodeGen) emitHeader() {
	cg.emit(".global _start")
	cg.emit(".text")
	cg.emit("")

	// Print integer function using write syscall
	// Converts number to string and prints to stdout
	cg.emit("# Function: print_int")
	cg.emit("# Input: rax = number to print")
	cg.emit("print_int:")
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")
	cg.emit("    push %%rbx             # preserve callee-saved register")
	cg.emit("    sub $40, %%rsp         # local buffer space")
	cg.emit("    mov %%rax, %%rcx        # save number")
	cg.emit("    lea -1(%%rbp), %%rsi    # buffer end (write backwards)")
	cg.emit("    mov $0, %%rbx          # digit count")
	cg.emit("    xor %%r8d, %%r8d        # sign flag: 0 = non-negative")
	cg.emit("    test %%rcx, %%rcx")
	cg.emit("    jge print_int_abs_ready")
	cg.emit("    neg %%rcx")
	cg.emit("    mov $1, %%r8b")
	cg.emit("print_int_abs_ready:")
	cg.emit("")
	cg.emit("print_int_loop:")
	cg.emit("    xor %%rdx, %%rdx")
	cg.emit("    mov %%rcx, %%rax        # dividend")
	cg.emit("    mov $10, %%rdi")
	cg.emit("    div %%rdi              # rax = number/10, rdx = number%%10")
	cg.emit("    mov %%rax, %%rcx        # next number")
	cg.emit("    add $48, %%dl          # convert remainder to ASCII")
	cg.emit("    mov %%dl, %%al")
	cg.emit("    movb %%al, (%%rsi)")
	cg.emit("    dec %%rsi")
	cg.emit("    inc %%rbx")
	cg.emit("    test %%rcx, %%rcx")
	cg.emit("    jnz print_int_loop")
	cg.emit("")
	cg.emit("    test %%r8b, %%r8b")
	cg.emit("    jz print_int_write")
	cg.emit("    movb $45, (%%rsi)      # '-' sign")
	cg.emit("    dec %%rsi")
	cg.emit("    inc %%rbx")
	cg.emit("")
	cg.emit("print_int_write:")
	cg.emit("    inc %%rsi              # point to first digit")
	cg.emit("    mov $1, %%rax          # syscall: write")
	cg.emit("    mov $1, %%rdi          # fd: stdout")
	cg.emit("    mov %%rbx, %%rdx        # length")
	cg.emit("    syscall")
	cg.emit("")
	cg.emit("    add $40, %%rsp")
	cg.emit("    pop %%rbx")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("")

	// Print boolean function using write syscall
	// Input: rax = 0/1
	cg.emit("# Function: print_bool")
	cg.emit("# Input: rax = boolean (0/1)")
	cg.emit("print_bool:")
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")
	cg.emit("    cmp $0, %%rax")
	cg.emit("    je print_bool_false")
	cg.emit("    lea bool_true(%%rip), %%rsi")
	cg.emit("    mov $4, %%rdx")
	cg.emit("    jmp print_bool_write")
	cg.emit("print_bool_false:")
	cg.emit("    lea bool_false(%%rip), %%rsi")
	cg.emit("    mov $5, %%rdx")
	cg.emit("print_bool_write:")
	cg.emit("    mov $1, %%rax")
	cg.emit("    mov $1, %%rdi")
	cg.emit("    syscall")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("")

	// Print C-string helper.
	// Input: rax = pointer to null-terminated string
	cg.emit("# Function: print_cstr")
	cg.emit("print_cstr:")
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")
	cg.emit("    mov %%rax, %%rsi")
	cg.emit("    xor %%rdx, %%rdx")
	cg.emit("print_cstr_len_loop:")
	cg.emit("    cmpb $0, (%%rsi,%%rdx,1)")
	cg.emit("    je print_cstr_write")
	cg.emit("    inc %%rdx")
	cg.emit("    jmp print_cstr_len_loop")
	cg.emit("print_cstr_write:")
	cg.emit("    mov $1, %%rax")
	cg.emit("    mov $1, %%rdi")
	cg.emit("    syscall")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("")

	// Print C-string helper to stderr.
	// Input: rax = pointer to null-terminated string
	cg.emit("print_cstr_stderr:")
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")
	cg.emit("    mov %%rax, %%rsi")
	cg.emit("    xor %%rdx, %%rdx")
	cg.emit("print_cstr_stderr_len_loop:")
	cg.emit("    cmpb $0, (%%rsi,%%rdx,1)")
	cg.emit("    je print_cstr_stderr_write")
	cg.emit("    inc %%rdx")
	cg.emit("    jmp print_cstr_stderr_len_loop")
	cg.emit("print_cstr_stderr_write:")
	cg.emit("    mov $1, %%rax")
	cg.emit("    mov $2, %%rdi")
	cg.emit("    syscall")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("")

	// Runtime failure helper: writes message to stderr and exits with code 1.
	// Input: rax = pointer to null-terminated error message
	cg.emit("runtime_fail:")
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")
	cg.emit("    call print_cstr_stderr")
	cg.emit("    lea newline_char(%%rip), %%rsi")
	cg.emit("    mov $1, %%rdx")
	cg.emit("    mov $1, %%rax")
	cg.emit("    mov $2, %%rdi")
	cg.emit("    syscall")
	cg.emit("    mov $60, %%rax")
	cg.emit("    mov $1, %%rdi")
	cg.emit("    syscall")
	cg.emit("")

	// Compare two null-terminated strings.
	// Input: rdi = left c-string, rsi = right c-string
	// Output: rax = 1 if equal, 0 otherwise
	cg.emit("cstr_eq:")
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")
	cg.emit("cstr_eq_loop:")
	cg.emit("    movzbq (%%rdi), %%rax")
	cg.emit("    movzbq (%%rsi), %%rcx")
	cg.emit("    cmp %%rcx, %%rax")
	cg.emit("    jne cstr_eq_false")
	cg.emit("    test %%rax, %%rax")
	cg.emit("    je cstr_eq_true")
	cg.emit("    inc %%rdi")
	cg.emit("    inc %%rsi")
	cg.emit("    jmp cstr_eq_loop")
	cg.emit("cstr_eq_true:")
	cg.emit("    mov $1, %%rax")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("cstr_eq_false:")
	cg.emit("    mov $0, %%rax")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("")

	// Print char helper.
	// Input: rax = codepoint (low byte used)
	cg.emit("# Function: print_char")
	cg.emit("print_char:")
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")
	cg.emit("    sub $16, %%rsp")
	cg.emit("    movb %%al, -1(%%rbp)")
	cg.emit("    lea -1(%%rbp), %%rsi")
	cg.emit("    mov $1, %%rdx")
	cg.emit("    mov $1, %%rax")
	cg.emit("    mov $1, %%rdi")
	cg.emit("    syscall")
	cg.emit("    add $16, %%rsp")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("")

	// Write newline helper.
	cg.emit("print_newline:")
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")
	cg.emit("    lea newline_char(%%rip), %%rsi")
	cg.emit("    mov $1, %%rdx")
	cg.emit("    mov $1, %%rax")
	cg.emit("    mov $1, %%rdi")
	cg.emit("    syscall")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("")

	// println wrappers.
	cg.emit("println_int:")
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")
	cg.emit("    call print_int")
	cg.emit("    call print_newline")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("")

	cg.emit("println_bool:")
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")
	cg.emit("    call print_bool")
	cg.emit("    call print_newline")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("")

	cg.emit("println_cstr:")
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")
	cg.emit("    call print_cstr")
	cg.emit("    call print_newline")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("")

	cg.emit("println_char:")
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")
	cg.emit("    call print_char")
	cg.emit("    call print_newline")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("")

	// Concatenate int + c-string into reusable buffer.
	// Input: rax = int value, rdx = c-string pointer
	// Output: rax = pointer to concatenated null-terminated string
	cg.emit("concat_int_cstr:")
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")
	cg.emit("    push %%rbx")
	cg.emit("    push %%r12")
	cg.emit("    push %%r13")
	cg.emit("    sub $80, %%rsp")
	cg.emit("    lea concat_buf(%%rip), %%r12")
	cg.emit("    mov %%r12, %%r13")
	cg.emit("    mov %%rdx, %%r11")
	cg.emit("    mov %%rax, %%rcx")
	cg.emit("    xor %%r8d, %%r8d")
	cg.emit("    test %%rcx, %%rcx")
	cg.emit("    jge concat_int_abs_ready")
	cg.emit("    neg %%rcx")
	cg.emit("    mov $1, %%r8b")
	cg.emit("concat_int_abs_ready:")
	cg.emit("    lea -1(%%rbp), %%rsi")
	cg.emit("    mov $0, %%rbx")
	cg.emit("concat_int_loop:")
	cg.emit("    xor %%rdx, %%rdx")
	cg.emit("    mov %%rcx, %%rax")
	cg.emit("    mov $10, %%rdi")
	cg.emit("    div %%rdi")
	cg.emit("    mov %%rax, %%rcx")
	cg.emit("    add $48, %%dl")
	cg.emit("    movb %%dl, (%%rsi)")
	cg.emit("    dec %%rsi")
	cg.emit("    inc %%rbx")
	cg.emit("    test %%rcx, %%rcx")
	cg.emit("    jnz concat_int_loop")
	cg.emit("    test %%r8b, %%r8b")
	cg.emit("    jz concat_int_copy_digits")
	cg.emit("    movb $45, (%%rsi)")
	cg.emit("    dec %%rsi")
	cg.emit("    inc %%rbx")
	cg.emit("concat_int_copy_digits:")
	cg.emit("    inc %%rsi")
	cg.emit("concat_int_digit_copy_loop:")
	cg.emit("    test %%rbx, %%rbx")
	cg.emit("    jz concat_int_copy_suffix")
	cg.emit("    movb (%%rsi), %%al")
	cg.emit("    movb %%al, (%%r13)")
	cg.emit("    inc %%rsi")
	cg.emit("    inc %%r13")
	cg.emit("    dec %%rbx")
	cg.emit("    jmp concat_int_digit_copy_loop")
	cg.emit("concat_int_copy_suffix:")
	cg.emit("    mov %%r11, %%rsi")
	cg.emit("concat_int_suffix_loop:")
	cg.emit("    movb (%%rsi), %%al")
	cg.emit("    test %%al, %%al")
	cg.emit("    je concat_int_suffix_done")
	cg.emit("concat_int_suffix_copy:")
	cg.emit("    movb %%al, (%%r13)")
	cg.emit("    inc %%rsi")
	cg.emit("    inc %%r13")
	cg.emit("    jmp concat_int_suffix_loop")
	cg.emit("concat_int_suffix_done:")
	cg.emit("    movb $0, (%%r13)")
	cg.emit("    mov %%r12, %%rax")
	cg.emit("    add $80, %%rsp")
	cg.emit("    pop %%r13")
	cg.emit("    pop %%r12")
	cg.emit("    pop %%rbx")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("")

	// Concatenate c-string + int into reusable buffer.
	// Input: rax = c-string pointer, rdx = int value
	// Output: rax = pointer to concatenated null-terminated string
	cg.emit("concat_cstr_int:")
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")
	cg.emit("    push %%rbx")
	cg.emit("    push %%r12")
	cg.emit("    push %%r13")
	cg.emit("    sub $80, %%rsp")
	cg.emit("    lea concat_buf(%%rip), %%r12")
	cg.emit("    mov %%r12, %%r13")
	cg.emit("    mov %%rax, %%r10")
	cg.emit("concat_cstr_copy_prefix:")
	cg.emit("    movb (%%r10), %%al")
	cg.emit("    test %%al, %%al")
	cg.emit("    je concat_cstr_prefix_done")
	cg.emit("concat_cstr_prefix_copy:")
	cg.emit("    movb %%al, (%%r13)")
	cg.emit("    inc %%r10")
	cg.emit("    inc %%r13")
	cg.emit("    jmp concat_cstr_copy_prefix")
	cg.emit("concat_cstr_prefix_done:")
	cg.emit("    mov %%rdx, %%rcx")
	cg.emit("    xor %%r8d, %%r8d")
	cg.emit("    test %%rcx, %%rcx")
	cg.emit("    jge concat_cstr_abs_ready")
	cg.emit("    neg %%rcx")
	cg.emit("    mov $1, %%r8b")
	cg.emit("concat_cstr_abs_ready:")
	cg.emit("    lea -1(%%rbp), %%rsi")
	cg.emit("    mov $0, %%rbx")
	cg.emit("concat_cstr_int_loop:")
	cg.emit("    xor %%rdx, %%rdx")
	cg.emit("    mov %%rcx, %%rax")
	cg.emit("    mov $10, %%rdi")
	cg.emit("    div %%rdi")
	cg.emit("    mov %%rax, %%rcx")
	cg.emit("    add $48, %%dl")
	cg.emit("    movb %%dl, (%%rsi)")
	cg.emit("    dec %%rsi")
	cg.emit("    inc %%rbx")
	cg.emit("    test %%rcx, %%rcx")
	cg.emit("    jnz concat_cstr_int_loop")
	cg.emit("    test %%r8b, %%r8b")
	cg.emit("    jz concat_cstr_copy_digits")
	cg.emit("    movb $45, (%%rsi)")
	cg.emit("    dec %%rsi")
	cg.emit("    inc %%rbx")
	cg.emit("concat_cstr_copy_digits:")
	cg.emit("    inc %%rsi")
	cg.emit("concat_cstr_digit_copy_loop:")
	cg.emit("    test %%rbx, %%rbx")
	cg.emit("    jz concat_cstr_done")
	cg.emit("    movb (%%rsi), %%al")
	cg.emit("    movb %%al, (%%r13)")
	cg.emit("    inc %%rsi")
	cg.emit("    inc %%r13")
	cg.emit("    dec %%rbx")
	cg.emit("    jmp concat_cstr_digit_copy_loop")
	cg.emit("concat_cstr_done:")
	cg.emit("    movb $0, (%%r13)")
	cg.emit("    mov %%r12, %%rax")
	cg.emit("    add $80, %%rsp")
	cg.emit("    pop %%r13")
	cg.emit("    pop %%r12")
	cg.emit("    pop %%rbx")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("")

	// Concatenate c-string + c-string into reusable buffer.
	// Input: rax = left c-string pointer, rdx = right c-string pointer
	// Output: rax = pointer to concatenated null-terminated string
	cg.emit("concat_cstr_cstr:")
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")
	cg.emit("    push %%r12")
	cg.emit("    push %%r13")
	cg.emit("    lea concat_buf(%%rip), %%r12")
	cg.emit("    mov %%r12, %%r13")
	cg.emit("    mov %%rax, %%r10")
	cg.emit("concat_cstr_cstr_copy_left:")
	cg.emit("    movb (%%r10), %%al")
	cg.emit("    test %%al, %%al")
	cg.emit("    je concat_cstr_cstr_left_done")
	cg.emit("concat_cstr_cstr_copy_left_char:")
	cg.emit("    movb %%al, (%%r13)")
	cg.emit("    inc %%r10")
	cg.emit("    inc %%r13")
	cg.emit("    jmp concat_cstr_cstr_copy_left")
	cg.emit("concat_cstr_cstr_left_done:")
	cg.emit("    mov %%rdx, %%r11")
	cg.emit("concat_cstr_cstr_copy_right:")
	cg.emit("    movb (%%r11), %%al")
	cg.emit("    test %%al, %%al")
	cg.emit("    je concat_cstr_cstr_right_done")
	cg.emit("concat_cstr_cstr_copy_right_char:")
	cg.emit("    movb %%al, (%%r13)")
	cg.emit("    inc %%r11")
	cg.emit("    inc %%r13")
	cg.emit("    jmp concat_cstr_cstr_copy_right")
	cg.emit("concat_cstr_cstr_right_done:")
	cg.emit("    movb $0, (%%r13)")
	cg.emit("    mov %%r12, %%rax")
	cg.emit("    pop %%r13")
	cg.emit("    pop %%r12")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("")

	// Allocate bytes from a fixed runtime pool.
	// Input: rdi = bytes requested
	// Output: rax = pointer
	cg.emit("list_alloc:")
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")
	cg.emit("    mov %%rdi, %%rcx")
	cg.emit("    add $7, %%rcx")
	cg.emit("    and $-8, %%rcx")
	cg.emit("    mov list_pool_ptr(%%rip), %%rax")
	cg.emit("    lea (%%rax,%%rcx), %%rdx")
	cg.emit("    lea list_pool_end(%%rip), %%r8")
	cg.emit("    cmp %%r8, %%rdx")
	cg.emit("    jbe list_alloc_ok")
	cg.emit("    lea runtime_err_out_of_memory(%%rip), %%rax")
	cg.emit("    call runtime_fail")
	cg.emit("list_alloc_ok:")
	cg.emit("    mov %%rdx, list_pool_ptr(%%rip)")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("")

	// Create a new list object.
	// Input: rdi = initial capacity hint
	// Output: rax = list*
	cg.emit("list_new:")
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")
	cg.emit("    push %%rbx")
	cg.emit("    mov %%rdi, %%rcx")
	cg.emit("    cmp $0, %%rcx")
	cg.emit("    jge list_new_nonneg")
	cg.emit("    xor %%rcx, %%rcx")
	cg.emit("list_new_nonneg:")
	cg.emit("    cmp $4, %%rcx")
	cg.emit("    jge list_new_cap_ok")
	cg.emit("    mov $4, %%rcx")
	cg.emit("list_new_cap_ok:")
	cg.emit("    mov $24, %%rdi")
	cg.emit("    call list_alloc")
	cg.emit("    mov %%rax, %%rbx")
	cg.emit("    mov %%rcx, %%rdi")
	cg.emit("    imul $8, %%rdi, %%rdi")
	cg.emit("    call list_alloc")
	cg.emit("    mov $0, (%%rbx)")
	cg.emit("    mov %%rcx, 8(%%rbx)")
	cg.emit("    mov %%rax, 16(%%rbx)")
	cg.emit("    mov %%rbx, %%rax")
	cg.emit("    pop %%rbx")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("")

	// Grow list capacity to at least rsi.
	// Input: rdi = list*, rsi = minimum required capacity
	cg.emit("list_grow:")
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")
	cg.emit("    push %%rbx")
	cg.emit("    push %%r12")
	cg.emit("    push %%r13")
	cg.emit("    mov %%rdi, %%rbx")
	cg.emit("    mov 8(%%rbx), %%rcx")
	cg.emit("    lea (%%rcx,%%rcx), %%r12")
	cg.emit("    cmp $4, %%r12")
	cg.emit("    jge list_grow_min4")
	cg.emit("    mov $4, %%r12")
	cg.emit("list_grow_min4:")
	cg.emit("    cmp %%rsi, %%r12")
	cg.emit("    jge list_grow_cap_ok")
	cg.emit("    mov %%rsi, %%r12")
	cg.emit("list_grow_cap_ok:")
	cg.emit("    mov %%r12, %%rdi")
	cg.emit("    imul $8, %%rdi, %%rdi")
	cg.emit("    call list_alloc")
	cg.emit("    mov %%rax, %%r13")
	cg.emit("    mov 16(%%rbx), %%r8")
	cg.emit("    mov (%%rbx), %%r9")
	cg.emit("    xor %%r10, %%r10")
	cg.emit("list_grow_copy_loop:")
	cg.emit("    cmp %%r9, %%r10")
	cg.emit("    jge list_grow_copy_done")
	cg.emit("    mov (%%r8,%%r10,8), %%r11")
	cg.emit("    mov %%r11, (%%r13,%%r10,8)")
	cg.emit("    inc %%r10")
	cg.emit("    jmp list_grow_copy_loop")
	cg.emit("list_grow_copy_done:")
	cg.emit("    mov %%r13, 16(%%rbx)")
	cg.emit("    mov %%r12, 8(%%rbx)")
	cg.emit("    pop %%r13")
	cg.emit("    pop %%r12")
	cg.emit("    pop %%rbx")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("")

	// Append value into list.
	// Input: rdi = list*, rsi = value
	cg.emit("list_append:")
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")
	cg.emit("    push %%rbx")
	cg.emit("    mov %%rdi, %%rbx")
	cg.emit("    mov (%%rbx), %%rax")
	cg.emit("    mov 8(%%rbx), %%rcx")
	cg.emit("    cmp %%rcx, %%rax")
	cg.emit("    jl list_append_store")
	cg.emit("    push %%rsi")
	cg.emit("    lea 1(%%rax), %%rsi")
	cg.emit("    mov %%rbx, %%rdi")
	cg.emit("    call list_grow")
	cg.emit("    pop %%rsi")
	cg.emit("    mov (%%rbx), %%rax")
	cg.emit("list_append_store:")
	cg.emit("    mov 16(%%rbx), %%rcx")
	cg.emit("    mov %%rsi, (%%rcx,%%rax,8)")
	cg.emit("    inc %%rax")
	cg.emit("    mov %%rax, (%%rbx)")
	cg.emit("    lea null_lit(%%rip), %%rax")
	cg.emit("    pop %%rbx")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("")

	// Get list element.
	// Input: rdi = list*, rsi = index
	// Output: rax = value
	cg.emit("list_get:")
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")
	cg.emit("    cmp $0, %%rsi")
	cg.emit("    jl list_get_oob")
	cg.emit("    mov (%%rdi), %%rcx")
	cg.emit("    cmp %%rcx, %%rsi")
	cg.emit("    jge list_get_oob")
	cg.emit("    mov 16(%%rdi), %%rax")
	cg.emit("    mov (%%rax,%%rsi,8), %%rax")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("list_get_oob:")
	cg.emit("    lea runtime_err_list_oob(%%rip), %%rax")
	cg.emit("    call runtime_fail")
	cg.emit("")

	// Set list element.
	// Input: rdi = list*, rsi = index, rdx = value
	cg.emit("list_set:")
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")
	cg.emit("    cmp $0, %%rsi")
	cg.emit("    jl list_set_oob")
	cg.emit("    mov (%%rdi), %%rcx")
	cg.emit("    cmp %%rcx, %%rsi")
	cg.emit("    jge list_set_oob")
	cg.emit("    mov 16(%%rdi), %%rax")
	cg.emit("    mov %%rdx, (%%rax,%%rsi,8)")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("list_set_oob:")
	cg.emit("    lea runtime_err_list_oob(%%rip), %%rax")
	cg.emit("    call runtime_fail")
	cg.emit("")

	// Pop last list element.
	// Input: rdi = list*
	// Output: rax = value or null
	cg.emit("list_pop:")
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")
	cg.emit("    mov (%%rdi), %%rcx")
	cg.emit("    cmp $0, %%rcx")
	cg.emit("    je list_pop_empty")
	cg.emit("    dec %%rcx")
	cg.emit("    mov %%rcx, (%%rdi)")
	cg.emit("    mov 16(%%rdi), %%rax")
	cg.emit("    mov (%%rax,%%rcx,8), %%rax")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("list_pop_empty:")
	cg.emit("    lea null_lit(%%rip), %%rax")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("")

	// Clear list.
	// Input: rdi = list*
	cg.emit("list_clear:")
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")
	cg.emit("    mov $0, (%%rdi)")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("")

	// Remove element at index.
	// Input: rdi = list*, rsi = index
	// Output: rax = removed element
	cg.emit("list_remove:")
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")
	cg.emit("    cmp $0, %%rsi")
	cg.emit("    jl list_remove_oob")
	cg.emit("    mov (%%rdi), %%rcx")
	cg.emit("    cmp %%rcx, %%rsi")
	cg.emit("    jge list_remove_oob")
	cg.emit("    mov 16(%%rdi), %%r8")
	cg.emit("    mov (%%r8,%%rsi,8), %%rax")
	cg.emit("    mov %%rsi, %%r9")
	cg.emit("list_remove_shift:")
	cg.emit("    lea 1(%%r9), %%r10")
	cg.emit("    cmp %%rcx, %%r10")
	cg.emit("    jge list_remove_done_shift")
	cg.emit("    mov (%%r8,%%r10,8), %%r11")
	cg.emit("    mov %%r11, (%%r8,%%r9,8)")
	cg.emit("    inc %%r9")
	cg.emit("    jmp list_remove_shift")
	cg.emit("list_remove_done_shift:")
	cg.emit("    dec %%rcx")
	cg.emit("    mov %%rcx, (%%rdi)")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("list_remove_oob:")
	cg.emit("    lea runtime_err_list_oob(%%rip), %%rax")
	cg.emit("    call runtime_fail")
	cg.emit("")

	// Insert value at index.
	// Input: rdi = list*, rsi = index, rdx = value
	cg.emit("list_insert:")
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")
	cg.emit("    push %%rbx")
	cg.emit("    mov %%rdi, %%rbx")
	cg.emit("    cmp $0, %%rsi")
	cg.emit("    jl list_insert_oob")
	cg.emit("    mov (%%rbx), %%rcx")
	cg.emit("    cmp %%rcx, %%rsi")
	cg.emit("    jg list_insert_oob")
	cg.emit("    mov %%rdx, %%r11")
	cg.emit("    mov 8(%%rbx), %%r8")
	cg.emit("    cmp %%r8, %%rcx")
	cg.emit("    jl list_insert_has_cap")
	cg.emit("    push %%rsi")
	cg.emit("    push %%r11")
	cg.emit("    lea 1(%%rcx), %%rsi")
	cg.emit("    mov %%rbx, %%rdi")
	cg.emit("    call list_grow")
	cg.emit("    pop %%r11")
	cg.emit("    pop %%rsi")
	cg.emit("    mov (%%rbx), %%rcx")
	cg.emit("list_insert_has_cap:")
	cg.emit("    mov 16(%%rbx), %%r8")
	cg.emit("    mov %%rcx, %%r9")
	cg.emit("list_insert_shift:")
	cg.emit("    cmp %%rsi, %%r9")
	cg.emit("    jle list_insert_store")
	cg.emit("    lea -1(%%r9), %%r10")
	cg.emit("    mov (%%r8,%%r10,8), %%rax")
	cg.emit("    mov %%rax, (%%r8,%%r9,8)")
	cg.emit("    dec %%r9")
	cg.emit("    jmp list_insert_shift")
	cg.emit("list_insert_store:")
	cg.emit("    mov %%r11, (%%r8,%%rsi,8)")
	cg.emit("    inc %%rcx")
	cg.emit("    mov %%rcx, (%%rbx)")
	cg.emit("    lea null_lit(%%rip), %%rax")
	cg.emit("    pop %%rbx")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("list_insert_oob:")
	cg.emit("    lea runtime_err_list_oob(%%rip), %%rax")
	cg.emit("    call runtime_fail")
	cg.emit("")

	// Contains check.
	// Input: rdi = list*, rsi = value, rdx = string mode (1=string compare)
	// Output: rax = 1/0
	cg.emit("list_contains:")
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")
	cg.emit("    push %%rbx")
	cg.emit("    push %%r12")
	cg.emit("    push %%r13")
	cg.emit("    mov %%rsi, %%r12")
	cg.emit("    mov %%rdx, %%r13")
	cg.emit("    mov (%%rdi), %%rcx")
	cg.emit("    mov 16(%%rdi), %%rbx")
	cg.emit("    xor %%r8, %%r8")
	cg.emit("list_contains_loop:")
	cg.emit("    cmp %%rcx, %%r8")
	cg.emit("    jge list_contains_false")
	cg.emit("    mov (%%rbx,%%r8,8), %%rax")
	cg.emit("    cmp $0, %%r13")
	cg.emit("    jne list_contains_string_cmp")
	cg.emit("    cmp %%r12, %%rax")
	cg.emit("    je list_contains_true")
	cg.emit("    inc %%r8")
	cg.emit("    jmp list_contains_loop")
	cg.emit("list_contains_string_cmp:")
	cg.emit("    mov %%rax, %%rdi")
	cg.emit("    mov %%r12, %%rsi")
	cg.emit("    call cstr_eq")
	cg.emit("    cmp $0, %%rax")
	cg.emit("    jne list_contains_true")
	cg.emit("    inc %%r8")
	cg.emit("    jmp list_contains_loop")
	cg.emit("list_contains_true:")
	cg.emit("    mov $1, %%rax")
	cg.emit("    pop %%r13")
	cg.emit("    pop %%r12")
	cg.emit("    pop %%rbx")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("list_contains_false:")
	cg.emit("    xor %%rax, %%rax")
	cg.emit("    pop %%r13")
	cg.emit("    pop %%r12")
	cg.emit("    pop %%rbx")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("")

	// Map layout:
	// [0]=len [8]=cap [16]=keys_ptr [24]=vals_ptr [32]=key_kind(1=string)
	cg.emit("map_new:")
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")
	cg.emit("    push %%rbx")
	cg.emit("    mov %%rdi, %%rcx")
	cg.emit("    mov %%rsi, %%r8")
	cg.emit("    cmp $0, %%rcx")
	cg.emit("    jge map_new_nonneg")
	cg.emit("    xor %%rcx, %%rcx")
	cg.emit("map_new_nonneg:")
	cg.emit("    cmp $4, %%rcx")
	cg.emit("    jge map_new_cap_ok")
	cg.emit("    mov $4, %%rcx")
	cg.emit("map_new_cap_ok:")
	cg.emit("    mov $40, %%rdi")
	cg.emit("    call list_alloc")
	cg.emit("    mov %%rax, %%rbx")
	cg.emit("    mov %%rcx, %%rdi")
	cg.emit("    imul $8, %%rdi, %%rdi")
	cg.emit("    call list_alloc")
	cg.emit("    mov %%rax, %%r9")
	cg.emit("    mov %%rcx, %%rdi")
	cg.emit("    imul $8, %%rdi, %%rdi")
	cg.emit("    call list_alloc")
	cg.emit("    mov $0, (%%rbx)")
	cg.emit("    mov %%rcx, 8(%%rbx)")
	cg.emit("    mov %%r9, 16(%%rbx)")
	cg.emit("    mov %%rax, 24(%%rbx)")
	cg.emit("    mov %%r8, 32(%%rbx)")
	cg.emit("    mov %%rbx, %%rax")
	cg.emit("    pop %%rbx")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("")

	cg.emit("map_grow:")
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")
	cg.emit("    push %%rbx")
	cg.emit("    push %%r12")
	cg.emit("    push %%r13")
	cg.emit("    push %%r14")
	cg.emit("    mov %%rdi, %%rbx")
	cg.emit("    mov 8(%%rbx), %%rcx")
	cg.emit("    lea (%%rcx,%%rcx), %%r12")
	cg.emit("    cmp $4, %%r12")
	cg.emit("    jge map_grow_min4")
	cg.emit("    mov $4, %%r12")
	cg.emit("map_grow_min4:")
	cg.emit("    cmp %%rsi, %%r12")
	cg.emit("    jge map_grow_cap_ok")
	cg.emit("    mov %%rsi, %%r12")
	cg.emit("map_grow_cap_ok:")
	cg.emit("    mov %%r12, %%rdi")
	cg.emit("    imul $8, %%rdi, %%rdi")
	cg.emit("    call list_alloc")
	cg.emit("    mov %%rax, %%r13")
	cg.emit("    mov %%r12, %%rdi")
	cg.emit("    imul $8, %%rdi, %%rdi")
	cg.emit("    call list_alloc")
	cg.emit("    mov %%rax, %%r14")
	cg.emit("    mov (%%rbx), %%r9")
	cg.emit("    mov 16(%%rbx), %%r10")
	cg.emit("    mov 24(%%rbx), %%r11")
	cg.emit("    xor %%r8, %%r8")
	cg.emit("map_grow_copy_loop:")
	cg.emit("    cmp %%r9, %%r8")
	cg.emit("    jge map_grow_copy_done")
	cg.emit("    mov (%%r10,%%r8,8), %%rax")
	cg.emit("    mov %%rax, (%%r13,%%r8,8)")
	cg.emit("    mov (%%r11,%%r8,8), %%rax")
	cg.emit("    mov %%rax, (%%r14,%%r8,8)")
	cg.emit("    inc %%r8")
	cg.emit("    jmp map_grow_copy_loop")
	cg.emit("map_grow_copy_done:")
	cg.emit("    mov %%r13, 16(%%rbx)")
	cg.emit("    mov %%r14, 24(%%rbx)")
	cg.emit("    mov %%r12, 8(%%rbx)")
	cg.emit("    pop %%r14")
	cg.emit("    pop %%r13")
	cg.emit("    pop %%r12")
	cg.emit("    pop %%rbx")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("")

	cg.emit("map_find:")
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")
	cg.emit("    push %%rbx")
	cg.emit("    push %%r12")
	cg.emit("    push %%r13")
	cg.emit("    mov %%rdi, %%rbx")
	cg.emit("    mov %%rsi, %%r12")
	cg.emit("    mov (%%rbx), %%rcx")
	cg.emit("    mov 16(%%rbx), %%r13")
	cg.emit("    mov 32(%%rbx), %%rdx")
	cg.emit("    xor %%r8, %%r8")
	cg.emit("map_find_loop:")
	cg.emit("    cmp %%rcx, %%r8")
	cg.emit("    jge map_find_notfound")
	cg.emit("    mov (%%r13,%%r8,8), %%rax")
	cg.emit("    cmp $1, %%rdx")
	cg.emit("    je map_find_cmp_string")
	cg.emit("    cmp %%r12, %%rax")
	cg.emit("    je map_find_found")
	cg.emit("    inc %%r8")
	cg.emit("    jmp map_find_loop")
	cg.emit("map_find_cmp_string:")
	cg.emit("    mov %%rax, %%rdi")
	cg.emit("    mov %%r12, %%rsi")
	cg.emit("    call cstr_eq")
	cg.emit("    cmp $0, %%rax")
	cg.emit("    jne map_find_found")
	cg.emit("    inc %%r8")
	cg.emit("    jmp map_find_loop")
	cg.emit("map_find_found:")
	cg.emit("    mov %%r8, %%rax")
	cg.emit("    pop %%r13")
	cg.emit("    pop %%r12")
	cg.emit("    pop %%rbx")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("map_find_notfound:")
	cg.emit("    mov $-1, %%rax")
	cg.emit("    pop %%r13")
	cg.emit("    pop %%r12")
	cg.emit("    pop %%rbx")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("")

	cg.emit("map_set:")
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")
	cg.emit("    push %%rbx")
	cg.emit("    push %%r12")
	cg.emit("    mov %%rdi, %%rbx")
	cg.emit("    mov %%rsi, %%r12")
	cg.emit("    push %%rdx")
	cg.emit("    mov %%rbx, %%rdi")
	cg.emit("    mov %%r12, %%rsi")
	cg.emit("    call map_find")
	cg.emit("    pop %%rdx")
	cg.emit("    cmp $-1, %%rax")
	cg.emit("    je map_set_insert")
	cg.emit("    mov 24(%%rbx), %%rcx")
	cg.emit("    mov %%rdx, (%%rcx,%%rax,8)")
	cg.emit("    lea null_lit(%%rip), %%rax")
	cg.emit("    pop %%r12")
	cg.emit("    pop %%rbx")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("map_set_insert:")
	cg.emit("    mov (%%rbx), %%rax")
	cg.emit("    mov 8(%%rbx), %%rcx")
	cg.emit("    cmp %%rcx, %%rax")
	cg.emit("    jl map_set_store")
	cg.emit("    push %%rdx")
	cg.emit("    push %%r12")
	cg.emit("    lea 1(%%rax), %%rsi")
	cg.emit("    mov %%rbx, %%rdi")
	cg.emit("    call map_grow")
	cg.emit("    pop %%r12")
	cg.emit("    pop %%rdx")
	cg.emit("    mov (%%rbx), %%rax")
	cg.emit("map_set_store:")
	cg.emit("    mov 16(%%rbx), %%rcx")
	cg.emit("    mov %%r12, (%%rcx,%%rax,8)")
	cg.emit("    mov 24(%%rbx), %%rcx")
	cg.emit("    mov %%rdx, (%%rcx,%%rax,8)")
	cg.emit("    inc %%rax")
	cg.emit("    mov %%rax, (%%rbx)")
	cg.emit("    lea null_lit(%%rip), %%rax")
	cg.emit("    pop %%r12")
	cg.emit("    pop %%rbx")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("")

	cg.emit("map_get:")
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")
	cg.emit("    push %%rbx")
	cg.emit("    mov %%rdi, %%rbx")
	cg.emit("    mov %%rbx, %%rdi")
	cg.emit("    call map_find")
	cg.emit("    cmp $-1, %%rax")
	cg.emit("    je map_get_missing")
	cg.emit("    mov 24(%%rbx), %%rcx")
	cg.emit("    mov (%%rcx,%%rax,8), %%rax")
	cg.emit("    pop %%rbx")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("map_get_missing:")
	cg.emit("    lea null_lit(%%rip), %%rax")
	cg.emit("    pop %%rbx")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("")

	cg.emit("map_has:")
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")
	cg.emit("    call map_find")
	cg.emit("    cmp $-1, %%rax")
	cg.emit("    je map_has_false")
	cg.emit("    mov $1, %%rax")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("map_has_false:")
	cg.emit("    xor %%rax, %%rax")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("")

	cg.emit("map_remove:")
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")
	cg.emit("    push %%rbx")
	cg.emit("    push %%r12")
	cg.emit("    mov %%rdi, %%rbx")
	cg.emit("    mov %%rsi, %%r12")
	cg.emit("    mov %%rbx, %%rdi")
	cg.emit("    mov %%r12, %%rsi")
	cg.emit("    call map_find")
	cg.emit("    cmp $-1, %%rax")
	cg.emit("    je map_remove_missing")
	cg.emit("    mov %%rax, %%r8")
	cg.emit("    mov (%%rbx), %%rcx")
	cg.emit("    mov 24(%%rbx), %%r9")
	cg.emit("    mov (%%r9,%%r8,8), %%rax")
	cg.emit("    mov 16(%%rbx), %%r10")
	cg.emit("map_remove_shift:")
	cg.emit("    lea 1(%%r8), %%r11")
	cg.emit("    cmp %%rcx, %%r11")
	cg.emit("    jge map_remove_done")
	cg.emit("    mov (%%r10,%%r11,8), %%rdx")
	cg.emit("    mov %%rdx, (%%r10,%%r8,8)")
	cg.emit("    mov (%%r9,%%r11,8), %%rdx")
	cg.emit("    mov %%rdx, (%%r9,%%r8,8)")
	cg.emit("    inc %%r8")
	cg.emit("    jmp map_remove_shift")
	cg.emit("map_remove_done:")
	cg.emit("    dec %%rcx")
	cg.emit("    mov %%rcx, (%%rbx)")
	cg.emit("    pop %%r12")
	cg.emit("    pop %%rbx")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("map_remove_missing:")
	cg.emit("    lea null_lit(%%rip), %%rax")
	cg.emit("    pop %%r12")
	cg.emit("    pop %%rbx")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("")

	cg.emit("map_clear:")
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")
	cg.emit("    mov $0, (%%rdi)")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("")

	// Entry point
	cg.emit("_start:")
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")
	cg.emit("__STACK_ALLOC_MAIN__")
	cg.emit("    lea list_pool(%%rip), %%rax")
	cg.emit("    mov %%rax, list_pool_ptr(%%rip)")
	cg.emit("    xor %%rdi, %%rdi        # default exit code 0")
}

// emitFooter outputs exit syscall and data section
func (cg *CodeGen) emitFooter() {
	cg.emit("")
	cg.emit("%s:", cg.normalExit)
	cg.emit("    xor %%rdi, %%rdi        # normal completion => exit code 0")
	cg.emit("    jmp %s", cg.exitLabel)
	cg.emit("")
	cg.emit("%s:", cg.exitLabel)
	cg.emit("    # Exit")
	cg.emit("    mov %%rbp, %%rsp")
	cg.emit("    pop %%rbp")
	cg.emit("    mov $60, %%rax         # syscall: exit")
	cg.emit("    syscall")
	cg.emit("")
	if cg.funcDefs.Len() > 0 {
		cg.output.WriteString(cg.funcDefs.String())
		cg.emit("")
	}
	cg.emit("    .section .rodata")
	cg.emit("bool_true:  .ascii \"true\"")
	cg.emit("bool_false: .ascii \"false\"")
	cg.emit("null_lit:   .asciz \"null\"")
	cg.emit("hasfield_length: .asciz \"length\"")
	cg.emit("newline_char: .byte 10")
	cg.emit("runtime_err_list_oob: .asciz \"Runtime error: list index out of bounds\"")
	cg.emit("runtime_err_out_of_memory: .asciz \"Runtime error: out of list memory\"")
	for lit, label := range cg.stringLits {
		cg.emit("%s: .asciz \"%s\"", label, escapeAsmString(lit))
	}
	cg.emit("")
	cg.emit("    .section .bss")
	cg.emit("    .lcomm concat_buf, 4096")
	cg.emit("    .align 8")
	cg.emit("list_pool_ptr: .quad 0")
	cg.emit("list_pool:")
	cg.emit("    .skip 1048576")
	cg.emit("list_pool_end:")
}

func (cg *CodeGen) stackAllocLine() string {
	if cg.maxStackOffset <= 0 {
		return "    # no local stack allocation"
	}
	aligned := ((cg.maxStackOffset + 15) / 16) * 16
	return fmt.Sprintf("    sub $%d, %%rsp", aligned)
}

func (cg *CodeGen) allocateSlots(n int) int {
	if n <= 0 {
		return cg.stackOffset
	}
	cg.stackOffset += n * 8
	if cg.stackOffset > cg.maxStackOffset {
		cg.maxStackOffset = cg.stackOffset
	}
	return cg.stackOffset
}

func (cg *CodeGen) ensureArrayLiteralSlot(al *ast.ArrayLiteral) int {
	if off, ok := cg.arraySlots[al]; ok {
		return off
	}
	off := cg.allocateSlots(len(al.Elements))
	cg.arraySlots[al] = off
	return off
}

func (cg *CodeGen) ensureTupleLiteralSlot(tl *ast.TupleLiteral) int {
	if off, ok := cg.tupleSlots[tl]; ok {
		return off
	}
	off := cg.allocateSlots(len(tl.Elements))
	cg.tupleSlots[tl] = off
	return off
}

func (cg *CodeGen) newLabel() string {
	label := fmt.Sprintf(".L%d", cg.labelCount)
	cg.labelCount++
	return label
}
