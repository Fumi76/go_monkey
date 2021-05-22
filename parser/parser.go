package parser

import (
	"fmt"
	"strconv"

	"example.com/monkey/ast"
	"example.com/monkey/lexer"
	"example.com/monkey/token"
)

// 定義している順番が重要、下に行くほど優先度が高くなる
const (
	_int = iota
	LOWEST
	EQUALS      // ==
	LESSGREATER // > or <
	SUM         // +
	PRODUCT     // *
	PREFIX      // -X or !X
	CALL        // myFunction(X)
	INDEX       // array[index]
)

// トークンとその優先順位の対応付け
var precedences = map[token.TokenType]int{
	token.EQ:       EQUALS,
	token.NOT_EQ:   EQUALS,
	token.LT:       LESSGREATER,
	token.GT:       LESSGREATER,
	token.PLUS:     SUM,
	token.MINUS:    SUM,
	token.SLASH:    PRODUCT,
	token.ASTERISK: PRODUCT,
	token.LPAREN:   CALL,
	token.LBRACKET: INDEX,
}

// 次の位置のトークンの優先度を取得する
func (p *Parser) peekPrecedence() int {
	if p, ok := precedences[p.peekToken.Type]; ok {
		return p
	}
	return LOWEST
}

// 現在のトークンの優先順位を取得する
func (p *Parser) curPrecedence() int {
	if p, ok := precedences[p.curToken.Type]; ok {
		return p
	}
	return LOWEST
}

type Parser struct {
	l         *lexer.Lexer
	curToken  token.Token
	peekToken token.Token
	errors    []string
	// トークンの種類と前置演算子用の解析関数との対応付け
	prefixParseFns map[token.TokenType]prefixParseFn
	// トークンの種類と中置演算子用の解析関数との対応付け
	infixParseFns map[token.TokenType]infixParseFn
}

type (
	// 関数の種類を２つ定義している

	// 前に付く形の解析関数（前置演算子）
	prefixParseFn func() ast.Expression
	// 間に挟まる形（前後と関係する形）の解析関数
	// 関数の引数(ast.Expression)は、中置演算子の左辺を表している
	infixParseFn func(ast.Expression) ast.Expression
)

func (p *Parser) registerPrefix(tokenType token.TokenType, fn prefixParseFn) {
	p.prefixParseFns[tokenType] = fn
}

func (p *Parser) registerInfix(tokenType token.TokenType, fn infixParseFn) {
	p.infixParseFns[tokenType] = fn
}

func New(l *lexer.Lexer) *Parser {
	p := &Parser{l: l, errors: []string{}}

	p.prefixParseFns = make(map[token.TokenType]prefixParseFn)
	// このトークンの種類に出会ったらこの関数を呼び出す
	// 識別子
	p.registerPrefix(token.IDENT, p.parseIdentifier)
	// 整数リテラル
	p.registerPrefix(token.INT, p.parseIntegerLiteral)
	// Boolean
	p.registerPrefix(token.TRUE, p.parseBoolean)
	p.registerPrefix(token.FALSE, p.parseBoolean)
	// 文字列
	p.registerPrefix(token.STRING, p.parseStringLiteral)

	// prefix operators
	p.registerPrefix(token.BANG, p.parsePrefixExpression)
	p.registerPrefix(token.MINUS, p.parsePrefixExpression)

	p.registerPrefix(token.LPAREN, p.parseGroupedExpression)

	p.registerPrefix(token.IF, p.parseIfExpression)

	p.registerPrefix(token.FUNCTION, p.parseFunctionLiteral)

	p.registerPrefix(token.LBRACKET, p.parseArrayLiteral)

	p.registerPrefix(token.LBRACE, p.parseHashLiteral)

	// infix operators
	p.infixParseFns = make(map[token.TokenType]infixParseFn)
	p.registerInfix(token.PLUS, p.parseInfixExpression)
	p.registerInfix(token.MINUS, p.parseInfixExpression)
	p.registerInfix(token.SLASH, p.parseInfixExpression)
	p.registerInfix(token.ASTERISK, p.parseInfixExpression)
	p.registerInfix(token.EQ, p.parseInfixExpression)
	p.registerInfix(token.NOT_EQ, p.parseInfixExpression)
	p.registerInfix(token.LT, p.parseInfixExpression)
	p.registerInfix(token.GT, p.parseInfixExpression)

	p.registerInfix(token.LPAREN, p.parseCallExpression)

	p.registerInfix(token.LBRACKET, p.parseIndexExpression)

	// Read two tokens, so curToken and peekToken are both set
	p.nextToken()
	p.nextToken()
	return p
}

