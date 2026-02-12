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
	output      strings.Builder
	funcDefs    strings.Builder
	labelCount  int
	exitLabel   string
	normalExit  string
	variables   map[string]int // name -> stack offset
	constVars   map[string]bool
	varTypes    map[string]valueType
	varDeclared map[string]valueType
	varIsNull   map[string]bool
	intVals     map[string]int64
	charVals    map[string]rune
	stringVals  map[string]string
	floatVals   map[string]float64
	stringLits  map[string]string
	stackOffset int // current stack position
	functions   map[string]*ast.FunctionLiteral
	funcLabels  map[string]string
	inFunction  bool
	funcRetLbl  string
	funcRetType valueType
	errors      []CodegenError
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
)

type CodegenError struct {
	Message string
	Context string
}

// New creates a new code generator
func New() *CodeGen {
	return &CodeGen{
		variables:   make(map[string]int),
		constVars:   make(map[string]bool),
		varTypes:    make(map[string]valueType),
		varDeclared: make(map[string]valueType),
		varIsNull:   make(map[string]bool),
		intVals:     make(map[string]int64),
		charVals:    make(map[string]rune),
		stringVals:  make(map[string]string),
		floatVals:   make(map[string]float64),
		stringLits:  make(map[string]string),
		functions:   make(map[string]*ast.FunctionLiteral),
		funcLabels:  make(map[string]string),
		stackOffset: 0,
		errors:      []CodegenError{},
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
	return cg.output.String()
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

	// Entry point
	cg.emit("_start:")
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")
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
}

func (cg *CodeGen) generateStatement(stmt ast.Statement) {
	switch s := stmt.(type) {
	case *ast.LetStatement:
		cg.generateLet(s)
	case *ast.ConstStatement:
		cg.generateConst(s)
	case *ast.AssignStatement:
		cg.generateAssign(s)
	case *ast.ReturnStatement:
		cg.generateReturn(s)
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
			cg.addNodeError("string concatenation in codegen requires compile-time known values", ie)
			cg.emit("    mov $0, %%rax")
			return
		}
		label := cg.stringLabel(combined + "\n")
		cg.emit("    lea %s(%%rip), %%rax", label)
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
		cg.addNodeError("identifier not found: "+i.Value, i)
		cg.emit("    mov $0, %%rax")
		return
	}
	if cg.varIsNull[i.Value] {
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
	if ls.TypeName != "" && declared == typeUnknown {
		cg.addNodeError("unknown type: "+ls.TypeName, ls)
		cg.emit("    mov $0, %%rax")
		return
	}
	if declared != typeUnknown {
		cg.varDeclared[ls.Name.Value] = declared
	}

	if ls.Value == nil {
		cg.generateNull(&ast.NullLiteral{})
	} else {
		cg.generateExpression(ls.Value)
	}

	// Allocate space on stack and store
	cg.stackOffset += 8 // 8 bytes for int64
	name := ls.Name.Value
	cg.variables[name] = cg.stackOffset
	inferred := typeNull
	if ls.Value != nil {
		inferred = cg.inferExpressionType(ls.Value)
	}
	if declared != typeUnknown && inferred != typeNull && declared != inferred {
		cg.addNodeError(fmt.Sprintf("cannot assign %s to %s", typeName(inferred), typeName(declared)), ls)
		cg.emit("    mov $0, %%rax")
		return
	}
	if declared != typeUnknown {
		cg.varTypes[name] = declared
	} else {
		cg.varTypes[name] = inferred
	}
	cg.varIsNull[name] = inferred == typeNull
	cg.trackKnownValue(name, cg.varTypes[name], ls.Value)

	cg.emit("    push %%rax           # let %s", name)
}

// generateConst handles immutable variable declarations.
func (cg *CodeGen) generateConst(cs *ast.ConstStatement) {
	if _, exists := cg.variables[cs.Name.Value]; exists {
		cg.addNodeError("identifier already declared: "+cs.Name.Value, cs)
		cg.emit("    mov $0, %%rax")
		return
	}

	declared := parseTypeName(cs.TypeName)
	if cs.TypeName != "" && declared == typeUnknown {
		cg.addNodeError("unknown type: "+cs.TypeName, cs)
		cg.emit("    mov $0, %%rax")
		return
	}
	if declared != typeUnknown {
		cg.varDeclared[cs.Name.Value] = declared
	}

	cg.generateExpression(cs.Value)

	cg.stackOffset += 8
	name := cs.Name.Value
	cg.variables[name] = cg.stackOffset
	cg.constVars[name] = true
	inferred := cg.inferExpressionType(cs.Value)
	if declared != typeUnknown && inferred != typeNull && declared != inferred {
		cg.addNodeError(fmt.Sprintf("cannot assign %s to %s", typeName(inferred), typeName(declared)), cs)
		cg.emit("    mov $0, %%rax")
		return
	}
	if declared != typeUnknown {
		cg.varTypes[name] = declared
	} else {
		cg.varTypes[name] = inferred
	}
	cg.varIsNull[name] = inferred == typeNull
	cg.trackKnownValue(name, cg.varTypes[name], cs.Value)

	cg.emit("    push %%rax           # const %s", name)
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
	target := cg.varTypes[as.Name.Value]
	if target == typeNull && inferred != typeNull {
		target = inferred
	}
	if declared, ok := cg.varDeclared[as.Name.Value]; ok {
		target = declared
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
	if inferred != typeNull && target != typeUnknown && target != inferred {
		cg.addNodeError(fmt.Sprintf("cannot assign %s to %s", typeName(inferred), typeName(target)), as)
		cg.emit("    mov $0, %%rax")
		return
	}
	if target == typeUnknown {
		cg.varTypes[as.Name.Value] = inferred
	} else {
		cg.varTypes[as.Name.Value] = target
	}
	cg.varIsNull[as.Name.Value] = inferred == typeNull
	cg.trackKnownValue(as.Name.Value, cg.varTypes[as.Name.Value], as.Value)
}

// generateReturn handles return statements
func (cg *CodeGen) generateReturn(rs *ast.ReturnStatement) {
	cg.generateExpression(rs.ReturnValue)
	if cg.inFunction {
		if cg.funcRetType != typeUnknown {
			got := cg.inferExpressionType(rs.ReturnValue)
			if got != typeNull && got != typeUnknown && got != cg.funcRetType {
				cg.addNodeError(fmt.Sprintf("cannot return %s from function returning %s", typeName(got), typeName(cg.funcRetType)), rs)
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
		t := cg.inferTypeofType(ce.Arguments[0])
		label := cg.stringLabel(typeName(t) + "\n")
		cg.emit("    lea %s(%%rip), %%rax", label)
	case "int", "float", "string", "char", "bool":
		cg.generateCastCall(fn.Value, ce)
	default:
		if fl, ok := cg.functions[fn.Value]; ok {
			cg.generateUserFunctionCall(fn.Value, fl, ce)
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
		if cg.varIsNull[e.Value] {
			return typeNull
		}
		if t, ok := cg.varTypes[e.Value]; ok {
			return t
		}
		return typeUnknown
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
			case "typeof":
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
			if fl, ok := cg.functions[fn.Value]; ok {
				if fl.ReturnType == "" {
					return typeUnknown
				}
				return parseTypeName(fl.ReturnType)
			}
		}
		return typeUnknown
	case *ast.NamedArgument:
		return cg.inferExpressionType(e.Value)
	default:
		return typeUnknown
	}
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
	cg.varIsNull = make(map[string]bool)
	cg.intVals = make(map[string]int64)
	cg.charVals = make(map[string]rune)
	cg.stringVals = make(map[string]string)
	cg.floatVals = make(map[string]float64)
	cg.stringLits = make(map[string]string)
	cg.functions = make(map[string]*ast.FunctionLiteral)
	cg.funcLabels = make(map[string]string)
	cg.stackOffset = 0
	cg.inFunction = false
	cg.funcRetLbl = ""
	cg.funcRetType = typeUnknown
	cg.errors = []CodegenError{}
}

func (cg *CodeGen) collectFunctions(program *ast.Program) {
	for _, stmt := range program.Statements {
		fs, ok := stmt.(*ast.FunctionStatement)
		if !ok || fs == nil || fs.Name == nil || fs.Function == nil {
			continue
		}
		name := fs.Name.Value
		if _, exists := cg.functions[name]; exists {
			cg.addNodeError("duplicate function declaration: "+name, fs)
			continue
		}
		cg.functions[name] = fs.Function
		cg.funcLabels[name] = "fn_" + name
	}
}

func (cg *CodeGen) generateFunctionDefinitions() {
	for name, fn := range cg.functions {
		cg.generateOneFunction(name, fn)
	}
}

type cgState struct {
	variables   map[string]int
	constVars   map[string]bool
	varTypes    map[string]valueType
	varDeclared map[string]valueType
	varIsNull   map[string]bool
	intVals     map[string]int64
	charVals    map[string]rune
	stringVals  map[string]string
	floatVals   map[string]float64
	stackOffset int
	inFunction  bool
	funcRetLbl  string
	funcRetType valueType
}

func (cg *CodeGen) saveState() cgState {
	return cgState{
		variables:   cg.variables,
		constVars:   cg.constVars,
		varTypes:    cg.varTypes,
		varDeclared: cg.varDeclared,
		varIsNull:   cg.varIsNull,
		intVals:     cg.intVals,
		charVals:    cg.charVals,
		stringVals:  cg.stringVals,
		floatVals:   cg.floatVals,
		stackOffset: cg.stackOffset,
		inFunction:  cg.inFunction,
		funcRetLbl:  cg.funcRetLbl,
		funcRetType: cg.funcRetType,
	}
}

func (cg *CodeGen) restoreState(st cgState) {
	cg.variables = st.variables
	cg.constVars = st.constVars
	cg.varTypes = st.varTypes
	cg.varDeclared = st.varDeclared
	cg.varIsNull = st.varIsNull
	cg.intVals = st.intVals
	cg.charVals = st.charVals
	cg.stringVals = st.stringVals
	cg.floatVals = st.floatVals
	cg.stackOffset = st.stackOffset
	cg.inFunction = st.inFunction
	cg.funcRetLbl = st.funcRetLbl
	cg.funcRetType = st.funcRetType
}

func (cg *CodeGen) generateOneFunction(name string, fn *ast.FunctionLiteral) {
	prevOutput := cg.output
	cg.output = strings.Builder{}
	state := cg.saveState()

	cg.variables = make(map[string]int)
	cg.constVars = make(map[string]bool)
	cg.varTypes = make(map[string]valueType)
	cg.varDeclared = make(map[string]valueType)
	cg.varIsNull = make(map[string]bool)
	cg.intVals = make(map[string]int64)
	cg.charVals = make(map[string]rune)
	cg.stringVals = make(map[string]string)
	cg.floatVals = make(map[string]float64)
	cg.stackOffset = 0
	cg.inFunction = true
	cg.funcRetLbl = cg.newLabel()
	cg.funcRetType = parseTypeName(fn.ReturnType)

	label := cg.funcLabels[name]
	cg.emit("%s:", label)
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")

	paramRegs := []string{"%rdi", "%rsi", "%rdx", "%rcx", "%r8", "%r9"}
	if len(fn.Parameters) > len(paramRegs) {
		cg.addNodeError("functions with more than 6 parameters are not supported in codegen", fn)
	}

	for idx, p := range fn.Parameters {
		if idx >= len(paramRegs) {
			break
		}
		cg.emit("    push %s           # param %s", paramRegs[idx], p.Name.Value)
		cg.stackOffset += 8
		cg.variables[p.Name.Value] = cg.stackOffset
		pt := parseTypeName(p.TypeName)
		if p.TypeName != "" && pt == typeUnknown {
			cg.addNodeError("unknown type: "+p.TypeName, p.Name)
		}
		cg.varTypes[p.Name.Value] = pt
		cg.varDeclared[p.Name.Value] = pt
		cg.varIsNull[p.Name.Value] = false
	}

	cg.generateBlockStatement(fn.Body)
	cg.emit("    mov $0, %%rax")
	cg.emit("%s:", cg.funcRetLbl)
	cg.emit("    mov %%rbp, %%rsp")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")

	cg.funcDefs.WriteString(cg.output.String())
	cg.output = prevOutput
	cg.restoreState(state)
}

func (cg *CodeGen) generateUserFunctionCall(name string, fn *ast.FunctionLiteral, ce *ast.CallExpression) {
	paramRegs := []string{"%rdi", "%rsi", "%rdx", "%rcx", "%r8", "%r9"}
	if len(fn.Parameters) > len(paramRegs) {
		cg.addNodeError("functions with more than 6 parameters are not supported in codegen", ce)
		cg.emit("    mov $0, %%rax")
		return
	}

	finalArgs := make([]ast.Expression, len(fn.Parameters))
	namedMode := false
	posIdx := 0
	for _, arg := range ce.Arguments {
		if na, ok := arg.(*ast.NamedArgument); ok {
			namedMode = true
			found := -1
			for i, p := range fn.Parameters {
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
		if posIdx >= len(fn.Parameters) {
			cg.addNodeError("too many positional arguments", ce)
			cg.emit("    mov $0, %%rax")
			return
		}
		finalArgs[posIdx] = arg
		posIdx++
	}

	for i, p := range fn.Parameters {
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
		want := parseTypeName(fn.Parameters[i].TypeName)
		got := cg.inferExpressionType(arg)
		if want != typeUnknown && got != typeUnknown && got != typeNull && got != want {
			cg.addNodeError(fmt.Sprintf("cannot assign %s to %s", typeName(got), typeName(want)), ce)
			cg.emit("    mov $0, %%rax")
			return
		}
		cg.generateExpression(arg)
		cg.emit("    mov %%rax, %s", paramRegs[i])
	}

	cg.emit("    call %s", cg.funcLabels[name])
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
	default:
		return "unknown"
	}
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
