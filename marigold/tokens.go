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
	Modulo                  = "Modulo"
	Plus                    = "Plus"
	Minus                   = "Minus"
	Multiply                = "Multiply"
	Exponent                = "Exponent"
	LogicalAnd              = "LogicalAnd"
	LogicalOr               = "LogicalOr"
	Identifier              = "Identifier"
	StringLiteral           = "StringLiteral"
	IntLiteral              = "IntLiteral"
	FloatLiteral            = "FloatLiteral"
	BoolLiteral             = "BoolLiteral"
	String                  = "String"
	Int                     = "Int"
	Float                   = "Float"
	Bool                    = "Bool"
	While                   = "While"
	Return                  = "Return"
	Define                  = "Define"
	Break                   = "Break"
	Continue                = "Continue"
	If                      = "If"
	Else                    = "Else"
	Len                     = "Len"
	Emit                    = "Emit"
	Stop                    = "Stop"
	Map                     = "Map"
	TFunction               = "TFunction"
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
	"%":      Modulo,
	"+":      Plus,
	"-":      Minus,
	"*":      Multiply,
	"^":      Exponent,
	"&&":     LogicalAnd,
	"||":     LogicalOr,
	"string": String,
	"int":    Int,
	"float":  Float,
	"bool":   Bool,
	"while":  While,
	"return": Return,
	"define": Define,
	"break":  Break,
	"continue": Continue,
	"if":     If,
	"else":   Else,
	"len":    Len,
	"emit":   Emit,
	"stop":   Stop,
	"map":    Map,
	"true":   BoolLiteral,
	"false":  BoolLiteral,
}

func (tt *TokenType) IsValidReturnType() bool {
	if tt == nil {
		return false
	}
	return *tt == String || *tt == Int || *tt == Float || *tt == Bool
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
