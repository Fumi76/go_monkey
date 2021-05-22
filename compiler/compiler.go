package compiler

import (
	"fmt"
	"log"
	"sort"

	"example.com/monkey/ast"
	"example.com/monkey/code"
	"example.com/monkey/object"
)

type Compiler struct {

	// constant pool
	constants []object.Object

	symbolTable *SymbolTable

	scopes     []CompilationScope
	scopeIndex int
}

type EmittedInstruction struct {
	Opcode   code.Opcode
	Position int
}

func New() *Compiler {

	mainScope := CompilationScope{
		instructions:        code.Instructions{},
		lastInstruction:     EmittedInstruction{},
		previousInstruction: EmittedInstruction{},
	}

	symbolTable := NewSymbolTable()

	for i, v := range object.Builtins {

		symbolTable.DefineBuiltin(i, v.Name)
	}

	return &Compiler{
		constants:   []object.Object{},
		symbolTable: symbolTable,
		scopes:      []CompilationScope{mainScope},
		scopeIndex:  0,
	}
}

func (c *Compiler) loadSymbol(s Symbol) {

	switch s.Scope {

	case GlobalScope:
		c.emit(code.OpGetGlobal, s.Index)

	case LocalScope:
		c.emit(code.OpGetLocal, s.Index)

	case BuiltinScope:
		c.emit(code.OpGetBuiltin, s.Index)

	case FreeScope:
		c.emit(code.OpGetFree, s.Index)

	case FunctionScope:
		c.emit(code.OpCurrentClosure)
	}
}

func NewWithState(s *SymbolTable, constants []object.Object) *Compiler {

	compiler := New()
	compiler.symbolTable = s
	compiler.constants = constants
	return compiler
}

func (c *Compiler) Bytecode() *Bytecode {

	return &Bytecode{
		Instructions: c.currentInstructions(),
		Constants:    c.constants,
	}
}

// コンパイラーが出力するものであり、
// 仮想マシンにこれが渡される
type Bytecode struct {
	// bytecode
	Instructions code.Instructions
	// constant pool
	Constants []object.Object
}

