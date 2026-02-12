package codegen

import (
	"fmt"
	"strings"

	"twice/internal/ast"
)

// CodeGen holds the state for code generation
type CodeGen struct {
	output      strings.Builder
	labelCount  int
	variables   map[string]int // name -> stack offset
	stackOffset int            // current stack position
}

// New creates a new code generator
func New() *CodeGen {
	return &CodeGen{
		variables:   make(map[string]int),
		stackOffset: 0,
	}
}

// Generate produces x86-64 assembly from a program
func (cg *CodeGen) Generate(program *ast.Program) string {
	cg.emitHeader()

	// Generate code for each statement
	for _, stmt := range program.Statements {
		cg.generateStatement(stmt)
	}

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
	cg.emit("_start:")
	cg.emit("    push %%rbp           # save old base pointer")
	cg.emit("    mov %%rsp, %%rbp     # set new base pointer")
}

// emitFooter outputs exit syscall and data section
func (cg *CodeGen) emitFooter() {
	cg.emit("")
	cg.emit("    # Exit with result in rax")
	cg.emit("    mov %%rbp, %%rsp     # restore stack pointer")
	cg.emit("    pop %%rbp            # restore base pointer")
	cg.emit("    mov %%rax, %%rdi     # exit code")
	cg.emit("    mov $60, %%rax       # syscall: exit")
	cg.emit("    syscall")
}

func (cg *CodeGen) generateStatement(stmt ast.Statement) {
	switch s := stmt.(type) {
	case *ast.LetStatement:
		cg.generateLet(s)
	case *ast.ReturnStatement:
		cg.generateReturn(s)
	case *ast.ExpressionStatement:
		cg.generateExpression(s.Expression)
	}
}

// generateExpression dispatches to specific expression generators
func (cg *CodeGen) generateExpression(expr ast.Expression) {
	switch e := expr.(type) {
	case *ast.IntegerLiteral:
		cg.generateInteger(e)
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
	}
}

// generateInteger loads an integer into rax
func (cg *CodeGen) generateInteger(il *ast.IntegerLiteral) {
	cg.emit("    mov $%d, %%rax", il.Value)
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
		cg.emit("    # ERROR: undefined variable %s", i.Value)
		return
	}
	// Load from stack: rbp - offset
	cg.emit("    mov -%d(%%rbp), %%rax  # load %s", offset, i.Value)
}

// generateLet handles variable declarations (simplified - no stack frame yet)
func (cg *CodeGen) generateLet(ls *ast.LetStatement) {
	cg.generateExpression(ls.Value)

	// Allocate space on stack and store
	cg.stackOffset += 8 // 8 bytes for int64
	name := ls.Name.Value
	cg.variables[name] = cg.stackOffset

	cg.emit("    push %%rax           # let %s", name)
}

// generateReturn handles return statements
func (cg *CodeGen) generateReturn(rs *ast.ReturnStatement) {
	cg.generateExpression(rs.ReturnValue)
	// Result is already in rax, which is our return value convention
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