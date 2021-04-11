package token

type TokenType string

type Token struct {
	// トークンの種類
	Type TokenType
	// トークンの文字列表現
	Literal string
}

const (
	// 想定外のトークンのとき
	ILLEGAL = "ILLEGAL"

	EOF = "EOF"

	// 識別子(変数の名前、関数の名前）、定数（リテラル）
	IDENT  = "IDENT" //add, foobar, x, y, ...
	INT    = "INT"   // 1343456
	STRING = "STRING"

	// 配列のインデックスアクセス
	LBRACKET = "["
	RBRACKET = "]"

	COLON = ":"

	// 演算子（オペレーター）
	ASSIGN   = "="
	PLUS     = "+"
	MINUS    = "-"
	BANG     = "!"
	ASTERISK = "*"
	SLASH    = "/"
	LT       = "<"
	GT       = ">"
	EQ       = "=="
	NOT_EQ   = "!="

	// 区切り文字（デリミタ）
	COMMA     = ","
	SEMICOLON = ";"
	LPAREN    = "("
	RPAREN    = ")"
	LBRACE    = "{"
	RBRACE    = "}"

	// キーワード（プログラム言語の予約語）
	FUNCTION = "FUNCTION"
	LET      = "LET"
	TRUE     = "TRUE"
	FALSE    = "FALSE"
	IF       = "IF"
	ELSE     = "ELSE"
	RETURN   = "RETURN"
)

// キーワード(予約語)とトークンの種類の対応付け
var keywords = map[string]TokenType{
	"fn":     FUNCTION,
	"let":    LET,
	"true":   TRUE,
	"false":  FALSE,
	"if":     IF,
	"else":   ELSE,
	"return": RETURN,
}

// 識別子(連続する文字)が言語のキーワード(予約語)なのか、
// ユーザー定義の識別子なのかを判別して
// それに応じたTokenTypeを返す
func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}
