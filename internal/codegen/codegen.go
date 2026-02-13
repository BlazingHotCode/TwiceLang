package codegen

import (
	"fmt"
	"strings"

	"twice/internal/ast"
)

// CodeGen holds the state for code generation
type CodeGen struct {
	output           strings.Builder
	funcDefs         strings.Builder
	labelCount       int
	exitLabel        string
	normalExit       string
	variables        map[string]int // name -> stack offset
	constVars        map[string]bool
	varTypes         map[string]valueType
	varDeclared      map[string]valueType
	varTypeNames     map[string]string
	varDeclaredNames map[string]string
	varValueTypeName map[string]string
	varIsNull        map[string]bool
	varArrayLen      map[string]int
	intVals          map[string]int64
	charVals         map[string]rune
	stringVals       map[string]string
	floatVals        map[string]float64
	stringLits       map[string]string
	stackOffset      int // current stack position
	maxStackOffset   int
	functions        map[string]*compiledFunction
	funcByName       map[string]string
	funcStmtKeys     map[*ast.FunctionStatement]string
	varFuncs         map[string]string
	typeAliases      map[string]string
	currentFn        string
	nextAnonFn       int
	inFunction       bool
	funcRetLbl       string
	funcRetType      valueType
	funcRetTypeName  string
	loopBreakLabels  []string
	loopContLabels   []string
	arraySlots       map[*ast.ArrayLiteral]int
	tupleSlots       map[*ast.TupleLiteral]int
	scopeDecls       []map[string]struct{}
	typeScopeDecls   []map[string]struct{}
	errors           []CodegenError
}

type compiledFunction struct {
	Key      string
	Name     string
	Label    string
	Literal  *ast.FunctionLiteral
	Captures []string
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
		variables:        make(map[string]int),
		constVars:        make(map[string]bool),
		varTypes:         make(map[string]valueType),
		varDeclared:      make(map[string]valueType),
		varTypeNames:     make(map[string]string),
		varDeclaredNames: make(map[string]string),
		varValueTypeName: make(map[string]string),
		varIsNull:        make(map[string]bool),
		varArrayLen:      make(map[string]int),
		intVals:          make(map[string]int64),
		charVals:         make(map[string]rune),
		stringVals:       make(map[string]string),
		floatVals:        make(map[string]float64),
		stringLits:       make(map[string]string),
		functions:        make(map[string]*compiledFunction),
		funcByName:       make(map[string]string),
		funcStmtKeys:     make(map[*ast.FunctionStatement]string),
		varFuncs:         make(map[string]string),
		typeAliases:      make(map[string]string),
		stackOffset:      0,
		maxStackOffset:   0,
		arraySlots:       make(map[*ast.ArrayLiteral]int),
		tupleSlots:       make(map[*ast.TupleLiteral]int),
		scopeDecls:       []map[string]struct{}{},
		typeScopeDecls:   []map[string]struct{}{},
		errors:           []CodegenError{},
	}
}

