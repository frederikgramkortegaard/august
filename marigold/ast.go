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
)

type Function struct {
	Name           string      `json:"name"`
	Parameters     []string    `json:"parameters"`
	ParameterTypes []TokenType `json:"parameter_types"`
	ReturnType     TokenType   `json:"return_type"`
	Block          *Block      `json:"block"`
}

type Expression struct {
	Type     ExpressionType `json:"type"`
	Operator TokenType      `json:"operator,omitempty"` // For binary/unary ops
	Left     *Expression    `json:"left,omitempty"`     // For binary ops
	Right    *Expression    `json:"right,omitempty"`    // For binary ops
	Value    interface{}    `json:"value,omitempty"`    // For literals/identifiers
	Args     []*Expression  `json:"args,omitempty"`     // For function calls
}

type Block struct {
	Statements []*Statement `json:"statements"`
}

type Statement struct {
	Type        StatementType `json:"type"`
	VarType     TokenType     `json:"var_type,omitempty"` // For assignments: Int, Float, String (from x: int = 5)
	Lhs         *Expression   `json:"lhs,omitempty"`
	Rhs         *Expression   `json:"rhs,omitempty"`
	Conditional *Expression   `json:"conditional,omitempty"`
	Block       *Block        `json:"block,omitempty"`
	ElseBlock   *Block        `json:"elseblock,omitempty"`
}

type Ast struct {
	Tokens    []*Token    `json:"tokens,omitempty"`
	Functions []*Function `json:"functions"`
	Block     *Block      `json:"block,omitempty"`
}

func NewAst(tokens []*Token) *Ast {
	return &Ast{
		Tokens:    tokens,
		Functions: make([]*Function, 0),
		Block:     nil,
	}
}
