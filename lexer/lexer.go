package lexer

import "example.com/monkey/token"

type Lexer struct {
	input string
	// 入力における現在の位置
	// current position in input (points to current char)
	position int
	// 次に読み取る位置
	// current reading position in input (after current char)
	readPosition int
	// 現在の位置の文字
	// current char under examination
	ch byte
}

func New(input string) *Lexer {
	l := &Lexer{input: input}
	l.readChar()
	return l
}

func newToken(tokenType token.TokenType, ch byte) token.Token {
	return token.Token{Type: tokenType, Literal: string(ch)}
}

// 次に読み取る位置から一文字読み取り、chにセットする
// 現在位置もその読み取った位置にずらす
func (l *Lexer) readChar() {
	if l.readPosition >= len(l.input) {
		l.ch = 0
	} else {
		l.ch = l.input[l.readPosition]
	}
	l.position = l.readPosition
	l.readPosition += 1
}

// 次に予定している読み取り位置から読み取るが、
// 現在位置はずらさない
func (l *Lexer) peekChar() byte {
	if l.readPosition >= len(l.input) {
		return 0 // つまり、EOF
	} else {
		return l.input[l.readPosition]
	}
}

func (l *Lexer) NextToken() token.Token {
	var tok token.Token

	// Monkeyでは空白は単語の区切り文字としての意味しかもたない
	// つまり、次に意味のある文字が来るまでスキップする
	l.skipWhitespace()

	switch l.ch {
	case '=':
		// すぐ後ろの文字が=の場合、==(EQ)というトークンにする
		// TODO makeTwoCharTokenという関数を作ってもよいかも（複数文字からなるトークンを切り出す用）
		if l.peekChar() == '=' {
			ch := l.ch
			// １文字進める
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.EQ, Literal: literal}
		} else {
			tok = newToken(token.ASSIGN, l.ch)
		}
	case '+':
		tok = newToken(token.PLUS, l.ch)
	case '-':
		tok = newToken(token.MINUS, l.ch)
	case '!':
		// すぐ後ろの文字が=の場合、!=(NOT_EQ)というトークンにする
		if l.peekChar() == '=' {
			ch := l.ch
			// １文字進める
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.NOT_EQ, Literal: literal}
		} else {
			tok = newToken(token.BANG, l.ch)
		}
	case '/':
		tok = newToken(token.SLASH, l.ch)
	case '*':
		tok = newToken(token.ASTERISK, l.ch)
	case '<':
		tok = newToken(token.LT, l.ch)
	case '>':
		tok = newToken(token.GT, l.ch)
	case ';':
		tok = newToken(token.SEMICOLON, l.ch)
	case '(':
		tok = newToken(token.LPAREN, l.ch)
	case ')':
		tok = newToken(token.RPAREN, l.ch)
	case ',':
		tok = newToken(token.COMMA, l.ch)
	case '{':
		tok = newToken(token.LBRACE, l.ch)
	case '}':
		tok = newToken(token.RBRACE, l.ch)
	case '[':
		tok = newToken(token.LBRACKET, l.ch)
	case ']':
		tok = newToken(token.RBRACKET, l.ch)
	case ':':
		tok = newToken(token.COLON, l.ch)
	case '"':
		tok.Type = token.STRING
		tok.Literal = l.readString()
	case 0:
		tok.Literal = ""
		tok.Type = token.EOF
	default:
		if isLetter(l.ch) {
			tok.Literal = l.readIdentifier()
			// 予約語なのかユーザー定義の識別子なのか
			tok.Type = token.LookupIdent(tok.Literal)
			return tok

		} else if isDigit(l.ch) { // 数字の場合
			tok.Type = token.INT
			tok.Literal = l.readNumber()
			return tok
		} else {
			tok = newToken(token.ILLEGAL, l.ch)
		}
	}

	l.readChar()

	return tok
}

func (l *Lexer) readString() string {

	position := l.position + 1

	// TODO 文字列が閉じられることなくEOFに達したらエラーにする
	// TODO "をエスケープできるようにする
	for {
		l.readChar()

		if l.ch == '"' || l.ch == 0 {
			break
		}
	}

	return l.input[position:l.position]
}

// 連続する文字を返す（文字出ない位置に遭遇するまで）
func (l *Lexer) readIdentifier() string {
	position := l.position
	for isLetter(l.ch) {
		l.readChar()
	}
	return l.input[position:l.position]
}

// 「文字」と判断する文字群を定義している
// 識別子(変数の名前、関数の名前)に使える文字を定義している
func isLetter(ch byte) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_'
}

// スペース、タブ、LF、CRは飛ばす
func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
		l.readChar()
	}
}

func (l *Lexer) readNumber() string {
	position := l.position
	for isDigit(l.ch) {
		l.readChar()
	}
	return l.input[position:l.position]
}

// 0～9は「数字」
// TODO 浮動小数点数、１６進数表記、８進数表記、精度を気にする場合
func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}