func (p *Parser) parseHashLiteral() ast.Expression {

	hash := &ast.HashLiteral{Token: p.curToken}

	hash.Pairs = make(map[ast.Expression]ast.Expression)

	for !p.peekTokenIs(token.RBRACE) {

		p.nextToken()

		key := p.parseExpression(LOWEST)

		if !p.expectPeek(token.COLON) {
			return nil
		}

		p.nextToken()

		value := p.parseExpression(LOWEST)

		hash.Pairs[key] = value

		if !p.peekTokenIs(token.RBRACE) && !p.expectPeek(token.COMMA) {
			return nil
		}
	}

	if !p.expectPeek(token.RBRACE) {
		return nil
	}

	return hash
}

func (p *Parser) parseIndexExpression(left ast.Expression) ast.Expression {

	exp := &ast.IndexExpression{Token: p.curToken, Left: left}

	p.nextToken()

	exp.Index = p.parseExpression(LOWEST)

	if !p.expectPeek(token.RBRACKET) {
		return nil
	}

	return exp
}

func (p *Parser) parseArrayLiteral() ast.Expression {

	array := &ast.ArrayLiteral{Token: p.curToken}

	array.Elements = p.parseExpressionList(token.RBRACKET)

	return array
}

func (p *Parser) parseExpressionList(end token.TokenType) []ast.Expression {

	list := []ast.Expression{}

	if p.peekTokenIs(end) {
		p.nextToken()
		return list
	}

	p.nextToken()

	list = append(list, p.parseExpression(LOWEST))

	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		p.nextToken()
		list = append(list, p.parseExpression(LOWEST))
	}

	if !p.expectPeek(end) {
		return nil
	}

	return list
}

func (p *Parser) parseStringLiteral() ast.Expression {
	return &ast.StringLiteral{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseCallExpression(function ast.Expression) ast.Expression {

	exp := &ast.CallExpression{Token: p.curToken, Function: function}

	exp.Arguments = p.parseExpressionList(token.RPAREN)

	return exp
}

/* 廃止
func (p *Parser) parseCallArguments() []ast.Expression {

	args := []ast.Expression{}

	if p.peekTokenIs(token.RPAREN) {
		p.nextToken()
		return args
	}

	p.nextToken()

	args = append(args, p.parseExpression(LOWEST))

	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		p.nextToken()
		args = append(args, p.parseExpression(LOWEST))
	}

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	return args
}
*/

func (p *Parser) parseFunctionLiteral() ast.Expression {

	lit := &ast.FunctionLiteral{Token: p.curToken}

	if !p.expectPeek(token.LPAREN) {
		return nil
	}

	lit.Parameters = p.parseFunctionParameters()

	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	lit.Body = p.parseBlockStatement()

	return lit
}

func (p *Parser) parseFunctionParameters() []*ast.Identifier {

	identifiers := []*ast.Identifier{}

	if p.peekTokenIs(token.RPAREN) {
		p.nextToken()
		return identifiers
	}

	p.nextToken()

	ident := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	identifiers = append(identifiers, ident)

	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		p.nextToken()
		ident := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		identifiers = append(identifiers, ident)
	}

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	return identifiers
}

func (p *Parser) parseIfExpression() ast.Expression {
	expression := &ast.IfExpression{Token: p.curToken}
	if !p.expectPeek(token.LPAREN) {
		return nil
	}
	p.nextToken()
	expression.Condition = p.parseExpression(LOWEST)
	if !p.expectPeek(token.RPAREN) {
		return nil
	}
	if !p.expectPeek(token.LBRACE) {
		return nil
	}
	expression.Consequence = p.parseBlockStatement()

	if p.peekTokenIs(token.ELSE) {
		p.nextToken()
		if !p.expectPeek(token.LBRACE) {
			return nil
		}
		expression.Alternative = p.parseBlockStatement()
	}
	return expression
}

func (p *Parser) parseBlockStatement() *ast.BlockStatement {
	block := &ast.BlockStatement{Token: p.curToken}
	block.Statements = []ast.Statement{}

	p.nextToken()

	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		p.nextToken()
	}
	return block
}

