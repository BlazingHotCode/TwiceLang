package codegen

import (
	"fmt"
	"math"
	"strings"

	"twice/internal/ast"
	"twice/internal/token"
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
	varFuncs         map[string]string
	currentFn        string
	nextAnonFn       int
	inFunction       bool
	funcRetLbl       string
	funcRetType      valueType
	funcRetTypeName  string
	loopBreakLabels  []string
	loopContLabels   []string
	arraySlots       map[*ast.ArrayLiteral]int
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
		varIsNull:        make(map[string]bool),
		varArrayLen:      make(map[string]int),
		intVals:          make(map[string]int64),
		charVals:         make(map[string]rune),
		stringVals:       make(map[string]string),
		floatVals:        make(map[string]float64),
		stringLits:       make(map[string]string),
		functions:        make(map[string]*compiledFunction),
		funcByName:       make(map[string]string),
		varFuncs:         make(map[string]string),
		stackOffset:      0,
		maxStackOffset:   0,
		arraySlots:       make(map[*ast.ArrayLiteral]int),
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
	cg.generateFunctionDefinitions()

	cg.emitFooter()
	asm := cg.output.String()
	asm = strings.ReplaceAll(asm, "__STACK_ALLOC_MAIN__", cg.stackAllocLine())
	return asm
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

func (cg *CodeGen) generateStatement(stmt ast.Statement) {
	switch s := stmt.(type) {
	case *ast.LetStatement:
		cg.generateLet(s)
	case *ast.ConstStatement:
		cg.generateConst(s)
	case *ast.AssignStatement:
		cg.generateAssign(s)
	case *ast.IndexAssignStatement:
		cg.generateIndexAssign(s)
	case *ast.ReturnStatement:
		cg.generateReturn(s)
	case *ast.WhileStatement:
		cg.generateWhileStatement(s)
	case *ast.LoopStatement:
		cg.generateLoopStatement(s)
	case *ast.ForStatement:
		cg.generateForStatement(s)
	case *ast.BreakStatement:
		cg.generateBreakStatement(s)
	case *ast.ContinueStatement:
		cg.generateContinueStatement(s)
	case *ast.ExpressionStatement:
		cg.generateExpression(s.Expression)
	case *ast.FunctionStatement:
		if cg.inFunction {
			cg.addNodeError("nested function declarations are not supported in codegen", s)
		}
	}
}

// generateExpression dispatches to specific expression generators
func (cg *CodeGen) generateExpression(expr ast.Expression) {
	switch e := expr.(type) {
	case *ast.IntegerLiteral:
		cg.generateInteger(e)
	case *ast.FloatLiteral:
		cg.generateFloat(e)
	case *ast.StringLiteral:
		cg.generateString(e)
	case *ast.CharLiteral:
		cg.generateChar(e)
	case *ast.NullLiteral:
		cg.generateNull(e)
	case *ast.ArrayLiteral:
		cg.generateArrayLiteral(e)
	case *ast.Boolean:
		cg.generateBoolean(e)
	case *ast.InfixExpression:
		cg.generateInfix(e)
	case *ast.PrefixExpression:
		cg.generatePrefix(e)
	case *ast.Identifier:
		cg.generateIdentifier(e)
	case *ast.IfExpression:
		cg.generateIfExpression(e)
	case *ast.CallExpression:
		cg.generateCallExpression(e)
	case *ast.IndexExpression:
		cg.generateIndexExpression(e)
	case *ast.MethodCallExpression:
		cg.generateMethodCallExpression(e)
	case *ast.FunctionLiteral:
		cg.addNodeError("function literals are not supported in codegen yet", e)
		cg.emit("    mov $0, %%rax")
	case *ast.NamedArgument:
		cg.addNodeError("named arguments are only valid inside function calls", e)
		cg.emit("    mov $0, %%rax")
	default:
		cg.addNodeError("unsupported expression in codegen", e)
		cg.emit("    mov $0, %%rax")
	}
}

// generateInteger loads an integer into rax
func (cg *CodeGen) generateInteger(il *ast.IntegerLiteral) {
	cg.emit("    mov $%d, %%rax", il.Value)
}

func (cg *CodeGen) generateFloat(fl *ast.FloatLiteral) {
	label := cg.stringLabel(fmt.Sprintf("%g\n", fl.Value))
	cg.emit("    lea %s(%%rip), %%rax", label)
}

func (cg *CodeGen) generateString(sl *ast.StringLiteral) {
	label := cg.stringLabel(sl.Value + "\n")
	cg.emit("    lea %s(%%rip), %%rax", label)
}

func (cg *CodeGen) generateChar(cl *ast.CharLiteral) {
	cg.emit("    mov $%d, %%rax", cl.Value)
}

func (cg *CodeGen) generateNull(_ *ast.NullLiteral) {
	cg.emit("    lea null_lit(%%rip), %%rax")
}

func (cg *CodeGen) generateArrayLiteral(al *ast.ArrayLiteral) {
	if al == nil || len(al.Elements) == 0 {
		cg.addNodeError("empty array literals are not supported in codegen", al)
		cg.emit("    mov $0, %%rax")
		return
	}
	elemTypeName, ok := cg.inferArrayLiteralTypeName(al)
	if !ok {
		cg.addNodeError("array literal elements must have the same type", al)
		cg.emit("    mov $0, %%rax")
		return
	}
	if elemTypeName == "null" {
		cg.addNodeError("array literal elements cannot be null", al)
		cg.emit("    mov $0, %%rax")
		return
	}

	baseOffset := cg.ensureArrayLiteralSlot(al)
	for i, el := range al.Elements {
		cg.generateExpression(el)
		cg.emit("    mov %%rax, -%d(%%rbp)", baseOffset-i*8)
	}
	cg.emit("    lea -%d(%%rbp), %%rax", baseOffset)
}

func (cg *CodeGen) generateIndexExpression(ie *ast.IndexExpression) {
	if ie == nil {
		cg.emit("    mov $0, %%rax")
		return
	}
	leftTypeName := cg.inferExpressionTypeName(ie.Left)
	if id, ok := ie.Left.(*ast.Identifier); ok && cg.varIsNull[id.Value] && leftTypeName != "string" {
		cg.addNodeError("cannot index null array", ie)
		cg.emit("    mov $0, %%rax")
		return
	}
	if cg.inferExpressionType(ie.Index) != typeInt {
		cg.addNodeError("index must be int", ie.Index)
		cg.emit("    mov $0, %%rax")
		return
	}

	if leftTypeName == "string" {
		if idx, ok := cg.constIntValue(ie.Index); ok {
			if s, ok := cg.constStringValue(ie.Left); ok {
				if idx < 0 || int(idx) >= len(s) {
					cg.addNodeError(fmt.Sprintf("string index out of bounds: %d", idx), ie)
					cg.emit("    mov $0, %%rax")
					return
				}
			}
		}
		cg.generateExpression(ie.Left) // string pointer
		cg.emit("    push %%rax")
		cg.generateExpression(ie.Index)
		cg.emit("    pop %%rcx")
		cg.emit("    movzbq (%%rcx,%%rax), %%rax")
		return
	}

	elemTypeName, arrLen, ok := peelArrayType(leftTypeName)
	if !ok {
		cg.addNodeError("index operator not supported for non-array/string type", ie)
		cg.emit("    mov $0, %%rax")
		return
	}
	if idx, ok := cg.constIntValue(ie.Index); ok && arrLen >= 0 {
		if idx < 0 || int(idx) >= arrLen {
			cg.addNodeError(fmt.Sprintf("array index out of bounds: %d", idx), ie)
			cg.emit("    mov $0, %%rax")
			return
		}
	}

	cg.generateExpression(ie.Left) // array pointer
	cg.emit("    push %%rax")
	cg.generateExpression(ie.Index)
	cg.emit("    imul $8, %%rax, %%rax")
	cg.emit("    pop %%rcx")
	cg.emit("    mov (%%rcx,%%rax), %%rax")

	_ = elemTypeName
}

func (cg *CodeGen) generateMethodCallExpression(mce *ast.MethodCallExpression) {
	if mce == nil || mce.Method == nil {
		cg.addNodeError("invalid method call", mce)
		cg.emit("    mov $0, %%rax")
		return
	}
	switch mce.Method.Value {
	case "length":
		if len(mce.Arguments) != 0 {
			cg.addNodeError(fmt.Sprintf("length expects 0 arguments, got=%d", len(mce.Arguments)), mce)
			cg.emit("    mov $0, %%rax")
			return
		}
		typeName := cg.inferExpressionTypeName(mce.Object)
		_, n, ok := peelArrayType(typeName)
		if !ok {
			cg.addNodeError("length is only supported on arrays", mce)
			cg.emit("    mov $0, %%rax")
			return
		}
		if n < 0 {
			cg.addNodeError("array length is unknown at compile time", mce)
			cg.emit("    mov $0, %%rax")
			return
		}
		cg.emit("    mov $%d, %%rax", n)
	default:
		cg.addNodeError("unknown method: "+mce.Method.Value, mce)
		cg.emit("    mov $0, %%rax")
	}
}

func (cg *CodeGen) generateBoolean(b *ast.Boolean) {
	if b.Value {
		cg.emit("    mov $1, %%rax") // true = 1
	} else {
		cg.emit("    mov $0, %%rax") // false = 0
	}
}

// generateInfix handles binary operations: left op right
// We use the stack to hold intermediate results
func (cg *CodeGen) generateInfix(ie *ast.InfixExpression) {
	leftType := cg.inferExpressionType(ie.Left)
	rightType := cg.inferExpressionType(ie.Right)

	if leftType == typeBool && rightType == typeBool {
		cg.generateExpression(ie.Right)
		cg.emit("    push %%rax")
		cg.generateExpression(ie.Left)
		cg.emit("    pop %%rcx")
		switch ie.Operator {
		case "&&":
			cg.emit("    and %%rcx, %%rax")
			return
		case "||":
			cg.emit("    or %%rcx, %%rax")
			cg.emit("    test %%rax, %%rax")
			cg.emit("    setne %%al")
			cg.emit("    movzbq %%al, %%rax")
			return
		case "^^":
			cg.emit("    xor %%rcx, %%rax")
			return
		default:
			cg.addNodeError("unsupported boolean operator in codegen", ie)
			cg.emit("    mov $0, %%rax")
			return
		}
	}

	if leftType == typeString && ie.Operator == "+" {
		combined, ok := cg.constStringValue(ie)
		if !ok {
			if rightType == typeInt {
				// runtime cstr + int
				cg.generateExpression(ie.Right) // int
				cg.emit("    push %%rax")
				cg.generateExpression(ie.Left) // c-string ptr
				cg.emit("    pop %%rdx")
				cg.emit("    call concat_cstr_int")
				return
			}
			cg.addNodeError("string concatenation in codegen requires compile-time known values", ie)
			cg.emit("    mov $0, %%rax")
			return
		}
		label := cg.stringLabel(combined + "\n")
		cg.emit("    lea %s(%%rip), %%rax", label)
		return
	}

	if rightType == typeString && leftType == typeInt && ie.Operator == "+" {
		// runtime int + cstr
		cg.generateExpression(ie.Right) // c-string ptr
		cg.emit("    push %%rax")
		cg.generateExpression(ie.Left) // int
		cg.emit("    pop %%rdx")
		cg.emit("    call concat_int_cstr")
		return
	}

	if isNumericType(leftType) && isNumericType(rightType) && (leftType == typeFloat || rightType == typeFloat) {
		v, ok := cg.constFloatValue(ie)
		if !ok {
			cg.addNodeError("numeric infix with float result requires compile-time known values in codegen", ie)
			cg.emit("    mov $0, %%rax")
			return
		}
		label := cg.stringLabel(fmt.Sprintf("%g\n", v))
		cg.emit("    lea %s(%%rip), %%rax", label)
		return
	}

	if leftType == typeChar && rightType == typeChar {
		if ie.Operator != "+" {
			cg.addNodeError("char infix supports only + in codegen", ie)
			cg.emit("    mov $0, %%rax")
			return
		}
		// char + char -> char
		cg.generateExpression(ie.Right)
		cg.emit("    push %%rax")
		cg.generateExpression(ie.Left)
		cg.emit("    pop %%rcx")
		cg.emit("    add %%rcx, %%rax")
		return
	}

	if leftType == typeChar && rightType == typeInt && ie.Operator == "+" {
		// char + int -> char
		cg.generateExpression(ie.Right)
		cg.emit("    push %%rax")
		cg.generateExpression(ie.Left)
		cg.emit("    pop %%rcx")
		cg.emit("    add %%rcx, %%rax")
		return
	}

	if leftType == typeType && rightType == typeType {
		if ie.Operator != "==" && ie.Operator != "!=" {
			cg.addNodeError("type comparisons support only == and !=", ie)
			cg.emit("    mov $0, %%rax")
			return
		}
		cg.generateExpression(ie.Right)
		cg.emit("    push %%rax")
		cg.generateExpression(ie.Left)
		cg.emit("    pop %%rcx")
		cg.emit("    cmp %%rcx, %%rax")
		if ie.Operator == "==" {
			cg.emit("    sete %%al")
		} else {
			cg.emit("    setne %%al")
		}
		cg.emit("    movzbq %%al, %%rax")
		return
	}

	if leftType != typeInt || rightType != typeInt {
		cg.addNodeError("unsupported infix operand types in codegen", ie)
		cg.emit("    mov $0, %%rax")
		return
	}

	// Generate right side first (will be in rax)
	cg.generateExpression(ie.Right)
	// Push right side to stack
	cg.emit("    push %%rax")

	// Generate left side (will be in rax)
	cg.generateExpression(ie.Left)
	// Pop right side into rcx
	cg.emit("    pop %%rcx")

	// Now rax = left, rcx = right
	switch ie.Operator {
	case "+":
		cg.emit("    add %%rcx, %%rax")
	case "-":
		cg.emit("    sub %%rcx, %%rax")
	case "*":
		cg.emit("    imul %%rcx, %%rax")
	case "/":
		cg.emit("    cqo                 # sign extend rax to rdx:rax")
		cg.emit("    idiv %%rcx          # rax = rdx:rax / rcx")
	case "%":
		cg.emit("    cqo                 # sign extend rax to rdx:rax")
		cg.emit("    idiv %%rcx          # rdx = rdx:rax %% rcx")
		cg.emit("    mov %%rdx, %%rax")
	case "&":
		cg.emit("    and %%rcx, %%rax")
	case "|":
		cg.emit("    or %%rcx, %%rax")
	case "^":
		cg.emit("    xor %%rcx, %%rax")
	case "<<":
		cg.emit("    mov %%ecx, %%ecx")
		cg.emit("    shl %%cl, %%rax")
	case ">>":
		cg.emit("    mov %%ecx, %%ecx")
		cg.emit("    sar %%cl, %%rax")
	case "<":
		cg.emit("    cmp %%rcx, %%rax")
		cg.emit("    setl %%al           # set al to 1 if less, 0 otherwise")
		cg.emit("    movzbq %%al, %%rax  # zero extend to 64 bits")
	case ">":
		cg.emit("    cmp %%rcx, %%rax")
		cg.emit("    setg %%al")
		cg.emit("    movzbq %%al, %%rax")
	case "==":
		cg.emit("    cmp %%rcx, %%rax")
		cg.emit("    sete %%al")
		cg.emit("    movzbq %%al, %%rax")
	case "!=":
		cg.emit("    cmp %%rcx, %%rax")
		cg.emit("    setne %%al")
		cg.emit("    movzbq %%al, %%rax")
	}
}

// generatePrefix handles unary operators
func (cg *CodeGen) generatePrefix(pe *ast.PrefixExpression) {
	cg.generateExpression(pe.Right)

	switch pe.Operator {
	case "-":
		cg.emit("    neg %%rax")
	case "!":
		// !x is equivalent to x == 0
		cg.emit("    test %%rax, %%rax")
		cg.emit("    sete %%al")
		cg.emit("    movzbq %%al, %%rax")
	}
}

func (cg *CodeGen) generateIdentifier(i *ast.Identifier) {
	offset, ok := cg.variables[i.Value]
	if !ok {
		if isTypeLiteralIdentifier(i.Value) {
			label := cg.stringLabel(i.Value + "\n")
			cg.emit("    lea %s(%%rip), %%rax", label)
			return
		}
		cg.addNodeError("identifier not found: "+i.Value, i)
		cg.emit("    mov $0, %%rax")
		return
	}
	if cg.varIsNull[i.Value] {
		if cg.varTypes[i.Value] == typeArray {
			cg.emit("    mov $0, %%rax")
			return
		}
		cg.emit("    lea null_lit(%%rip), %%rax")
		return
	}
	// Load from stack: rbp - offset
	cg.emit("    mov -%d(%%rbp), %%rax  # load %s", offset, i.Value)
}

// generateLet handles variable declarations (simplified - no stack frame yet)
func (cg *CodeGen) generateLet(ls *ast.LetStatement) {
	if _, exists := cg.variables[ls.Name.Value]; exists {
		if cg.constVars[ls.Name.Value] {
			cg.addNodeError("cannot reassign const: "+ls.Name.Value, ls)
		} else {
			cg.addNodeError("identifier already declared: "+ls.Name.Value, ls)
		}
		cg.emit("    mov $0, %%rax")
		return
	}

	declared := parseTypeName(ls.TypeName)
	if ls.TypeName != "" && !isKnownTypeName(ls.TypeName) {
		cg.addNodeError("unknown type: "+ls.TypeName, ls)
		cg.emit("    mov $0, %%rax")
		return
	}
	if declared != typeUnknown {
		cg.varDeclared[ls.Name.Value] = declared
		cg.varDeclaredNames[ls.Name.Value] = ls.TypeName
	}

	if ls.Value == nil {
		cg.generateNull(&ast.NullLiteral{})
	} else {
		cg.generateExpression(ls.Value)
	}

	name := ls.Name.Value
	offset := cg.allocateSlots(1)
	cg.variables[name] = offset
	inferred := typeNull
	inferredName := "null"
	if ls.Value != nil {
		inferred = cg.inferExpressionType(ls.Value)
		inferredName = cg.inferExpressionTypeName(ls.Value)
	}
	targetName := ls.TypeName
	if targetName == "" {
		targetName = inferredName
	}
	if targetName != "unknown" && inferredName != "unknown" && !isAssignableTypeName(targetName, inferredName) {
		cg.addNodeError(fmt.Sprintf("cannot assign %s to %s", inferredName, targetName), ls)
		cg.emit("    mov $0, %%rax")
		return
	}
	if declared != typeUnknown {
		cg.varTypes[name] = declared
	} else {
		cg.varTypes[name] = inferred
	}
	cg.varTypeNames[name] = targetName
	if _, n, ok := peelArrayType(targetName); ok {
		cg.varArrayLen[name] = n
	} else {
		delete(cg.varArrayLen, name)
	}
	cg.varIsNull[name] = inferred == typeNull
	cg.trackKnownValue(name, cg.varTypes[name], ls.Value)

	cg.emit("    mov %%rax, -%d(%%rbp)  # let %s", offset, name)
}

// generateConst handles immutable variable declarations.
func (cg *CodeGen) generateConst(cs *ast.ConstStatement) {
	if _, exists := cg.variables[cs.Name.Value]; exists {
		cg.addNodeError("identifier already declared: "+cs.Name.Value, cs)
		cg.emit("    mov $0, %%rax")
		return
	}

	declared := parseTypeName(cs.TypeName)
	if cs.TypeName != "" && !isKnownTypeName(cs.TypeName) {
		cg.addNodeError("unknown type: "+cs.TypeName, cs)
		cg.emit("    mov $0, %%rax")
		return
	}
	if declared != typeUnknown {
		cg.varDeclared[cs.Name.Value] = declared
		cg.varDeclaredNames[cs.Name.Value] = cs.TypeName
	}

	cg.generateExpression(cs.Value)

	name := cs.Name.Value
	offset := cg.allocateSlots(1)
	cg.variables[name] = offset
	cg.constVars[name] = true
	inferred := cg.inferExpressionType(cs.Value)
	inferredName := cg.inferExpressionTypeName(cs.Value)
	targetName := cs.TypeName
	if targetName == "" {
		targetName = inferredName
	}
	if targetName != "unknown" && inferredName != "unknown" && !isAssignableTypeName(targetName, inferredName) {
		cg.addNodeError(fmt.Sprintf("cannot assign %s to %s", inferredName, targetName), cs)
		cg.emit("    mov $0, %%rax")
		return
	}
	if declared != typeUnknown {
		cg.varTypes[name] = declared
	} else {
		cg.varTypes[name] = inferred
	}
	cg.varTypeNames[name] = targetName
	if _, n, ok := peelArrayType(targetName); ok {
		cg.varArrayLen[name] = n
	} else {
		delete(cg.varArrayLen, name)
	}
	cg.varIsNull[name] = inferred == typeNull
	cg.trackKnownValue(name, cg.varTypes[name], cs.Value)

	cg.emit("    mov %%rax, -%d(%%rbp)  # const %s", offset, name)
}

// generateAssign handles variable reassignment.
func (cg *CodeGen) generateAssign(as *ast.AssignStatement) {
	offset, exists := cg.variables[as.Name.Value]
	if !exists {
		cg.addNodeError("identifier not found: "+as.Name.Value, as)
		cg.emit("    mov $0, %%rax")
		return
	}
	if cg.constVars[as.Name.Value] {
		cg.addNodeError("cannot reassign const: "+as.Name.Value, as)
		cg.emit("    mov $0, %%rax")
		return
	}

	cg.generateExpression(as.Value)
	cg.emit("    mov %%rax, -%d(%%rbp)  # assign %s", offset, as.Name.Value)
	inferred := cg.inferExpressionType(as.Value)
	inferredName := cg.inferExpressionTypeName(as.Value)
	target := cg.varTypes[as.Name.Value]
	targetName := cg.varTypeNames[as.Name.Value]
	if target == typeNull && inferred != typeNull {
		target = inferred
	}
	if declared, ok := cg.varDeclared[as.Name.Value]; ok {
		target = declared
	}
	if declaredName, ok := cg.varDeclaredNames[as.Name.Value]; ok {
		targetName = declaredName
	}
	if targetName == "" {
		targetName = typeName(target)
	}
	if inf, ok := as.Value.(*ast.InfixExpression); ok {
		if inf.Token.Type == token.PLUSPLUS || inf.Token.Type == token.MINUSMIN {
			if target != typeInt {
				cg.addNodeError("++/-- only supported for int variables", as)
				cg.emit("    mov $0, %%rax")
				return
			}
		}
	}
	if inferredName != "unknown" && targetName != "unknown" && !isAssignableTypeName(targetName, inferredName) {
		cg.addNodeError(fmt.Sprintf("cannot assign %s to %s", inferredName, targetName), as)
		cg.emit("    mov $0, %%rax")
		return
	}
	if _, isUnion := splitTopLevelUnion(targetName); isUnion && inferred != typeUnknown && inferred != typeNull {
		cg.varTypes[as.Name.Value] = inferred
	} else if target == typeUnknown {
		cg.varTypes[as.Name.Value] = inferred
	} else {
		cg.varTypes[as.Name.Value] = target
	}
	cg.varTypeNames[as.Name.Value] = targetName
	if _, n, ok := peelArrayType(targetName); ok {
		cg.varArrayLen[as.Name.Value] = n
	} else {
		delete(cg.varArrayLen, as.Name.Value)
	}
	cg.varIsNull[as.Name.Value] = inferred == typeNull
	cg.trackKnownValue(as.Name.Value, cg.varTypes[as.Name.Value], as.Value)
}

func (cg *CodeGen) generateIndexAssign(ias *ast.IndexAssignStatement) {
	if ias == nil || ias.Left == nil {
		cg.addNodeError("invalid indexed assignment", ias)
		cg.emit("    mov $0, %%rax")
		return
	}
	id, ok := ias.Left.Left.(*ast.Identifier)
	if !ok {
		cg.addNodeError("indexed assignment target must be an identifier", ias.Left.Left)
		cg.emit("    mov $0, %%rax")
		return
	}
	name := id.Value
	offset, exists := cg.variables[name]
	if !exists {
		cg.addNodeError("identifier not found: "+name, ias)
		cg.emit("    mov $0, %%rax")
		return
	}
	if cg.constVars[name] {
		cg.addNodeError("cannot reassign const: "+name, ias)
		cg.emit("    mov $0, %%rax")
		return
	}
	if cg.varIsNull[name] {
		cg.addNodeError("cannot index null array", ias)
		cg.emit("    mov $0, %%rax")
		return
	}

	arrTypeName := cg.varTypeNames[name]
	elemTypeName, arrLen, ok := peelArrayType(arrTypeName)
	if !ok {
		cg.addNodeError("indexed assignment target is not an array", ias)
		cg.emit("    mov $0, %%rax")
		return
	}
	if cg.inferExpressionType(ias.Left.Index) != typeInt {
		cg.addNodeError("array index must be int", ias.Left.Index)
		cg.emit("    mov $0, %%rax")
		return
	}
	if idx, ok := cg.constIntValue(ias.Left.Index); ok && arrLen >= 0 {
		if idx < 0 || int(idx) >= arrLen {
			cg.addNodeError(fmt.Sprintf("array index out of bounds: %d", idx), ias)
			cg.emit("    mov $0, %%rax")
			return
		}
	}

	valueTypeName := cg.inferExpressionTypeName(ias.Value)
	if valueTypeName != "unknown" && !isAssignableTypeName(elemTypeName, valueTypeName) {
		cg.addNodeError(fmt.Sprintf("cannot assign %s to %s", valueTypeName, elemTypeName), ias)
		cg.emit("    mov $0, %%rax")
		return
	}

	cg.generateExpression(ias.Value)
	cg.emit("    push %%rax")
	cg.generateExpression(ias.Left.Index)
	cg.emit("    imul $8, %%rax, %%rax")
	cg.emit("    mov -%d(%%rbp), %%rcx  # load %s", offset, name)
	cg.emit("    pop %%rdx")
	cg.emit("    mov %%rdx, (%%rcx,%%rax)")
	cg.emit("    mov %%rdx, %%rax")
}

// generateReturn handles return statements
func (cg *CodeGen) generateReturn(rs *ast.ReturnStatement) {
	cg.generateExpression(rs.ReturnValue)
	if cg.inFunction {
		if cg.funcRetType != typeUnknown {
			got := cg.inferExpressionType(rs.ReturnValue)
			if got != typeNull && got != typeUnknown && got != cg.funcRetType && cg.funcRetType != typeArray {
				cg.addNodeError(fmt.Sprintf("cannot return %s from function returning %s", typeName(got), typeName(cg.funcRetType)), rs)
			}
		}
		if cg.funcRetTypeName != "" {
			gotName := cg.inferExpressionTypeName(rs.ReturnValue)
			if gotName != "null" && gotName != "unknown" && !isAssignableTypeName(cg.funcRetTypeName, gotName) {
				cg.addNodeError(fmt.Sprintf("cannot return %s from function returning %s", gotName, cg.funcRetTypeName), rs)
			}
		}
		cg.emit("    jmp %s", cg.funcRetLbl)
		return
	}
	cg.emit("    mov %%rax, %%rdi       # return value as process exit code")
	cg.emit("    jmp %s", cg.exitLabel)
}

func (cg *CodeGen) generateIfExpression(ie *ast.IfExpression) {
	elseLabel := cg.newLabel()
	endLabel := cg.newLabel()

	// Generate condition
	cg.generateExpression(ie.Condition)

	// Test if false (0)
	cg.emit("    test %%rax, %%rax")
	cg.emit("    jz %s              # jump if condition is false", elseLabel)

	// Generate consequence (if block)
	cg.generateBlockStatement(ie.Consequence)
	cg.emit("    jmp %s             # jump to end", endLabel)

	// Else block
	cg.emit("%s:", elseLabel)
	if ie.Alternative != nil {
		cg.generateBlockStatement(ie.Alternative)
	}

	// End
	cg.emit("%s:", endLabel)
}

func (cg *CodeGen) generateWhileStatement(ws *ast.WhileStatement) {
	startLabel := cg.newLabel()
	endLabel := cg.newLabel()
	cg.pushLoopLabels(endLabel, startLabel)
	defer cg.popLoopLabels()
	cg.emit("%s:", startLabel)
	cg.generateExpression(ws.Condition)
	cg.emit("    test %%rax, %%rax")
	cg.emit("    jz %s", endLabel)
	cg.generateBlockStatement(ws.Body)
	cg.emit("    jmp %s", startLabel)
	cg.emit("%s:", endLabel)
}

func (cg *CodeGen) generateLoopStatement(ls *ast.LoopStatement) {
	startLabel := cg.newLabel()
	cg.pushLoopLabels(cg.newLabel(), startLabel)
	breakLabel := cg.currentBreakLabel()
	defer cg.popLoopLabels()
	cg.emit("%s:", startLabel)
	cg.generateBlockStatement(ls.Body)
	cg.emit("    jmp %s", startLabel)
	cg.emit("%s:", breakLabel)
}

func (cg *CodeGen) generateForStatement(fs *ast.ForStatement) {
	if fs.Init != nil {
		cg.generateStatement(fs.Init)
	}
	startLabel := cg.newLabel()
	periodicLabel := cg.newLabel()
	endLabel := cg.newLabel()
	cg.pushLoopLabels(endLabel, periodicLabel)
	defer cg.popLoopLabels()
	cg.emit("%s:", startLabel)
	if fs.Condition != nil {
		cg.generateExpression(fs.Condition)
		cg.emit("    test %%rax, %%rax")
		cg.emit("    jz %s", endLabel)
	}
	cg.generateBlockStatement(fs.Body)
	cg.emit("%s:", periodicLabel)
	if fs.Periodic != nil {
		cg.generateStatement(fs.Periodic)
	}
	cg.emit("    jmp %s", startLabel)
	cg.emit("%s:", endLabel)
}

func (cg *CodeGen) generateBreakStatement(bs *ast.BreakStatement) {
	label := cg.currentBreakLabel()
	if label == "" {
		cg.addNodeError("break not inside loop", bs)
		cg.emit("    mov $0, %%rax")
		return
	}
	cg.emit("    jmp %s", label)
}

func (cg *CodeGen) generateContinueStatement(cs *ast.ContinueStatement) {
	label := cg.currentContinueLabel()
	if label == "" {
		cg.addNodeError("continue not inside loop", cs)
		cg.emit("    mov $0, %%rax")
		return
	}
	cg.emit("    jmp %s", label)
}

func (cg *CodeGen) pushLoopLabels(breakLabel, contLabel string) {
	cg.loopBreakLabels = append(cg.loopBreakLabels, breakLabel)
	cg.loopContLabels = append(cg.loopContLabels, contLabel)
}

func (cg *CodeGen) popLoopLabels() {
	if len(cg.loopBreakLabels) > 0 {
		cg.loopBreakLabels = cg.loopBreakLabels[:len(cg.loopBreakLabels)-1]
	}
	if len(cg.loopContLabels) > 0 {
		cg.loopContLabels = cg.loopContLabels[:len(cg.loopContLabels)-1]
	}
}

func (cg *CodeGen) currentBreakLabel() string {
	if len(cg.loopBreakLabels) == 0 {
		return ""
	}
	return cg.loopBreakLabels[len(cg.loopBreakLabels)-1]
}

func (cg *CodeGen) currentContinueLabel() string {
	if len(cg.loopContLabels) == 0 {
		return ""
	}
	return cg.loopContLabels[len(cg.loopContLabels)-1]
}

func (cg *CodeGen) generateCallExpression(ce *ast.CallExpression) {
	fn, ok := ce.Function.(*ast.Identifier)
	if !ok {
		cg.addNodeError("unsupported call target", ce)
		cg.emit("    mov $0, %%rax")
		return
	}

	switch fn.Value {
	case "print":
		if len(ce.Arguments) != 1 {
			cg.addNodeError("print expects exactly 1 argument", ce)
			cg.emit("    mov $0, %%rax")
			return
		}
		if _, ok := ce.Arguments[0].(*ast.NamedArgument); ok {
			cg.addNodeError("named arguments are not supported for print", ce.Arguments[0])
			cg.emit("    mov $0, %%rax")
			return
		}
		argType := cg.inferExpressionType(ce.Arguments[0])
		prevErrCount := len(cg.errors)
		cg.generateExpression(ce.Arguments[0])
		if len(cg.errors) > prevErrCount {
			cg.emit("    mov $0, %%rax")
			return
		}
		switch argType {
		case typeBool:
			cg.emit("    call print_bool")
		case typeInt:
			cg.emit("    call print_int")
		case typeChar:
			cg.emit("    call print_char")
		case typeString, typeFloat, typeType, typeNull:
			cg.emit("    call print_cstr")
		default:
			cg.addNodeError("print supports only int, bool, float, string, char, null, and type arguments", ce.Arguments[0])
			cg.emit("    mov $0, %%rax")
		}
	case "typeof":
		if len(ce.Arguments) != 1 {
			cg.addNodeError("typeof expects exactly 1 argument", ce)
			cg.emit("    mov $0, %%rax")
			return
		}
		if _, ok := ce.Arguments[0].(*ast.NamedArgument); ok {
			cg.addNodeError("named arguments are not supported for typeof", ce.Arguments[0])
			cg.emit("    mov $0, %%rax")
			return
		}
		typeNameStr := cg.inferExpressionTypeName(ce.Arguments[0])
		if id, ok := ce.Arguments[0].(*ast.Identifier); ok {
			if declared, ok := cg.varDeclaredNames[id.Value]; ok && declared != "" {
				typeNameStr = declared
			}
		}
		label := cg.stringLabel(typeNameStr + "\n")
		cg.emit("    lea %s(%%rip), %%rax", label)
	case "typeofValue", "typeofvalue":
		if len(ce.Arguments) != 1 {
			cg.addNodeError(fmt.Sprintf("%s expects exactly 1 argument", fn.Value), ce)
			cg.emit("    mov $0, %%rax")
			return
		}
		if _, ok := ce.Arguments[0].(*ast.NamedArgument); ok {
			cg.addNodeError(fmt.Sprintf("named arguments are not supported for %s", fn.Value), ce.Arguments[0])
			cg.emit("    mov $0, %%rax")
			return
		}
		typeNameStr := typeName(cg.inferExpressionType(ce.Arguments[0]))
		if typeNameStr == "array" {
			typeNameStr = cg.inferExpressionTypeName(ce.Arguments[0])
		}
		label := cg.stringLabel(typeNameStr + "\n")
		cg.emit("    lea %s(%%rip), %%rax", label)
	case "int", "float", "string", "char", "bool":
		cg.generateCastCall(fn.Value, ce)
	default:
		if key, ok := cg.varFuncs[fn.Value]; ok {
			cg.generateUserFunctionCall(cg.functions[key], ce)
			return
		}
		if key, ok := cg.funcByName[fn.Value]; ok {
			cg.generateUserFunctionCall(cg.functions[key], ce)
			return
		}
		cg.addNodeError("unknown function "+fn.Value, ce)
		cg.emit("    mov $0, %%rax")
	}
}

func (cg *CodeGen) generateBlockStatement(block *ast.BlockStatement) {
	for _, stmt := range block.Statements {
		cg.generateStatement(stmt)
	}
}

func (cg *CodeGen) newLabel() string {
	label := fmt.Sprintf(".L%d", cg.labelCount)
	cg.labelCount++
	return label
}

func (cg *CodeGen) addError(msg string) {
	cg.errors = append(cg.errors, CodegenError{Message: msg})
}

func (cg *CodeGen) addNodeError(msg string, node ast.Node) {
	ctx := ""
	if node != nil {
		ctx = strings.TrimSpace(node.String())
		if ctx == "" {
			ctx = strings.TrimSpace(node.TokenLiteral())
		}
	}
	cg.errors = append(cg.errors, CodegenError{
		Message: msg,
		Context: ctx,
	})
}

func (cg *CodeGen) Errors() []string {
	formatted := make([]string, 0, len(cg.errors))
	for _, err := range cg.errors {
		if err.Context == "" {
			formatted = append(formatted, err.Message)
			continue
		}
		formatted = append(formatted, fmt.Sprintf("%s (at `%s`)", err.Message, err.Context))
	}
	return formatted
}

func (cg *CodeGen) DetailedErrors() []CodegenError {
	out := make([]CodegenError, len(cg.errors))
	copy(out, cg.errors)
	return out
}

func (cg *CodeGen) inferExpressionType(expr ast.Expression) valueType {
	switch e := expr.(type) {
	case *ast.IntegerLiteral:
		return typeInt
	case *ast.FloatLiteral:
		return typeFloat
	case *ast.StringLiteral:
		return typeString
	case *ast.CharLiteral:
		return typeChar
	case *ast.NullLiteral:
		return typeNull
	case *ast.Boolean:
		return typeBool
	case *ast.Identifier:
		if _, ok := cg.intVals[e.Value]; ok {
			return typeInt
		}
		if _, ok := cg.floatVals[e.Value]; ok {
			return typeFloat
		}
		if _, ok := cg.charVals[e.Value]; ok {
			return typeChar
		}
		if _, ok := cg.stringVals[e.Value]; ok {
			return typeString
		}
		if cg.varIsNull[e.Value] {
			return typeNull
		}
		if t, ok := cg.varTypes[e.Value]; ok {
			return t
		}
		if tn, ok := cg.varTypeNames[e.Value]; ok {
			return parseTypeName(tn)
		}
		if isTypeLiteralIdentifier(e.Value) {
			return typeType
		}
		return typeUnknown
	case *ast.ArrayLiteral:
		return typeArray
	case *ast.PrefixExpression:
		switch e.Operator {
		case "!":
			return typeBool
		case "-":
			t := cg.inferExpressionType(e.Right)
			if t == typeInt || t == typeFloat {
				return t
			}
		}
		return typeUnknown
	case *ast.InfixExpression:
		left := cg.inferExpressionType(e.Left)
		right := cg.inferExpressionType(e.Right)
		switch e.Operator {
		case "&&", "||", "^^":
			if left == typeBool && right == typeBool {
				return typeBool
			}
			return typeUnknown
		case "+":
			if left == typeInt && right == typeInt {
				return typeInt
			}
			if isNumericType(left) && isNumericType(right) && (left == typeFloat || right == typeFloat) {
				return typeFloat
			}
			if left == typeString && (right == typeString || right == typeInt || right == typeFloat || right == typeChar) {
				return typeString
			}
			if right == typeString && (left == typeString || left == typeInt || left == typeFloat || left == typeChar) {
				return typeString
			}
			if left == typeChar && right == typeChar {
				return typeChar
			}
			if left == typeChar && right == typeInt {
				return typeChar
			}
			return typeUnknown
		case "-", "*", "/", "%":
			if left == typeInt && right == typeInt {
				return typeInt
			}
			if isNumericType(left) && isNumericType(right) {
				return typeFloat
			}
			return typeUnknown
		case "&", "|", "^", "<<", ">>":
			if left == typeInt && right == typeInt {
				return typeInt
			}
			return typeUnknown
		case "<", ">", "==", "!=":
			return typeBool
		}
		return typeUnknown
	case *ast.IfExpression:
		if e.Alternative == nil {
			return typeUnknown
		}
		cons := cg.inferBlockType(e.Consequence)
		alt := cg.inferBlockType(e.Alternative)
		if cons == alt {
			return cons
		}
		return typeUnknown
	case *ast.CallExpression:
		if fn, ok := e.Function.(*ast.Identifier); ok {
			switch fn.Value {
			case "typeof", "typeofValue", "typeofvalue":
				return typeType
			case "int":
				return typeInt
			case "float":
				return typeFloat
			case "string":
				return typeString
			case "char":
				return typeChar
			case "bool":
				return typeBool
			}
			if key, ok := cg.varFuncs[fn.Value]; ok {
				fl := cg.functions[key]
				if fl.Literal.ReturnType == "" {
					return typeUnknown
				}
				return parseTypeName(fl.Literal.ReturnType)
			}
			if key, ok := cg.funcByName[fn.Value]; ok {
				fl := cg.functions[key]
				if fl.Literal.ReturnType == "" {
					return typeUnknown
				}
				return parseTypeName(fl.Literal.ReturnType)
			}
		}
		return typeUnknown
	case *ast.IndexExpression:
		if cg.inferExpressionTypeName(e.Left) == "string" {
			return typeChar
		}
		elemTypeName, _, ok := peelArrayType(cg.inferExpressionTypeName(e.Left))
		if !ok {
			return typeUnknown
		}
		return parseTypeName(elemTypeName)
	case *ast.MethodCallExpression:
		if e.Method != nil && e.Method.Value == "length" {
			return typeInt
		}
		return typeUnknown
	case *ast.NamedArgument:
		return cg.inferExpressionType(e.Value)
	default:
		return typeUnknown
	}
}

func (cg *CodeGen) inferExpressionTypeName(expr ast.Expression) string {
	switch e := expr.(type) {
	case *ast.Identifier:
		if t, ok := cg.varTypeNames[e.Value]; ok && t != "" {
			return t
		}
		return typeName(cg.inferExpressionType(e))
	case *ast.ArrayLiteral:
		t, ok := cg.inferArrayLiteralTypeName(e)
		if !ok {
			return "unknown"
		}
		return t
	case *ast.IndexExpression:
		if cg.inferExpressionTypeName(e.Left) == "string" {
			return "char"
		}
		elem, _, ok := peelArrayType(cg.inferExpressionTypeName(e.Left))
		if !ok {
			return "unknown"
		}
		return elem
	case *ast.MethodCallExpression:
		if e.Method != nil && e.Method.Value == "length" {
			return "int"
		}
		return "unknown"
	case *ast.CallExpression:
		if fn, ok := e.Function.(*ast.Identifier); ok && (fn.Value == "int" || fn.Value == "float" || fn.Value == "string" || fn.Value == "char" || fn.Value == "bool" || fn.Value == "typeof" || fn.Value == "typeofValue" || fn.Value == "typeofvalue") {
			return typeName(cg.inferExpressionType(e))
		}
	}
	return typeName(cg.inferExpressionType(expr))
}

func (cg *CodeGen) inferArrayLiteralTypeName(al *ast.ArrayLiteral) (string, bool) {
	if al == nil || len(al.Elements) == 0 {
		return "", false
	}
	elemType := ""
	for _, el := range al.Elements {
		cur := cg.inferExpressionTypeName(el)
		if cur == "null" || cur == "unknown" {
			return "", false
		}
		if elemType == "" {
			elemType = cur
			continue
		}
		merged, ok := mergeTypeNames(elemType, cur)
		if !ok {
			return "", false
		}
		elemType = merged
	}
	return withArrayDimension(elemType, len(al.Elements)), true
}

func (cg *CodeGen) inferBlockType(block *ast.BlockStatement) valueType {
	if block == nil || len(block.Statements) == 0 {
		return typeUnknown
	}
	last := block.Statements[len(block.Statements)-1]
	switch s := last.(type) {
	case *ast.ExpressionStatement:
		return cg.inferExpressionType(s.Expression)
	case *ast.ReturnStatement:
		return cg.inferExpressionType(s.ReturnValue)
	default:
		return typeUnknown
	}
}

func (cg *CodeGen) inferTypeofType(expr ast.Expression) valueType {
	if id, ok := expr.(*ast.Identifier); ok {
		if declared, ok := cg.varDeclared[id.Value]; ok && declared != typeUnknown {
			return declared
		}
		if t, ok := cg.varTypes[id.Value]; ok && t != typeUnknown {
			return t
		}
	}
	return cg.inferExpressionType(expr)
}

func (cg *CodeGen) reset() {
	cg.output = strings.Builder{}
	cg.funcDefs = strings.Builder{}
	cg.labelCount = 0
	cg.exitLabel = ""
	cg.normalExit = ""
	cg.variables = make(map[string]int)
	cg.constVars = make(map[string]bool)
	cg.varTypes = make(map[string]valueType)
	cg.varDeclared = make(map[string]valueType)
	cg.varTypeNames = make(map[string]string)
	cg.varDeclaredNames = make(map[string]string)
	cg.varIsNull = make(map[string]bool)
	cg.varArrayLen = make(map[string]int)
	cg.intVals = make(map[string]int64)
	cg.charVals = make(map[string]rune)
	cg.stringVals = make(map[string]string)
	cg.floatVals = make(map[string]float64)
	cg.stringLits = make(map[string]string)
	cg.functions = make(map[string]*compiledFunction)
	cg.funcByName = make(map[string]string)
	cg.varFuncs = make(map[string]string)
	cg.stackOffset = 0
	cg.maxStackOffset = 0
	cg.currentFn = ""
	cg.nextAnonFn = 0
	cg.inFunction = false
	cg.funcRetLbl = ""
	cg.funcRetType = typeUnknown
	cg.funcRetTypeName = ""
	cg.loopBreakLabels = nil
	cg.loopContLabels = nil
	cg.arraySlots = make(map[*ast.ArrayLiteral]int)
	cg.errors = []CodegenError{}
}

func (cg *CodeGen) collectFunctions(program *ast.Program) {
	globalScope := map[string]struct{}{}
	for _, stmt := range program.Statements {
		switch s := stmt.(type) {
		case *ast.FunctionStatement:
			if s != nil && s.Name != nil {
				globalScope[s.Name.Value] = struct{}{}
			}
		case *ast.LetStatement:
			if s != nil && s.Name != nil {
				globalScope[s.Name.Value] = struct{}{}
			}
		case *ast.ConstStatement:
			if s != nil && s.Name != nil {
				globalScope[s.Name.Value] = struct{}{}
			}
		}
	}
	for _, stmt := range program.Statements {
		cg.collectFunctionsInStatement(stmt, globalScope)
	}
}

func (cg *CodeGen) collectFunctionsInStatement(stmt ast.Statement, scope map[string]struct{}) {
	switch s := stmt.(type) {
	case *ast.FunctionStatement:
		if s == nil || s.Name == nil || s.Function == nil {
			return
		}
		key := s.Name.Value
		if _, exists := cg.functions[key]; exists {
			cg.addNodeError("duplicate function declaration: "+key, s)
			return
		}
		captures := cg.computeCaptures(s.Function, scope)
		cg.functions[key] = &compiledFunction{
			Key:      key,
			Name:     s.Name.Value,
			Label:    "fn_" + s.Name.Value,
			Literal:  s.Function,
			Captures: captures,
		}
		cg.funcByName[s.Name.Value] = key

		childScope := copyScope(scope)
		for _, p := range s.Function.Parameters {
			childScope[p.Name.Value] = struct{}{}
		}
		collectDeclaredNames(s.Function.Body, childScope)
		for _, st := range s.Function.Body.Statements {
			cg.collectFunctionsInStatement(st, childScope)
		}
	case *ast.LetStatement:
		if s != nil && s.Value != nil {
			cg.collectFunctionsInExpression(s.Value, scope)
		}
	case *ast.ConstStatement:
		if s != nil && s.Value != nil {
			cg.collectFunctionsInExpression(s.Value, scope)
		}
	case *ast.AssignStatement:
		if s != nil && s.Value != nil {
			cg.collectFunctionsInExpression(s.Value, scope)
		}
	case *ast.IndexAssignStatement:
		if s != nil && s.Left != nil {
			cg.collectFunctionsInExpression(s.Left.Left, scope)
			cg.collectFunctionsInExpression(s.Left.Index, scope)
		}
		if s != nil && s.Value != nil {
			cg.collectFunctionsInExpression(s.Value, scope)
		}
	case *ast.WhileStatement:
		if s != nil && s.Condition != nil {
			cg.collectFunctionsInExpression(s.Condition, scope)
		}
		if s != nil && s.Body != nil {
			for _, st := range s.Body.Statements {
				cg.collectFunctionsInStatement(st, scope)
			}
		}
	case *ast.LoopStatement:
		if s != nil && s.Body != nil {
			for _, st := range s.Body.Statements {
				cg.collectFunctionsInStatement(st, scope)
			}
		}
	case *ast.ForStatement:
		if s != nil && s.Init != nil {
			cg.collectFunctionsInStatement(s.Init, scope)
		}
		if s != nil && s.Condition != nil {
			cg.collectFunctionsInExpression(s.Condition, scope)
		}
		if s != nil && s.Periodic != nil {
			cg.collectFunctionsInStatement(s.Periodic, scope)
		}
		if s != nil && s.Body != nil {
			for _, st := range s.Body.Statements {
				cg.collectFunctionsInStatement(st, scope)
			}
		}
	case *ast.ReturnStatement:
		if s != nil && s.ReturnValue != nil {
			cg.collectFunctionsInExpression(s.ReturnValue, scope)
		}
	case *ast.ExpressionStatement:
		if s != nil && s.Expression != nil {
			cg.collectFunctionsInExpression(s.Expression, scope)
		}
	}
}

func (cg *CodeGen) collectFunctionsInExpression(expr ast.Expression, scope map[string]struct{}) {
	switch e := expr.(type) {
	case *ast.FunctionLiteral:
		key := fmt.Sprintf("lit_%d", cg.nextAnonFn)
		cg.nextAnonFn++
		captures := cg.computeCaptures(e, scope)
		cg.functions[key] = &compiledFunction{
			Key:      key,
			Label:    "fn_" + key,
			Literal:  e,
			Captures: captures,
		}
		childScope := copyScope(scope)
		for _, p := range e.Parameters {
			childScope[p.Name.Value] = struct{}{}
		}
		collectDeclaredNames(e.Body, childScope)
		for _, st := range e.Body.Statements {
			cg.collectFunctionsInStatement(st, childScope)
		}
	case *ast.CallExpression:
		cg.collectFunctionsInExpression(e.Function, scope)
		for _, arg := range e.Arguments {
			cg.collectFunctionsInExpression(arg, scope)
		}
	case *ast.ArrayLiteral:
		for _, el := range e.Elements {
			cg.collectFunctionsInExpression(el, scope)
		}
	case *ast.IndexExpression:
		cg.collectFunctionsInExpression(e.Left, scope)
		cg.collectFunctionsInExpression(e.Index, scope)
	case *ast.MethodCallExpression:
		cg.collectFunctionsInExpression(e.Object, scope)
		for _, arg := range e.Arguments {
			cg.collectFunctionsInExpression(arg, scope)
		}
	case *ast.InfixExpression:
		cg.collectFunctionsInExpression(e.Left, scope)
		cg.collectFunctionsInExpression(e.Right, scope)
	case *ast.PrefixExpression:
		cg.collectFunctionsInExpression(e.Right, scope)
	case *ast.IfExpression:
		cg.collectFunctionsInExpression(e.Condition, scope)
		if e.Consequence != nil {
			for _, st := range e.Consequence.Statements {
				cg.collectFunctionsInStatement(st, scope)
			}
		}
		if e.Alternative != nil {
			for _, st := range e.Alternative.Statements {
				cg.collectFunctionsInStatement(st, scope)
			}
		}
	case *ast.NamedArgument:
		cg.collectFunctionsInExpression(e.Value, scope)
	}
}

func (cg *CodeGen) generateFunctionDefinitions() {
	keys := make([]string, 0, len(cg.functions))
	for k := range cg.functions {
		keys = append(keys, k)
	}
	// deterministic output for tests
	for i := 0; i < len(keys)-1; i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	for _, key := range keys {
		cg.generateOneFunction(cg.functions[key])
	}
}

type cgState struct {
	variables        map[string]int
	constVars        map[string]bool
	varTypes         map[string]valueType
	varDeclared      map[string]valueType
	varTypeNames     map[string]string
	varDeclaredNames map[string]string
	varIsNull        map[string]bool
	varArrayLen      map[string]int
	intVals          map[string]int64
	charVals         map[string]rune
	stringVals       map[string]string
	floatVals        map[string]float64
	varFuncs         map[string]string
	stackOffset      int
	maxStackOffset   int
	inFunction       bool
	funcRetLbl       string
	funcRetType      valueType
	funcRetTypeName  string
	currentFn        string
	loopBreakLabels  []string
	loopContLabels   []string
	arraySlots       map[*ast.ArrayLiteral]int
}

func (cg *CodeGen) saveState() cgState {
	return cgState{
		variables:        cg.variables,
		constVars:        cg.constVars,
		varTypes:         cg.varTypes,
		varDeclared:      cg.varDeclared,
		varTypeNames:     cg.varTypeNames,
		varDeclaredNames: cg.varDeclaredNames,
		varIsNull:        cg.varIsNull,
		varArrayLen:      cg.varArrayLen,
		intVals:          cg.intVals,
		charVals:         cg.charVals,
		stringVals:       cg.stringVals,
		floatVals:        cg.floatVals,
		varFuncs:         cg.varFuncs,
		stackOffset:      cg.stackOffset,
		maxStackOffset:   cg.maxStackOffset,
		inFunction:       cg.inFunction,
		funcRetLbl:       cg.funcRetLbl,
		funcRetType:      cg.funcRetType,
		funcRetTypeName:  cg.funcRetTypeName,
		currentFn:        cg.currentFn,
		loopBreakLabels:  cg.loopBreakLabels,
		loopContLabels:   cg.loopContLabels,
		arraySlots:       cg.arraySlots,
	}
}

func (cg *CodeGen) restoreState(st cgState) {
	cg.variables = st.variables
	cg.constVars = st.constVars
	cg.varTypes = st.varTypes
	cg.varDeclared = st.varDeclared
	cg.varTypeNames = st.varTypeNames
	cg.varDeclaredNames = st.varDeclaredNames
	cg.varIsNull = st.varIsNull
	cg.varArrayLen = st.varArrayLen
	cg.intVals = st.intVals
	cg.charVals = st.charVals
	cg.stringVals = st.stringVals
	cg.floatVals = st.floatVals
	cg.varFuncs = st.varFuncs
	cg.stackOffset = st.stackOffset
	cg.maxStackOffset = st.maxStackOffset
	cg.inFunction = st.inFunction
	cg.funcRetLbl = st.funcRetLbl
	cg.funcRetType = st.funcRetType
	cg.funcRetTypeName = st.funcRetTypeName
	cg.currentFn = st.currentFn
	cg.loopBreakLabels = st.loopBreakLabels
	cg.loopContLabels = st.loopContLabels
	cg.arraySlots = st.arraySlots
}

func (cg *CodeGen) generateOneFunction(fn *compiledFunction) {
	prevOutput := cg.output
	cg.output = strings.Builder{}
	state := cg.saveState()

	cg.variables = make(map[string]int)
	cg.constVars = make(map[string]bool)
	cg.varTypes = make(map[string]valueType)
	cg.varDeclared = make(map[string]valueType)
	cg.varTypeNames = make(map[string]string)
	cg.varDeclaredNames = make(map[string]string)
	cg.varIsNull = make(map[string]bool)
	cg.varArrayLen = make(map[string]int)
	cg.intVals = make(map[string]int64)
	cg.charVals = make(map[string]rune)
	cg.stringVals = make(map[string]string)
	cg.floatVals = make(map[string]float64)
	cg.varFuncs = make(map[string]string)
	cg.stackOffset = 0
	cg.maxStackOffset = 0
	cg.inFunction = true
	cg.funcRetLbl = cg.newLabel()
	cg.funcRetType = parseTypeName(fn.Literal.ReturnType)
	cg.funcRetTypeName = fn.Literal.ReturnType
	if fn.Literal.ReturnType != "" && !isKnownTypeName(fn.Literal.ReturnType) {
		cg.addNodeError("unknown type: "+fn.Literal.ReturnType, fn.Literal)
	}
	cg.currentFn = fn.Key
	cg.arraySlots = make(map[*ast.ArrayLiteral]int)

	label := fn.Label
	cg.emit("%s:", label)
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")
	cg.emit("__STACK_ALLOC_FUNC__")

	paramRegs := []string{"%rdi", "%rsi", "%rdx", "%rcx", "%r8", "%r9"}
	totalParams := len(fn.Literal.Parameters) + len(fn.Captures)
	if totalParams > len(paramRegs) {
		cg.addNodeError("functions with more than 6 total parameters/captures are not supported in codegen", fn.Literal)
	}

	for idx, p := range fn.Literal.Parameters {
		if idx >= len(paramRegs) {
			break
		}
		offset := cg.allocateSlots(1)
		cg.variables[p.Name.Value] = offset
		cg.emit("    mov %s, -%d(%%rbp)  # param %s", paramRegs[idx], offset, p.Name.Value)
		pt := parseTypeName(p.TypeName)
		if p.TypeName != "" && !isKnownTypeName(p.TypeName) {
			cg.addNodeError("unknown type: "+p.TypeName, p.Name)
		}
		cg.varTypes[p.Name.Value] = pt
		cg.varDeclared[p.Name.Value] = pt
		if p.TypeName != "" {
			cg.varTypeNames[p.Name.Value] = p.TypeName
			cg.varDeclaredNames[p.Name.Value] = p.TypeName
			if _, n, ok := peelArrayType(p.TypeName); ok {
				cg.varArrayLen[p.Name.Value] = n
			}
		}
		cg.varIsNull[p.Name.Value] = false
	}

	for idx, name := range fn.Captures {
		regIdx := len(fn.Literal.Parameters) + idx
		if regIdx >= len(paramRegs) {
			break
		}
		offset := cg.allocateSlots(1)
		cg.variables[name] = offset
		cg.emit("    mov %s, -%d(%%rbp)  # capture %s", paramRegs[regIdx], offset, name)
		cg.varTypes[name] = typeUnknown
		cg.varDeclared[name] = typeUnknown
		cg.varTypeNames[name] = "unknown"
		cg.varIsNull[name] = false
	}

	for _, st := range fn.Literal.Body.Statements {
		if fs, ok := st.(*ast.FunctionStatement); ok && fs != nil && fs.Name != nil {
			if key, ok := cg.funcByName[fs.Name.Value]; ok {
				cg.varFuncs[fs.Name.Value] = key
			}
		}
	}

	cg.generateBlockStatement(fn.Literal.Body)
	cg.emit("    mov $0, %%rax")
	cg.emit("%s:", cg.funcRetLbl)
	cg.emit("    mov %%rbp, %%rsp")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	fnAsm := cg.output.String()
	fnAsm = strings.ReplaceAll(fnAsm, "__STACK_ALLOC_FUNC__", cg.stackAllocLine())
	cg.funcDefs.WriteString(fnAsm)
	cg.output = prevOutput
	cg.restoreState(state)
}

func (cg *CodeGen) generateUserFunctionCall(fn *compiledFunction, ce *ast.CallExpression) {
	paramRegs := []string{"%rdi", "%rsi", "%rdx", "%rcx", "%r8", "%r9"}
	totalParams := len(fn.Literal.Parameters) + len(fn.Captures)
	if totalParams > len(paramRegs) {
		cg.addNodeError("functions with more than 6 total parameters/captures are not supported in codegen", ce)
		cg.emit("    mov $0, %%rax")
		return
	}

	finalArgs := make([]ast.Expression, len(fn.Literal.Parameters))
	namedMode := false
	posIdx := 0
	for _, arg := range ce.Arguments {
		if na, ok := arg.(*ast.NamedArgument); ok {
			namedMode = true
			found := -1
			for i, p := range fn.Literal.Parameters {
				if p.Name.Value == na.Name {
					found = i
					break
				}
			}
			if found == -1 {
				cg.addNodeError("unknown named argument: "+na.Name, na)
				cg.emit("    mov $0, %%rax")
				return
			}
			if finalArgs[found] != nil {
				cg.addNodeError("argument provided twice: "+na.Name, na)
				cg.emit("    mov $0, %%rax")
				return
			}
			finalArgs[found] = na.Value
			continue
		}

		if namedMode {
			cg.addNodeError("positional arguments cannot appear after named arguments", ce)
			cg.emit("    mov $0, %%rax")
			return
		}
		if posIdx >= len(fn.Literal.Parameters) {
			cg.addNodeError("too many positional arguments", ce)
			cg.emit("    mov $0, %%rax")
			return
		}
		finalArgs[posIdx] = arg
		posIdx++
	}

	for i, p := range fn.Literal.Parameters {
		if finalArgs[i] != nil {
			continue
		}
		if p.DefaultValue == nil {
			cg.addNodeError("missing required argument: "+p.Name.Value, ce)
			cg.emit("    mov $0, %%rax")
			return
		}
		finalArgs[i] = p.DefaultValue
	}

	for i, arg := range finalArgs {
		want := parseTypeName(fn.Literal.Parameters[i].TypeName)
		got := cg.inferExpressionType(arg)
		wantName := fn.Literal.Parameters[i].TypeName
		if wantName == "" {
			wantName = typeName(want)
		}
		gotName := cg.inferExpressionTypeName(arg)
		if wantName != "unknown" && gotName != "unknown" && !isAssignableTypeName(wantName, gotName) {
			cg.addNodeError(fmt.Sprintf("cannot assign %s to %s", gotName, wantName), ce)
			cg.emit("    mov $0, %%rax")
			return
		}
		if want != typeUnknown && got != typeUnknown && got != typeNull && got != want && parseTypeName(wantName) != typeArray {
			cg.addNodeError(fmt.Sprintf("cannot assign %s to %s", typeName(got), typeName(want)), ce)
			cg.emit("    mov $0, %%rax")
			return
		}
		cg.generateExpression(arg)
		cg.emit("    mov %%rax, %s", paramRegs[i])
	}

	for idx, capName := range fn.Captures {
		regIdx := len(fn.Literal.Parameters) + idx
		if regIdx >= len(paramRegs) {
			break
		}
		cg.generateExpression(&ast.Identifier{Value: capName})
		cg.emit("    mov %%rax, %s", paramRegs[regIdx])
	}

	cg.emit("    call %s", fn.Label)
}

func copyScope(scope map[string]struct{}) map[string]struct{} {
	out := make(map[string]struct{}, len(scope))
	for k := range scope {
		out[k] = struct{}{}
	}
	return out
}

func collectDeclaredNames(block *ast.BlockStatement, scope map[string]struct{}) {
	if block == nil {
		return
	}
	for _, st := range block.Statements {
		switch s := st.(type) {
		case *ast.LetStatement:
			if s != nil && s.Name != nil {
				scope[s.Name.Value] = struct{}{}
			}
		case *ast.ConstStatement:
			if s != nil && s.Name != nil {
				scope[s.Name.Value] = struct{}{}
			}
		case *ast.FunctionStatement:
			if s != nil && s.Name != nil {
				scope[s.Name.Value] = struct{}{}
			}
		case *ast.ForStatement:
			switch init := s.Init.(type) {
			case *ast.LetStatement:
				if init != nil && init.Name != nil {
					scope[init.Name.Value] = struct{}{}
				}
			case *ast.ConstStatement:
				if init != nil && init.Name != nil {
					scope[init.Name.Value] = struct{}{}
				}
			}
		}
	}
}

func (cg *CodeGen) computeCaptures(fn *ast.FunctionLiteral, outerScope map[string]struct{}) []string {
	local := make(map[string]struct{})
	for _, p := range fn.Parameters {
		local[p.Name.Value] = struct{}{}
	}
	collectDeclaredNames(fn.Body, local)

	used := map[string]struct{}{}
	collectUsedNamesInBlock(fn.Body, used)

	captures := []string{}
	for name := range used {
		if _, isLocal := local[name]; isLocal {
			continue
		}
		if _, existsOuter := outerScope[name]; !existsOuter {
			continue
		}
		if isBuiltinName(name) {
			continue
		}
		captures = append(captures, name)
	}

	for i := 0; i < len(captures)-1; i++ {
		for j := i + 1; j < len(captures); j++ {
			if captures[j] < captures[i] {
				captures[i], captures[j] = captures[j], captures[i]
			}
		}
	}
	return captures
}

func isBuiltinName(name string) bool {
	switch name {
	case "print", "typeof", "typeofValue", "typeofvalue", "int", "float", "string", "char", "bool":
		return true
	default:
		return false
	}
}

func isTypeLiteralIdentifier(name string) bool {
	switch name {
	case "int", "bool", "float", "string", "char", "null", "type":
		return true
	default:
		return false
	}
}

func collectUsedNamesInBlock(block *ast.BlockStatement, used map[string]struct{}) {
	if block == nil {
		return
	}
	for _, st := range block.Statements {
		collectUsedNamesInStatement(st, used)
	}
}

func collectUsedNamesInStatement(stmt ast.Statement, used map[string]struct{}) {
	switch s := stmt.(type) {
	case *ast.LetStatement:
		if s != nil && s.Value != nil {
			collectUsedNamesInExpression(s.Value, used)
		}
	case *ast.ConstStatement:
		if s != nil && s.Value != nil {
			collectUsedNamesInExpression(s.Value, used)
		}
	case *ast.AssignStatement:
		if s != nil && s.Name != nil {
			used[s.Name.Value] = struct{}{}
		}
		if s != nil && s.Value != nil {
			collectUsedNamesInExpression(s.Value, used)
		}
	case *ast.IndexAssignStatement:
		if s != nil && s.Left != nil {
			collectUsedNamesInExpression(s.Left.Left, used)
			collectUsedNamesInExpression(s.Left.Index, used)
		}
		if s != nil && s.Value != nil {
			collectUsedNamesInExpression(s.Value, used)
		}
	case *ast.WhileStatement:
		if s != nil && s.Condition != nil {
			collectUsedNamesInExpression(s.Condition, used)
		}
		if s != nil && s.Body != nil {
			collectUsedNamesInBlock(s.Body, used)
		}
	case *ast.LoopStatement:
		if s != nil && s.Body != nil {
			collectUsedNamesInBlock(s.Body, used)
		}
	case *ast.ForStatement:
		if s != nil && s.Init != nil {
			collectUsedNamesInStatement(s.Init, used)
		}
		if s != nil && s.Condition != nil {
			collectUsedNamesInExpression(s.Condition, used)
		}
		if s != nil && s.Periodic != nil {
			collectUsedNamesInStatement(s.Periodic, used)
		}
		if s != nil && s.Body != nil {
			collectUsedNamesInBlock(s.Body, used)
		}
	case *ast.ReturnStatement:
		if s != nil && s.ReturnValue != nil {
			collectUsedNamesInExpression(s.ReturnValue, used)
		}
	case *ast.ExpressionStatement:
		if s != nil && s.Expression != nil {
			collectUsedNamesInExpression(s.Expression, used)
		}
	case *ast.FunctionStatement:
		// Nested function bodies are handled independently; skip here.
		return
	}
}

func collectUsedNamesInExpression(expr ast.Expression, used map[string]struct{}) {
	switch e := expr.(type) {
	case *ast.Identifier:
		used[e.Value] = struct{}{}
	case *ast.PrefixExpression:
		collectUsedNamesInExpression(e.Right, used)
	case *ast.InfixExpression:
		collectUsedNamesInExpression(e.Left, used)
		collectUsedNamesInExpression(e.Right, used)
	case *ast.IfExpression:
		collectUsedNamesInExpression(e.Condition, used)
		collectUsedNamesInBlock(e.Consequence, used)
		collectUsedNamesInBlock(e.Alternative, used)
	case *ast.CallExpression:
		collectUsedNamesInExpression(e.Function, used)
		for _, arg := range e.Arguments {
			collectUsedNamesInExpression(arg, used)
		}
	case *ast.ArrayLiteral:
		for _, el := range e.Elements {
			collectUsedNamesInExpression(el, used)
		}
	case *ast.IndexExpression:
		collectUsedNamesInExpression(e.Left, used)
		collectUsedNamesInExpression(e.Index, used)
	case *ast.MethodCallExpression:
		collectUsedNamesInExpression(e.Object, used)
		for _, arg := range e.Arguments {
			collectUsedNamesInExpression(arg, used)
		}
	case *ast.NamedArgument:
		collectUsedNamesInExpression(e.Value, used)
	case *ast.FunctionLiteral:
		// Nested function body capture set should be computed independently.
		return
	}
}

func (cg *CodeGen) generateCastCall(castName string, ce *ast.CallExpression) {
	if len(ce.Arguments) != 1 {
		cg.addNodeError(fmt.Sprintf("%s expects exactly 1 argument", castName), ce)
		cg.emit("    mov $0, %%rax")
		return
	}
	argType := cg.inferExpressionType(ce.Arguments[0])
	cg.generateExpression(ce.Arguments[0])

	switch castName {
	case "int":
		if argType == typeInt || argType == typeBool || argType == typeChar {
			return
		}
	case "bool":
		if argType == typeBool {
			return
		}
		if argType == typeInt || argType == typeChar {
			cg.emit("    test %%rax, %%rax")
			cg.emit("    setne %%al")
			cg.emit("    movzbq %%al, %%rax")
			return
		}
	case "char":
		if argType == typeChar || argType == typeInt || argType == typeBool {
			return
		}
	case "string":
		if argType == typeString || argType == typeFloat || argType == typeType || argType == typeNull {
			return
		}
	case "float":
		if argType == typeFloat {
			return
		}
	}

	cg.addNodeError(fmt.Sprintf("cannot cast %s to %s", typeName(argType), castName), ce)
	cg.emit("    mov $0, %%rax")
}

func (cg *CodeGen) constStringValue(expr ast.Expression) (string, bool) {
	switch e := expr.(type) {
	case *ast.StringLiteral:
		return e.Value, true
	case *ast.Identifier:
		v, ok := cg.stringVals[e.Value]
		return v, ok
	case *ast.InfixExpression:
		if e.Operator != "+" {
			return "", false
		}
		left, ok := cg.constStringValue(e.Left)
		if !ok {
			return "", false
		}
		if right, ok := cg.constStringValue(e.Right); ok {
			return left + right, true
		}
		if right, ok := cg.constIntValue(e.Right); ok {
			return left + fmt.Sprintf("%d", right), true
		}
		if right, ok := cg.constFloatValue(e.Right); ok {
			return left + fmt.Sprintf("%g", right), true
		}
		if right, ok := cg.constCharValue(e.Right); ok {
			return left + string(right), true
		}
		return "", false
	default:
		return "", false
	}
}

func (cg *CodeGen) constFloatValue(expr ast.Expression) (float64, bool) {
	switch e := expr.(type) {
	case *ast.IntegerLiteral:
		return float64(e.Value), true
	case *ast.FloatLiteral:
		return e.Value, true
	case *ast.Identifier:
		if v, ok := cg.floatVals[e.Value]; ok {
			return v, true
		}
		if v, ok := cg.intVals[e.Value]; ok {
			return float64(v), true
		}
		return 0, false
	case *ast.PrefixExpression:
		if e.Operator != "-" {
			return 0, false
		}
		v, ok := cg.constFloatValue(e.Right)
		if !ok {
			return 0, false
		}
		return -v, true
	case *ast.InfixExpression:
		left, ok := cg.constFloatValue(e.Left)
		if !ok {
			return 0, false
		}
		right, ok := cg.constFloatValue(e.Right)
		if !ok {
			return 0, false
		}
		switch e.Operator {
		case "+":
			return left + right, true
		case "-":
			return left - right, true
		case "*":
			return left * right, true
		case "/":
			return left / right, true
		case "%":
			return math.Mod(left, right), true
		default:
			return 0, false
		}
	default:
		return 0, false
	}
}

func (cg *CodeGen) constIntValue(expr ast.Expression) (int64, bool) {
	switch e := expr.(type) {
	case *ast.IntegerLiteral:
		return e.Value, true
	case *ast.Identifier:
		v, ok := cg.intVals[e.Value]
		return v, ok
	case *ast.PrefixExpression:
		if e.Operator != "-" {
			return 0, false
		}
		v, ok := cg.constIntValue(e.Right)
		if !ok {
			return 0, false
		}
		return -v, true
	case *ast.InfixExpression:
		left, ok := cg.constIntValue(e.Left)
		if !ok {
			return 0, false
		}
		right, ok := cg.constIntValue(e.Right)
		if !ok {
			return 0, false
		}
		switch e.Operator {
		case "+":
			return left + right, true
		case "-":
			return left - right, true
		case "*":
			return left * right, true
		case "/":
			return left / right, true
		case "%":
			return left % right, true
		default:
			return 0, false
		}
	default:
		return 0, false
	}
}

func (cg *CodeGen) constCharValue(expr ast.Expression) (rune, bool) {
	switch e := expr.(type) {
	case *ast.CharLiteral:
		return e.Value, true
	case *ast.Identifier:
		v, ok := cg.charVals[e.Value]
		return v, ok
	case *ast.InfixExpression:
		if e.Operator == "+" {
			if left, ok := cg.constCharValue(e.Left); ok {
				if right, ok := cg.constCharValue(e.Right); ok {
					return left + right, true
				}
				if right, ok := cg.constIntValue(e.Right); ok {
					return left + rune(right), true
				}
			}
		}
		return 0, false
	default:
		return 0, false
	}
}

func (cg *CodeGen) trackKnownValue(name string, t valueType, expr ast.Expression) {
	delete(cg.intVals, name)
	delete(cg.charVals, name)
	delete(cg.stringVals, name)
	delete(cg.floatVals, name)
	if expr == nil {
		return
	}
	switch t {
	case typeInt:
		if v, ok := cg.constIntValue(expr); ok {
			cg.intVals[name] = v
		}
	case typeChar:
		if v, ok := cg.constCharValue(expr); ok {
			cg.charVals[name] = v
		}
	case typeString:
		if v, ok := cg.constStringValue(expr); ok {
			cg.stringVals[name] = v
		}
	case typeFloat:
		if v, ok := cg.constFloatValue(expr); ok {
			cg.floatVals[name] = v
		}
	}
}

func isNumericType(t valueType) bool {
	return t == typeInt || t == typeFloat
}

func parseTypeName(s string) valueType {
	if _, dims, ok := parseTypeDescriptor(s); ok && len(dims) > 0 {
		return typeArray
	}
	switch s {
	case "int":
		return typeInt
	case "bool":
		return typeBool
	case "float":
		return typeFloat
	case "string":
		return typeString
	case "char":
		return typeChar
	case "type":
		return typeType
	case "null":
		return typeNull
	default:
		return typeUnknown
	}
}

func typeName(t valueType) string {
	switch t {
	case typeInt:
		return "int"
	case typeBool:
		return "bool"
	case typeFloat:
		return "float"
	case typeString:
		return "string"
	case typeChar:
		return "char"
	case typeNull:
		return "null"
	case typeType:
		return "type"
	case typeArray:
		return "array"
	default:
		return "unknown"
	}
}

func parseTypeDescriptor(t string) (string, []int, bool) {
	if t == "" {
		return "", nil, false
	}
	base := t
	dimsRev := make([]int, 0, 2)
	for strings.HasSuffix(base, "]") {
		open := strings.LastIndexByte(base, '[')
		if open <= 0 {
			return "", nil, false
		}
		sizeLit := base[open+1 : len(base)-1]
		if sizeLit == "" {
			dimsRev = append(dimsRev, -1)
		} else {
			var size int
			if _, err := fmt.Sscanf(sizeLit, "%d", &size); err != nil || size < 0 {
				return "", nil, false
			}
			dimsRev = append(dimsRev, size)
		}
		base = base[:open]
	}
	base = stripOuterParens(base)
	if base == "" {
		return "", nil, false
	}
	dims := make([]int, len(dimsRev))
	for i := range dimsRev {
		dims[len(dimsRev)-1-i] = dimsRev[i]
	}
	return base, dims, true
}

func formatTypeDescriptor(base string, dims []int) string {
	if len(dims) > 0 {
		if _, isUnion := splitTopLevelUnion(base); isUnion && !isWrappedInParens(base) {
			base = "(" + base + ")"
		}
	}
	out := base
	for _, d := range dims {
		if d < 0 {
			out += "[]"
		} else {
			out += fmt.Sprintf("[%d]", d)
		}
	}
	return out
}

func peelArrayType(t string) (string, int, bool) {
	base, dims, ok := parseTypeDescriptor(t)
	if !ok || len(dims) == 0 {
		return "", 0, false
	}
	elem := formatTypeDescriptor(base, dims[:len(dims)-1])
	return elem, dims[len(dims)-1], true
}

func withArrayDimension(elem string, n int) string {
	if n < 0 {
		return formatTypeDescriptor(elem, []int{-1})
	}
	return formatTypeDescriptor(elem, []int{n})
}

func mergeTypeNames(a, b string) (string, bool) {
	if a == b {
		return a, true
	}
	baseA, dimsA, okA := parseTypeDescriptor(a)
	baseB, dimsB, okB := parseTypeDescriptor(b)
	if !okA || !okB || len(dimsA) != len(dimsB) {
		return "", false
	}
	merged := make([]int, len(dimsA))
	for i := range dimsA {
		if dimsA[i] == dimsB[i] {
			merged[i] = dimsA[i]
		} else {
			merged[i] = -1
		}
	}
	mergedBase, ok := mergeUnionBases(baseA, baseB)
	if !ok {
		return "", false
	}
	return formatTypeDescriptor(mergedBase, merged), true
}

func isAssignableTypeName(target, value string) bool {
	if target == "" || target == "unknown" {
		return true
	}
	if value == "null" {
		return true
	}
	if target == value {
		return true
	}
	if targetMembers, targetIsUnion := splitTopLevelUnion(target); targetIsUnion {
		for _, m := range targetMembers {
			if isAssignableTypeName(m, value) {
				return true
			}
		}
		return false
	}
	if valueMembers, valueIsUnion := splitTopLevelUnion(value); valueIsUnion {
		for _, m := range valueMembers {
			if !isAssignableTypeName(target, m) {
				return false
			}
		}
		return true
	}
	tb, td, okT := parseTypeDescriptor(target)
	vb, vd, okV := parseTypeDescriptor(value)
	if !okT || !okV || len(td) != len(vd) {
		return false
	}
	for i := range td {
		if td[i] == -1 {
			continue
		}
		if td[i] != vd[i] {
			return false
		}
	}
	if len(td) == 0 {
		return tb == vb
	}
	return isAssignableTypeName(tb, vb)
}

func isKnownTypeName(t string) bool {
	base, _, ok := parseTypeDescriptor(t)
	if !ok {
		return false
	}
	if parts, isUnion := splitTopLevelUnion(base); isUnion {
		for _, p := range parts {
			if !isKnownTypeName(p) {
				return false
			}
		}
		return true
	}
	switch base {
	case "int", "bool", "float", "string", "char", "null", "type":
		return true
	default:
		return false
	}
}

func splitTopLevelUnion(t string) ([]string, bool) {
	s := stripOuterParens(t)
	parts := []string{}
	depth := 0
	start := 0
	found := false
	for i := 0; i < len(s)-1; i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case '|':
			if depth == 0 && s[i+1] == '|' {
				part := strings.TrimSpace(s[start:i])
				if part == "" {
					return nil, false
				}
				parts = append(parts, stripOuterParens(part))
				start = i + 2
				found = true
				i++
			}
		}
	}
	if !found {
		return nil, false
	}
	last := strings.TrimSpace(s[start:])
	if last == "" {
		return nil, false
	}
	parts = append(parts, stripOuterParens(last))
	return parts, true
}

