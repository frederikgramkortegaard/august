package marigold

// Statement types
type StatementType string

const (
	AssignmentStmt StatementType = "AssignmentStmt"
	ReturnStmt                   = "ReturnStmt"
	IfStmt                       = "IfStmt"
	WhileStmt                    = "WhileStmt"
	EmitStmt                     = "EmitStmt"
	ExpressionStmt               = "ExpressionStmt"
)

// Expression types
type ExpressionType string

const (
	BinaryExpr     ExpressionType = "BinaryExpr"
	UnaryExpr                     = "UnaryExpr"
	CallExpr                      = "CallExpr"
	IdentifierExpr                = "IdentifierExpr"
	LiteralExpr                   = "LiteralExpr"
	ArrayLiteral                  = "ArrayLiteral"
	MapLiteral                    = "MapLiteral"
	IndexExpr                     = "IndexExpr"
)

type Function struct {
	Name           string   `json:"name"`
	Parameters     []string `json:"parameters"`
	ParameterTypes []Type   `json:"parameter_types"`
	ReturnType     Type     `json:"return_type"`
	Block          *Block   `json:"block"`
}

type Expression struct {
	Type         ExpressionType `json:"type"`
	Operator     TokenType      `json:"operator,omitempty"`   // For binary/unary ops
	Lhs          *Expression    `json:"lhs,omitempty"`        // For binary ops, array base for indexing
	Rhs          *Expression    `json:"rhs,omitempty"`        // For binary/unary ops, index for array indexing
	Value        interface{}    `json:"value,omitempty"`      // For literals/identifiers
	ValueType    TokenType      `json:"value_type,omitempty"` // IntLiteral, FloatLiteral, StringLiteral, etc
	Args         []*Expression  `json:"args,omitempty"`       // For function calls, array literal elements
	Token        *Token         `json:"token,omitempty"`      // Position information
}

type Block struct {
	Statements []*Statement `json:"statements"`
	Scope      *Scope       `json:"scope"`
}

type Statement struct {
	Type         StatementType `json:"type"`
	VarType      Type          `json:"var_type,omitempty"` // For assignments: the complete type
	Lhs          *Expression   `json:"lhs,omitempty"`
	Rhs          *Expression   `json:"rhs,omitempty"`
	Conditional  *Expression   `json:"conditional,omitempty"`
	Block        *Block        `json:"block,omitempty"`
	ElseBlock    *Block        `json:"elseblock,omitempty"`
	Token        *Token        `json:"token,omitempty"`    // Position information
}

type Scope struct {
	Variables map[string]*Variable `json:"variables"`
	Parent    *Scope               `json:"parent"`
}
type Ast struct {
	Tokens    []*Token             `json:"tokens,omitempty"`
	Functions map[string]*Function `json:"functions"`
	Scope     *Scope               `json:"scope,omitempty"`
}

func NewAst(tokens []*Token) *Ast {
	return &Ast{
		Tokens:    tokens,
		Functions: make(map[string]*Function),
		Scope:     nil,
	}
}

type Variable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Type  Type   `json:"type"` // The complete type
}