func (p *Parser) parseGroupedExpression() ast.Expression {

	p.nextToken()

	exp := p.parseExpression(LOWEST)

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	return exp
}

func (p *Parser) parseBoolean() ast.Expression {
	return &ast.Boolean{Token: p.curToken, Value: p.curTokenIs(token.TRUE)}
}

// infix operators
func (p *Parser) parseInfixExpression(left ast.Expression) ast.Expression {

	//defer untrace(trace("parseInfixExpression"))

	expression := &ast.InfixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
		Left:     left,
	}

	// 現在位置の演算子の優先順位を取得
	precedence := p.curPrecedence()

	p.nextToken()

	// 現在の演算子の右側のパース
	expression.Right = p.parseExpression(precedence)

	return expression
}

// prefix operators (prefix expressions)
func (p *Parser) parsePrefixExpression() ast.Expression {

	//defer untrace(trace("parsePrefixExpression"))

	expression := &ast.PrefixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
	}

	p.nextToken()

	// 右側パース prefix operatorの優先順位を渡す
	expression.Right = p.parseExpression(PREFIX)

	return expression
}

// トークンの位置を進めないのが重要
func (p *Parser) parseIdentifier() ast.Expression {
	return &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) Errors() []string {
	return p.errors
}

func (p *Parser) peekError(t token.TokenType) {
	msg := fmt.Sprintf("expected next token to be %s, got %s instead", t, p.peekToken.Type)
	p.errors = append(p.errors, msg)
}

// 現在位置を１つ進める（次のトークンに現在位置を進める）
func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

func (p *Parser) ParseProgram() *ast.Program {

	// ASTのルートとなるノードを作っている
	program := &ast.Program{}
	program.Statements = []ast.Statement{}

	for p.curToken.Type != token.EOF {
		// 「文」を１つ切り出す（「文」が１つあるか解析する）
		stmt := p.parseStatement()
		if stmt != nil {
			program.Statements = append(program.Statements, stmt)
		}
		// トークン順列上の現在位置を進める
		p.nextToken()
	}
	return program
}

func (p *Parser) parseStatement() ast.Statement {

	// 現在位置のトークンの種類により、解析処理を分岐する
	switch p.curToken.Type {
	// 現在位置のトークンがletの場合、LET文の取り出し（LET文であるかの検証）を開始する
	case token.LET:
		return p.parseLetStatement()
	case token.RETURN:
		return p.parseReturnStatement()
	default:
		return p.parseExpressionStatement()
	}
}

func (p *Parser) parseExpressionStatement() *ast.ExpressionStatement {

	//defer untrace(trace("parseExpressionStatement"))

	stmt := &ast.ExpressionStatement{Token: p.curToken}
	stmt.Expression = p.parseExpression(LOWEST)
	// セミコロンは必須ではない
	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}
	return stmt
}

/* 改良版が下にある
func (p *Parser) parseExpression(precedence int) ast.Expression {

	prefix := p.prefixParseFns[p.curToken.Type]

	if prefix == nil {
		return nil
	}

	leftExp := prefix()

	return leftExp
}
*/

