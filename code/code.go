package code

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

type Instructions []byte

type Opcode byte

const (
	// 1つ目のopcode OpConstant
	OpConstant Opcode = iota
	// オペランドを持たない
	// スタック上の上から２つの要素を加算する
	OpAdd
	// スタックの先頭を返す
	OpPop
	// 減算
	OpSub
	// 掛け算
	OpMul
	// 割り算
	OpDiv

	// Boolean
	// スタックにBoolean値をプッシュする
	OpTrue
	OpFalse

	// 比較演算子
	OpEqual
	OpNotEqual
	OpGreaterThan
	// LessThanを追加することもできるが、
	// コンパイル時にコードの順番を入れ替えることをデモしたいので追加していない
	// これはインタープリターではできない(?)ことである

	// 前置演算子
	OpMinus
	OpBang

	// Jump
	OpJumpNotTruthy
	OpJump

	// Null
	OpNull

	// オペランドのインデックス値(globals store)の位置にある値を取得し、
	// スタックの先頭にプッシュする
	OpGetGlobal
	// スタックの先頭要素を取得し、オペランドのインデックス値(globals store)の位置にその値を保存する
	OpSetGlobal

	// ローカルbindings
	OpGetLocal
	OpSetLocal

	// 配列
	OpArray
	// Hash
	OpHash

	// Index演算子
	OpIndex

	// 関数呼び出し
	OpCall
	// 明示的なreturn
	OpReturnValue
	// 返すものがないときのreturn
	OpReturn

	OpGetBuiltin

	OpClosure
	OpGetFree

	OpCurrentClosure
)

// Opcodeの定義情報（人間が理解する用）
type Definition struct {
	// opcodeの人が読める名前
	Name string
	// operandそれぞれが占めるバイト数
	Operandwidths []int
}

var definitions = map[Opcode]*Definition{

	// オペランドは2バイト、定数プールのインデックス
	OpConstant: {"OpConstant", []int{2}},

	// OpAddはオペランドが無いので空の配列
	OpAdd:         {"OpAdd", []int{}},
	OpPop:         {"OpPop", []int{}},
	OpSub:         {"OpSub", []int{}},
	OpMul:         {"OpMul", []int{}},
	OpDiv:         {"OpDiv", []int{}},
	OpTrue:        {"OpTrue", []int{}},
	OpFalse:       {"OpFalse", []int{}},
	OpEqual:       {"OpEqual", []int{}},
	OpNotEqual:    {"OpNotEqual", []int{}},
	OpGreaterThan: {"OpGreaterThan", []int{}},
	OpMinus:       {"OpMinus", []int{}},
	OpBang:        {"OpBang", []int{}},

	// Jump  オペランドは2バイト、ジャンプ先のオフセット
	OpJumpNotTruthy: {"OpJumpNotTruthy", []int{2}},
	OpJump:          {"OpJump", []int{2}},

	OpNull: {"OpNull", []int{}},

	OpGetGlobal: {"OpGetGlobal", []int{2}},
	OpSetGlobal: {"OpSetGlobal", []int{2}},

	OpGetLocal: {"OpGetLocal", []int{1}},
	OpSetLocal: {"OpSetLocal", []int{1}},

	OpArray: {"OpArray", []int{2}},
	OpHash:  {"OpHash", []int{2}},

	OpIndex: {"OpIndex", []int{}},

	OpCall:        {"OpCall", []int{1}},
	OpReturnValue: {"OpReturnValue", []int{}},
	OpReturn:      {"OpReturn", []int{}},

	OpGetBuiltin: {"OpGetBuiltin", []int{1}},

	// 1つめは、compiled functionのconstant index
	// 2つめは、スタック上にある、転送する必要があるfree variableの数
	OpClosure: {"OpClosure", []int{2, 1}},
	OpGetFree: {"OpGetFree", []int{1}},

	OpCurrentClosure: {"OpCurrentClosure", []int{}},
}

func Lookup(op byte) (*Definition, error) {
	def, ok := definitions[Opcode(op)]
	if !ok {
		return nil, fmt.Errorf("opcode %d undefined", op)
	}
	return def, nil
}

func Make(op Opcode, operands ...int) []byte {

	def, ok := definitions[op]

	if !ok {
		return []byte{}
	}

	// opcode is 1 byte
	instructionLen := 1

	for _, w := range def.Operandwidths {

		instructionLen += w
	}

	instruction := make([]byte, instructionLen)

	// 1バイト名はopcode
	instruction[0] = byte(op)

	offset := 1

	for i, o := range operands {

		// あるオペランドのバイト数
		width := def.Operandwidths[i]

		switch width {

		case 2:
			// オペランド(の値)を2バイトの幅で
			// インストラクションの指定したオフセットを開始位置として埋め込んでいる
			binary.BigEndian.PutUint16(instruction[offset:],
				uint16(o))

		case 1:
			instruction[offset] = byte(o)
		}

		offset += width
	}

	return instruction
}

func (ins Instructions) String() string {

	var out bytes.Buffer

	i := 0

	for i < len(ins) {

		// opcode(1バイト)を渡してその定義を取得
		def, err := Lookup(ins[i])

		if err != nil {

			fmt.Fprintf(&out, "ERROR: %s\n", err)

			continue
		}

		operands, read := ReadOperands(def, ins[i+1:])

		fmt.Fprintf(&out, "%04d %s\n",
			i,
			ins.fmtInstruction(def, operands))

		i += 1 + read
	}

	return out.String()
}

func (ins Instructions) fmtInstruction(def *Definition, operands []int) string {

	operandCount := len(def.Operandwidths)

	// オペランドの数を検証
	if len(operands) != operandCount {

		return fmt.Sprintf("ERROR: operand len %d does not match defined %d\n",
			len(operands),
			operandCount)
	}

	switch operandCount {

	case 0: // オペランドなし
		return def.Name

	case 1:
		// opcodeの名前とオペランドの値を表示している
		return fmt.Sprintf("%s %d",
			def.Name,
			operands[0])

	case 2:
		return fmt.Sprintf("%s %d %d",
			def.Name,
			operands[0],
			operands[1])
	}

	return fmt.Sprintf("ERROR: unhandled operandCount for %s\n",
		def.Name)
}

func ReadOperands(def *Definition, ins Instructions) ([]int, int) {

	operands := make([]int, len(def.Operandwidths))

	offset := 0

	// オペランドごとのバイト数を取得し、そのバイト数分読み取る
	for i, width := range def.Operandwidths {

		switch width {
		case 2:
			operands[i] = int(ReadUint16(ins[offset:]))

		case 1:
			operands[i] = int(ReadUint8(ins[offset:]))
		}

		offset += width
	}

	return operands, offset
}

func ReadUint16(ins Instructions) uint16 {

	// おそらく2バイト(16ビット)読み取っている
	return binary.BigEndian.Uint16(ins)
}

func ReadUint8(ins Instructions) uint8 {
	return uint8(ins[0])
}