func stripOuterParens(s string) string {
	out := strings.TrimSpace(s)
	for isWrappedInParens(out) {
		out = strings.TrimSpace(out[1 : len(out)-1])
	}
	return out
}

func isWrappedInParens(s string) bool {
	if len(s) < 2 || s[0] != '(' || s[len(s)-1] != ')' {
		return false
	}
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 && i != len(s)-1 {
				return false
			}
		}
		if depth < 0 {
			return false
		}
	}
	return depth == 0
}

func mergeUnionBases(a, b string) (string, bool) {
	listA := []string{stripOuterParens(a)}
	if parts, ok := splitTopLevelUnion(a); ok {
		listA = parts
	}
	listB := []string{stripOuterParens(b)}
	if parts, ok := splitTopLevelUnion(b); ok {
		listB = parts
	}
	merged := make([]string, 0, len(listA)+len(listB))
	seen := map[string]struct{}{}
	for _, x := range append(listA, listB...) {
		if x == "" {
			return "", false
		}
		if _, ok := seen[x]; ok {
			continue
		}
		seen[x] = struct{}{}
		merged = append(merged, x)
	}
	if len(merged) == 0 {
		return "", false
	}
	if len(merged) == 1 {
		return merged[0], true
	}
	return strings.Join(merged, "||"), true
}

func (cg *CodeGen) stringLabel(lit string) string {
	if label, ok := cg.stringLits[lit]; ok {
		return label
	}
	label := fmt.Sprintf("str_%d", len(cg.stringLits))
	cg.stringLits[lit] = label
	return label
}

func escapeAsmString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}