// Generate produces x86-64 assembly from a program
func (cg *CodeGen) Generate(program *ast.Program) string {
	cg.reset()
	cg.exitLabel = cg.newLabel()
	cg.normalExit = cg.newLabel()
	cg.collectFunctions(program)
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
	cg.emit("    lea -9(%%rbp), %%rsi    # buffer end (write backwards)")
	cg.emit("    movb $10, (%%rsi)      # newline character")
	cg.emit("    dec %%rsi")
	cg.emit("    mov $1, %%rbx          # digit count (at least 1 for newline)")
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
	cg.emit("    mov $5, %%rdx")
	cg.emit("    jmp print_bool_write")
	cg.emit("print_bool_false:")
	cg.emit("    lea bool_false(%%rip), %%rsi")
	cg.emit("    mov $6, %%rdx")
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

	// Print char helper.
	// Input: rax = codepoint (low byte used)
	cg.emit("# Function: print_char")
	cg.emit("print_char:")
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")
	cg.emit("    sub $16, %%rsp")
	cg.emit("    movb %%al, -2(%%rbp)")
	cg.emit("    movb $10, -1(%%rbp)")
	cg.emit("    lea -2(%%rbp), %%rsi")
	cg.emit("    mov $2, %%rdx")
	cg.emit("    mov $1, %%rax")
	cg.emit("    mov $1, %%rdi")
	cg.emit("    syscall")
	cg.emit("    add $16, %%rsp")
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
	cg.emit("    movb %%al, (%%r13)")
	cg.emit("    inc %%rsi")
	cg.emit("    inc %%r13")
	cg.emit("    test %%al, %%al")
	cg.emit("    jne concat_int_suffix_loop")
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
	cg.emit("    movb %%al, (%%r13)")
	cg.emit("    inc %%r10")
	cg.emit("    inc %%r13")
	cg.emit("    jmp concat_cstr_cstr_copy_left")
	cg.emit("concat_cstr_cstr_left_done:")
	cg.emit("    mov %%rdx, %%r11")
	cg.emit("concat_cstr_cstr_copy_right:")
	cg.emit("    movb (%%r11), %%al")
	cg.emit("    movb %%al, (%%r13)")
	cg.emit("    inc %%r11")
	cg.emit("    inc %%r13")
	cg.emit("    test %%al, %%al")
	cg.emit("    jne concat_cstr_cstr_copy_right")
	cg.emit("    mov %%r12, %%rax")
	cg.emit("    pop %%r13")
	cg.emit("    pop %%r12")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	cg.emit("")

	// Entry point
	cg.emit("_start:")
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")
	cg.emit("__STACK_ALLOC_MAIN__")
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
	cg.emit("bool_true:  .ascii \"true\\n\"")
	cg.emit("bool_false: .ascii \"false\\n\"")
	cg.emit("null_lit:   .asciz \"null\\n\"")
	for lit, label := range cg.stringLits {
		cg.emit("%s: .asciz \"%s\"", label, escapeAsmString(lit))
	}
	cg.emit("")
	cg.emit("    .section .bss")
	cg.emit("    .lcomm concat_buf, 4096")
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

type lexicalScopeState struct {
	variables        map[string]int
	constVars        map[string]bool
	varTypes         map[string]valueType
	varDeclared      map[string]valueType
	varTypeNames     map[string]string
	varDeclaredNames map[string]string
	varValueTypeName map[string]string
	varIsNull        map[string]bool
	varArrayLen      map[string]int
	intVals          map[string]int64
	charVals         map[string]rune
	stringVals       map[string]string
	floatVals        map[string]float64
	varFuncs         map[string]string
	typeAliases      map[string]string
}

func (cg *CodeGen) snapshotLexicalState() lexicalScopeState {
	return lexicalScopeState{
		variables:        cloneMap(cg.variables),
		constVars:        cloneMap(cg.constVars),
		varTypes:         cloneMap(cg.varTypes),
		varDeclared:      cloneMap(cg.varDeclared),
		varTypeNames:     cloneMap(cg.varTypeNames),
		varDeclaredNames: cloneMap(cg.varDeclaredNames),
		varValueTypeName: cloneMap(cg.varValueTypeName),
		varIsNull:        cloneMap(cg.varIsNull),
		varArrayLen:      cloneMap(cg.varArrayLen),
		intVals:          cloneMap(cg.intVals),
		charVals:         cloneMap(cg.charVals),
		stringVals:       cloneMap(cg.stringVals),
		floatVals:        cloneMap(cg.floatVals),
		varFuncs:         cloneMap(cg.varFuncs),
		typeAliases:      cloneMap(cg.typeAliases),
	}
}

func (cg *CodeGen) restoreLexicalState(st lexicalScopeState) {
	cg.variables = st.variables
	cg.constVars = st.constVars
	cg.varTypes = st.varTypes
	cg.varDeclared = st.varDeclared
	cg.varTypeNames = st.varTypeNames
	cg.varDeclaredNames = st.varDeclaredNames
	cg.varValueTypeName = st.varValueTypeName
	cg.varIsNull = st.varIsNull
	cg.varArrayLen = st.varArrayLen
	cg.intVals = st.intVals
	cg.charVals = st.charVals
	cg.stringVals = st.stringVals
	cg.floatVals = st.floatVals
	cg.varFuncs = st.varFuncs
	cg.typeAliases = st.typeAliases
}

func cloneMap[K comparable, V any](in map[K]V) map[K]V {
	out := make(map[K]V, len(in))
	for k := range in {
		out[k] = in[k]
	}
	return out
}

func (cg *CodeGen) currentScopeDecls() map[string]struct{} {
	if len(cg.scopeDecls) == 0 {
		cg.scopeDecls = append(cg.scopeDecls, make(map[string]struct{}))
	}
	return cg.scopeDecls[len(cg.scopeDecls)-1]
}

func (cg *CodeGen) currentTypeScopeDecls() map[string]struct{} {
	if len(cg.typeScopeDecls) == 0 {
		cg.typeScopeDecls = append(cg.typeScopeDecls, make(map[string]struct{}))
	}
	return cg.typeScopeDecls[len(cg.typeScopeDecls)-1]
}

func (cg *CodeGen) isDeclaredInCurrentScope(name string) bool {
	_, ok := cg.currentScopeDecls()[name]
	return ok
}

func (cg *CodeGen) markDeclaredInCurrentScope(name string) {
	cg.currentScopeDecls()[name] = struct{}{}
}

func (cg *CodeGen) isTypeAliasDeclaredInCurrentScope(name string) bool {
	_, ok := cg.currentTypeScopeDecls()[name]
	return ok
}

func (cg *CodeGen) markTypeAliasDeclaredInCurrentScope(name string) {
	cg.currentTypeScopeDecls()[name] = struct{}{}
}

func (cg *CodeGen) enterScope() lexicalScopeState {
	cg.scopeDecls = append(cg.scopeDecls, make(map[string]struct{}))
	cg.typeScopeDecls = append(cg.typeScopeDecls, make(map[string]struct{}))
	return cg.snapshotLexicalState()
}

func (cg *CodeGen) exitScope(st lexicalScopeState) {
	if len(cg.scopeDecls) == 0 || len(cg.typeScopeDecls) == 0 {
		return
	}
	declared := cg.scopeDecls[len(cg.scopeDecls)-1]
	cg.scopeDecls = cg.scopeDecls[:len(cg.scopeDecls)-1]
	cg.typeScopeDecls = cg.typeScopeDecls[:len(cg.typeScopeDecls)-1]

	currVariables := cg.variables
	currConstVars := cg.constVars
	currVarTypes := cg.varTypes
	currVarDeclared := cg.varDeclared
	currVarTypeNames := cg.varTypeNames
	currVarDeclaredNames := cg.varDeclaredNames
	currVarValueTypeName := cg.varValueTypeName
	currVarIsNull := cg.varIsNull
	currVarArrayLen := cg.varArrayLen
	currIntVals := cg.intVals
	currCharVals := cg.charVals
	currStringVals := cg.stringVals
	currFloatVals := cg.floatVals
	currVarFuncs := cg.varFuncs

	cg.restoreLexicalState(st)

	for name := range st.variables {
		if _, shadowed := declared[name]; shadowed {
			continue
		}
		if _, ok := currVariables[name]; !ok {
			continue
		}

		cg.variables[name] = currVariables[name]
		if v, ok := currConstVars[name]; ok {
			cg.constVars[name] = v
		} else {
			delete(cg.constVars, name)
		}
		if v, ok := currVarTypes[name]; ok {
			cg.varTypes[name] = v
		} else {
			delete(cg.varTypes, name)
		}
		if v, ok := currVarDeclared[name]; ok {
			cg.varDeclared[name] = v
		} else {
			delete(cg.varDeclared, name)
		}
		if v, ok := currVarTypeNames[name]; ok {
			cg.varTypeNames[name] = v
		} else {
			delete(cg.varTypeNames, name)
		}
		if v, ok := currVarDeclaredNames[name]; ok {
			cg.varDeclaredNames[name] = v
		} else {
			delete(cg.varDeclaredNames, name)
		}
		if v, ok := currVarValueTypeName[name]; ok {
			cg.varValueTypeName[name] = v
		} else {
			delete(cg.varValueTypeName, name)
		}
		if v, ok := currVarIsNull[name]; ok {
			cg.varIsNull[name] = v
		} else {
			delete(cg.varIsNull, name)
		}
		if v, ok := currVarArrayLen[name]; ok {
			cg.varArrayLen[name] = v
		} else {
			delete(cg.varArrayLen, name)
		}
		if v, ok := currIntVals[name]; ok {
			cg.intVals[name] = v
		} else {
			delete(cg.intVals, name)
		}
		if v, ok := currCharVals[name]; ok {
			cg.charVals[name] = v
		} else {
			delete(cg.charVals, name)
		}
		if v, ok := currStringVals[name]; ok {
			cg.stringVals[name] = v
		} else {
			delete(cg.stringVals, name)
		}
		if v, ok := currFloatVals[name]; ok {
			cg.floatVals[name] = v
		} else {
			delete(cg.floatVals, name)
		}
		if v, ok := currVarFuncs[name]; ok {
			cg.varFuncs[name] = v
		} else {
			delete(cg.varFuncs, name)
		}
	}
}

func (cg *CodeGen) newLabel() string {
	label := fmt.Sprintf(".L%d", cg.labelCount)
	cg.labelCount++
	return label
}