func (c *Compiler) Compile(node ast.Node) error {

	switch node := node.(type) {

	case *ast.Program:
		log.Println("program...")
		for _, s := range node.Statements {
			err := c.Compile(s)
			if err != nil {
				return err
			}
		}

	case *ast.FunctionLiteral:

		c.enterScope()

		if node.Name != "" {

			c.symbolTable.DefineFunctionName(node.Name)
		}

		for _, p := range node.Parameters {

			c.symbolTable.Define(p.Value)
		}

		err := c.Compile(node.Body)

		if err != nil {
			return err
		}

		if c.lastInstructionIs(code.OpPop) {
			c.replaceLastPopWithReturn()
		}

		if !c.lastInstructionIs(code.OpReturnValue) {
			c.emit(code.OpReturn)
		}

		freeSymbols := c.symbolTable.FreeSymbols

		numLocals := c.symbolTable.numDefinitions

		instructions := c.leaveScope()

		for _, s := range freeSymbols {

			c.loadSymbol(s)
		}

		compiledFn := &object.CompiledFunction{
			Instructions:  instructions,
			NumLocals:     numLocals,
			NumParameters: len(node.Parameters),
		}

		fnIndex := c.addConstant(compiledFn)

		c.emit(code.OpClosure, fnIndex, len(freeSymbols))

	case *ast.ReturnStatement:

		err := c.Compile(node.ReturnValue)

		if err != nil {
			return err
		}

		c.emit(code.OpReturnValue)

	case *ast.IfExpression:
		log.Println("if expression...")
		log.Println("condition...")
		err := c.Compile(node.Condition)

		if err != nil {
			return err
		}

		// Emit an `OpJumpNotTruthy` with a bogus value
		jumpNotTruthyPos := c.emit(code.OpJumpNotTruthy, 9999)

		log.Println("consequence...start")
		err = c.Compile(node.Consequence)
		log.Println("consequence...end")

		if err != nil {
			return err
		}

		// Consequenceの最後のインストラクションがOpPopの場合、
		// それを取り除く
		// そうしないとConsequenceの最後の値がポップされ、
		// if文の値として取得できなくなる
		// ちなみに、ExpressionStatementでのみ、最後にOpPopを追加している
		if c.lastInstructionIs(code.OpPop) {
			log.Println("last instruction is pop, it's removed")
			c.removeLastPop()
		}

		// Emit an `OpJump` with a bogus value
		jumpPos := c.emit(code.OpJump, 9999)

		afterConsequencePos := len(c.currentInstructions())

		c.changeOperand(jumpNotTruthyPos, afterConsequencePos)

		if node.Alternative == nil {

			c.emit(code.OpNull)

		} else {

			err := c.Compile(node.Alternative)

			if err != nil {
				return err
			}

			// 最後がOpPopの場合、それを削除する（スタックに残しておく）
			if c.lastInstructionIs(code.OpPop) {
				c.removeLastPop()
			}
		}

		afterAlternativePos := len(c.currentInstructions())

		c.changeOperand(jumpPos, afterAlternativePos)

	case *ast.BlockStatement:
		log.Println("block start...")
		for _, s := range node.Statements {

			err := c.Compile(s)

			if err != nil {
				return err
			}
		}
		log.Println("block end")

	case *ast.ExpressionStatement:
		log.Printf("ExpressionStatement start %s\n", node.String())
		err := c.Compile(node.Expression)
		if err != nil {
			return err
		}
		log.Printf("before OpPop %s\n", node.String())
		// 文の実行が終わったあと、スタックから先頭要素をポップするため
		c.emit(code.OpPop)
		log.Printf("ExpressionStatement end %s\n", node.String())

	case *ast.LetStatement:

		symbol := c.symbolTable.Define(node.Name.Value)

		err := c.Compile(node.Value)

		if err != nil {
			return err
		}

		if symbol.Scope == GlobalScope {
			c.emit(code.OpSetGlobal, symbol.Index)
		} else {
			c.emit(code.OpSetLocal, symbol.Index)
		}

	case *ast.Identifier:

		symbol, ok := c.symbolTable.Resolve(node.Value)

		if !ok {
			return fmt.Errorf("undefined variable %s", node.Value)
		}

		c.loadSymbol(symbol)

	case *ast.CallExpression:

		err := c.Compile(node.Function)

		if err != nil {
			return err
		}

		for _, a := range node.Arguments {

			err := c.Compile(a)

			if err != nil {
				return err
			}
		}

		c.emit(code.OpCall, len(node.Arguments))

	case *ast.PrefixExpression:

		err := c.Compile(node.Right)

		if err != nil {
			return err
		}

		switch node.Operator {

		case "!":
			c.emit(code.OpBang)

		case "-":
			c.emit(code.OpMinus)

		default:
			return fmt.Errorf("unknown operator %s", node.Operator)
		}

	case *ast.InfixExpression:

		if node.Operator == "<" {

			// less than は greater thanを使用するため、
			// 右辺からスタックに積んでいる

			err := c.Compile(node.Right)
			if err != nil {
				return err
			}

			err = c.Compile(node.Left)
			if err != nil {
				return err
			}

			c.emit(code.OpGreaterThan)

			return nil
		}

		err := c.Compile(node.Left)
		if err != nil {
			return err
		}

		err = c.Compile(node.Right)
		if err != nil {
			return err
		}

		// 上記バイトコードにより、先にオペランド２つがスタック上にのる
		// その後に演算子のバイトコードが来る
		switch node.Operator {

		case "+":
			c.emit(code.OpAdd)

		case "-":
			c.emit(code.OpSub)

		case "*":
			c.emit(code.OpMul)

		case "/":
			c.emit(code.OpDiv)

		case ">":
			c.emit(code.OpGreaterThan)

		case "==":
			c.emit(code.OpEqual)

		case "!=":
			c.emit(code.OpNotEqual)

		default:
			return fmt.Errorf("unknown operator %s", node.Operator)
		}

	case *ast.IndexExpression:

		err := c.Compile(node.Left)

		if err != nil {
			return err
		}

		err = c.Compile(node.Index)

		if err != nil {
			return err
		}

		c.emit(code.OpIndex)

	case *ast.IntegerLiteral:

		integer := &object.Integer{Value: node.Value}

		index := c.addConstant(integer)

		c.emit(code.OpConstant, index)

	case *ast.StringLiteral:

		str := &object.String{Value: node.Value}

		index := c.addConstant(str)

		c.emit(code.OpConstant, index)

	case *ast.ArrayLiteral:

		for _, el := range node.Elements {

			err := c.Compile(el)

			if err != nil {
				return err
			}
		}

		c.emit(code.OpArray, len(node.Elements))

	case *ast.HashLiteral:

		keys := []ast.Expression{}

		for k := range node.Pairs {
			keys = append(keys, k)
		}

		// Goのmapを走査するとき、
		// キーの順序は一定ではないので、
		// 明示的にソートする（評価の順序が変わってしまいそうな気はする）
		// テストなどで検証に失敗する（テストでは他にやり方もありそうな気がするが...）
		sort.Slice(keys, func(i, j int) bool {

			return keys[i].String() < keys[j].String()
		})

		for _, k := range keys {

			// キー
			err := c.Compile(k)

			if err != nil {
				return err
			}

			// キーに対応する値
			err = c.Compile(node.Pairs[k])

			if err != nil {
				return err
			}
		}

		// オペランドはキーと値の数
		c.emit(code.OpHash, len(node.Pairs)*2)

	case *ast.Boolean:

		if node.Value {
			c.emit(code.OpTrue)
		} else {
			c.emit(code.OpFalse)
		}
	}

	return nil
}