func (p *Parser) parseReturnStatement() *ast.ReturnStatement {
	// 現在位置のトークンは"return"
	stmt := &ast.ReturnStatement{Token: p.curToken}
	// 次のトークンの位置に現在位置を進める
	p.nextToken()

	stmt.ReturnValue = p.parseExpression(LOWEST)

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseLetStatement() *ast.LetStatement {

	stmt := &ast.LetStatement{Token: p.curToken}
	// 次のトークンが識別子ではない場合、解析を終了する
	// 次のトークンが識別子の場合、expectPeek内で現在位置が１つ進められ、trueが返る
	if !p.expectPeek(token.IDENT) {
		return nil
	}
	// 識別子（値がセットされる変数）
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	// 次のトークンが=の場合、expectPeek内で現在位置が１つ進められ、trueが返る
	if !p.expectPeek(token.ASSIGN) {
		return nil
	}

	p.nextToken()

	stmt.Value = p.parseExpression(LOWEST)

	if fl, ok := stmt.Value.(*ast.FunctionLiteral); ok {

		fl.Name = stmt.Name.Value
	}

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

// 現在位置のトークンの種類を確認する
func (p *Parser) curTokenIs(t token.TokenType) bool {
	return p.curToken.Type == t
}

// 次の位置のトークンの種類を確認する
func (p *Parser) peekTokenIs(t token.TokenType) bool {
	return p.peekToken.Type == t
}

// 次の位置のトークンが指定した種類であれば、現在位置を１つ進める（かつ、trueを返す）
// 指定した種類のトークンでない場合、現在位置はそのままでfalseを返す
func (p *Parser) expectPeek(tokenType token.TokenType) bool {
	if p.peekTokenIs(tokenType) {
		p.nextToken()
		return true
	} else {
		p.peekError(tokenType)
		return false
	}
}

func (p *Parser) parseIntegerLiteral() ast.Expression {

	//defer untrace(trace("parseIntegerLiteral"))

	lit := &ast.IntegerLiteral{Token: p.curToken}

	value, err := strconv.ParseInt(p.curToken.Literal, 0, 64)

	if err != nil {
		msg := fmt.Sprintf("could not parse %q as integer", p.curToken.Literal)
		p.errors = append(p.errors, msg)
		return nil
	}

	lit.Value = value

	return lit
}

func (p *Parser) noPrefixParseFnError(t token.TokenType) {
	msg := fmt.Sprintf("no prefix parse function for %s found", t)
	p.errors = append(p.errors, msg)
}

func (p *Parser) parseExpression(precedence int) ast.Expression {

	//defer untrace(trace("parseExpression"))
	//trace2(fmt.Sprintf("curToken=%+v", p.curToken))

	prefix := p.prefixParseFns[p.curToken.Type]

	if prefix == nil {
		p.noPrefixParseFnError(p.curToken.Type)
		return nil
	}

	leftExp := prefix()

	//trace2(fmt.Sprintf("leftExp=%s", leftExp.String()))
	//trace2(fmt.Sprintf("precedence=%d, peekPrecedence=%d, peekToken=%+v", precedence, p.peekPrecedence(), p.peekToken))

	// ここがキモ
	// 引数の優先度が次の位置のトークンの優先度より低ければ、以下の処理を行う
	for !p.peekTokenIs(token.SEMICOLON) && precedence < p.peekPrecedence() {

		// 次の位置のトークンに対応するInfixParseFnを取得する
		infix := p.infixParseFns[p.peekToken.Type]

		if infix == nil {
			return leftExp
		}
		// トークンの位置を１つ進める
		p.nextToken()

		leftExp = infix(leftExp)
		//trace2(fmt.Sprintf("infix戻り値(leftExp)=%s", leftExp.String()))
		//trace2(fmt.Sprintf("precedence=%d, peekPrecedence=%d, peekToken=%+v", precedence, p.peekPrecedence(), p.peekToken))
	}

	return leftExp
}
