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
	IndexExpr                     = "IndexExpr"
)

type Function struct {
	Name              string       `json:"name"`
	Parameters        []string     `json:"parameters"`
	ParameterTypes    []TokenType  `json:"parameter_types"`
	ParameterArrayTypes []*ArrayType `json:"parameter_array_types,omitempty"`
	ReturnType        TokenType    `json:"return_type"`
	ReturnArrayType   *ArrayType   `json:"return_array_type,omitempty"`
	Block             *Block       `json:"block"`
}

type Expression struct {
	Type         ExpressionType `json:"type"`
	Operator     TokenType      `json:"operator,omitempty"`   // For binary/unary ops
	Lhs          *Expression    `json:"lhs,omitempty"`        // For binary ops, array base for indexing
	Rhs          *Expression    `json:"rhs,omitempty"`        // For binary/unary ops, index for array indexing
	Value        interface{}    `json:"value,omitempty"`      // For literals/identifiers
	ValueType    TokenType      `json:"value_type,omitempty"` // IntLiteral, FloatLiteral, StringLiteral, etc
	ArrayType    *ArrayType     `json:"array_type,omitempty"` // For array expressions
	Args         []*Expression  `json:"args,omitempty"`       // For function calls, array literal elements
	Token        *Token         `json:"token,omitempty"`      // Position information
}

type Block struct {
	Statements []*Statement `json:"statements"`
	Scope      *Scope       `json:"scope"`
}

type Statement struct {
	Type         StatementType `json:"type"`
	VarType      TokenType     `json:"var_type,omitempty"` // For assignments: Int, Float, String (from x: int = 5)
	VarArrayType *ArrayType    `json:"var_array_type,omitempty"` // For array assignments: [5]int
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

type ArrayType struct {
	Size        int       `json:"size"`
	ElementType TokenType `json:"element_type"`
}

type Variable struct {
	Name      string     `json:"name"`
	Value     string     `json:"value"`
	Type      TokenType  `json:"type"`       // IntLiteral, StringLiteral, FloatLiteral
	ArrayType *ArrayType `json:"array_type,omitempty"` // For array types like [5]int
}