func (c *Compiler) addConstant(obj object.Object) int {
	// 末尾に追加して、そのインデックスを返す（識別子として使う）
	c.constants = append(c.constants, obj)
	return len(c.constants) - 1
}

// バイトコードインストラクションを生成して追加する
func (c *Compiler) emit(op code.Opcode, operands ...int) int {

	ins := code.Make(op, operands...)

	pos := c.addInstruction(ins)

	c.setLastInstruction(op, pos)

	return pos
}

func (c *Compiler) setLastInstruction(op code.Opcode, pos int) {

	previous := c.scopes[c.scopeIndex].lastInstruction

	last := EmittedInstruction{Opcode: op, Position: pos}

	c.scopes[c.scopeIndex].previousInstruction = previous
	c.scopes[c.scopeIndex].lastInstruction = last
}

func (c *Compiler) addInstruction(ins []byte) int {

	posNewInstruction := len(c.currentInstructions())

	updatedInstructions := append(c.currentInstructions(), ins...)

	c.scopes[c.scopeIndex].instructions = updatedInstructions

	// 追加したインストラクションの、instructionsにおける開始位置を返す
	return posNewInstruction
}

func (c *Compiler) lastInstructionIs(op code.Opcode) bool {

	if len(c.currentInstructions()) == 0 {
		return false
	}

	return c.scopes[c.scopeIndex].lastInstruction.Opcode == op
}

func (c *Compiler) removeLastPop() {

	last := c.scopes[c.scopeIndex].lastInstruction
	previous := c.scopes[c.scopeIndex].previousInstruction

	old := c.currentInstructions()

	new := old[:last.Position]

	c.scopes[c.scopeIndex].instructions = new
	c.scopes[c.scopeIndex].lastInstruction = previous
}

func (c *Compiler) replaceInstruction(pos int, newInstruction []byte) {

	ins := c.currentInstructions()

	// もともとの範囲にあるバイト値を置き換えている
	// 置き換え前後で同じ長さなので問題はないことになる
	for i := 0; i < len(newInstruction); i++ {
		ins[pos+i] = newInstruction[i]
	}
}

func (c *Compiler) changeOperand(opPos int, operand int) {

	op := code.Opcode(c.currentInstructions()[opPos])

	// []byte 新しくインストラクションを作る
	newInstruction := code.Make(op, operand)

	// もともとあったインストラクションを置き換える
	c.replaceInstruction(opPos, newInstruction)
}

type CompilationScope struct {
	instructions        code.Instructions
	lastInstruction     EmittedInstruction
	previousInstruction EmittedInstruction
}

func (c *Compiler) currentInstructions() code.Instructions {
	return c.scopes[c.scopeIndex].instructions
}

func (c *Compiler) enterScope() {

	scope := CompilationScope{
		instructions:        code.Instructions{},
		lastInstruction:     EmittedInstruction{},
		previousInstruction: EmittedInstruction{},
	}

	c.scopes = append(c.scopes, scope)
	c.scopeIndex++

	c.symbolTable = NewEnclosedSymbolTable(c.symbolTable)
}

func (c *Compiler) leaveScope() code.Instructions {

	instructions := c.currentInstructions()

	c.scopes = c.scopes[:len(c.scopes)-1]
	c.scopeIndex--

	c.symbolTable = c.symbolTable.Outer

	return instructions
}

func (c *Compiler) replaceLastPopWithReturn() {

	lastPos := c.scopes[c.scopeIndex].lastInstruction.Position

	opReturn := code.Make(code.OpReturnValue)

	c.replaceInstruction(lastPos, opReturn)

	c.scopes[c.scopeIndex].lastInstruction.Opcode = code.OpReturnValue
}
