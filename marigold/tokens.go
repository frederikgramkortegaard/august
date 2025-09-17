package marigold

// TokenType represents the type of a token
type TokenType string

// Token type constants
const (
	LParen        TokenType = "LParen"
	RParen                  = "RParen"
	LSquare                 = "LSquare"
	RSquare                 = "RSquare"
	LBrace                  = "LBrace"
	RBrace                  = "RBrace"
	Comma                   = "Comma"
	Dot                     = "Dot"
	Pipe                    = "Pipe"
	Colon                   = "Colon"
	Semicolon               = "Semicolon"
	Assign                  = "Assign"
	Equal                   = "Equal"
	NotEqual                = "NotEqual"
	LessThan                = "LessThan"
	LessEqual               = "LessEqual"
	GreaterThan             = "GreaterThan"
	GreaterEqual            = "GreaterEqual"
	Divide                  = "Divide"
	Plus                    = "Plus"
	Minus                   = "Minus"
	Multiply                = "Multiply"
	Identifier              = "Identifier"
	StringLiteral           = "StringLiteral"
	IntLiteral              = "IntLiteral"
	FloatLiteral            = "FloatLiteral"
	String                  = "String"
	Int                     = "Int"
	Float                   = "Float"
	While                   = "While"
	Return                  = "Return"
	Define                  = "Define"
	Emit                    = "Emit"
	If                      = "If"
	Else                    = "Else"
	Eof                     = "Eof"
)

// Map from token values to their types
var TokenTypeMap = map[string]TokenType{
	"(":      LParen,
	")":      RParen,
	"[":      LSquare,
	"]":      RSquare,
	"{":      LBrace,
	"}":      RBrace,
	",":      Comma,
	".":      Dot,
	"|":      Pipe,
	":":      Colon,
	";":      Semicolon,
	"=":      Assign,
	"==":     Equal,
	"!=":     NotEqual,
	"<":      LessThan,
	"<=":     LessEqual,
	">":      GreaterThan,
	">=":     GreaterEqual,
	"/":      Divide,
	"+":      Plus,
	"-":      Minus,
	"*":      Multiply,
	"string": String,
	"int":    Int,
	"float":  Float,
	"while":  While,
	"return": Return,
	"define": Define,
	"emit":   Emit,
	"if":     If,
	"else":   Else,
}

func (tt *TokenType) IsValidReturnType() bool {
	if tt == nil {
		return false
	}
	return *tt == String || *tt == Int || *tt == Float
}

type Token struct {
	Type     TokenType `json:"type"`
	Value    string    `json:"value"`
	Col      uint64    `json:"col"`
	Row      uint64    `json:"row"`
	Filename string    `json:"filename"`
}

func NewToken(value string, tokenType TokenType, col, row uint64, filename string) *Token {
	return &Token{
		Type:     tokenType,
		Value:    value,
		Col:      col,
		Row:      row,
		Filename: filename,
	}
}
