package tests

import (
	"august/marigold"
	"august/types"
	"testing"
)

func TestCodegenSimpleLiteral(t *testing.T) {
	ast := &marigold.Ast{
		Functions: make(map[string]*marigold.Function),
		Scope: &marigold.Scope{
			Variables: make(map[string]*marigold.Variable),
			Parent:    nil,
		},
	}

	intExpr := &marigold.Expression{
		Type:      marigold.LiteralExpr,
		Value:     42,
		ValueType: marigold.IntLiteral,
	}

	ctx := &marigold.CodegenContext{
		Ast:              ast,
		Instructions:     make([]types.Instruction, 0),
		FunctionIndex:    make(map[string]int),
		LocalOffsets:     make(map[string]int),
		StringTable:      make(map[string]uint64),
		StringLiterals:   make([]string, 0),
		LoopStack:        make([]marigold.LoopContext, 0),
	}

	marigold.GenerateExpression(ctx, intExpr)

	if len(ctx.Instructions) > 0 {
		t.Logf("Generated %d instructions", len(ctx.Instructions))
		for _, inst := range ctx.Instructions {
			if inst.Value != nil {
				t.Logf("  %s %s", inst.Opcode.String(), *inst.Value)
			} else {
				t.Logf("  %s", inst.Opcode.String())
			}
		}
	} else {
		t.Log("No instructions generated yet")
	}
}

func TestCodegenBinaryExpression(t *testing.T) {
	ast := &marigold.Ast{
		Functions: make(map[string]*marigold.Function),
		Scope: &marigold.Scope{
			Variables: make(map[string]*marigold.Variable),
			Parent:    nil,
		},
	}

	leftExpr := &marigold.Expression{
		Type:      marigold.LiteralExpr,
		Value:     10,
		ValueType: marigold.IntLiteral,
	}

	rightExpr := &marigold.Expression{
		Type:      marigold.LiteralExpr,
		Value:     5,
		ValueType: marigold.IntLiteral,
	}

	binaryExpr := &marigold.Expression{
		Type:     marigold.BinaryExpr,
		Operator: marigold.Plus,
		Lhs:      leftExpr,
		Rhs:      rightExpr,
	}

	ctx := &marigold.CodegenContext{
		Ast:              ast,
		Instructions:     make([]types.Instruction, 0),
		FunctionIndex:    make(map[string]int),
		LocalOffsets:     make(map[string]int),
		StringTable:      make(map[string]uint64),
		StringLiterals:   make([]string, 0),
		LoopStack:        make([]marigold.LoopContext, 0),
	}

	marigold.GenerateExpression(ctx, binaryExpr)

	if len(ctx.Instructions) > 0 {
		t.Logf("Generated %d instructions for 10 + 5:", len(ctx.Instructions))
		for _, inst := range ctx.Instructions {
			if inst.Value != nil {
				t.Logf("  %s %s", inst.Opcode.String(), *inst.Value)
			} else {
				t.Logf("  %s", inst.Opcode.String())
			}
		}
	} else {
		t.Log("No instructions generated")
	}
}

func TestCodegenIdentifier(t *testing.T) {
	ast := &marigold.Ast{
		Functions: make(map[string]*marigold.Function),
		Scope: &marigold.Scope{
			Variables: map[string]*marigold.Variable{
				"x": {
					Name: "x",
					Type: marigold.IntType,
				},
			},
			Parent: nil,
		},
	}

	identExpr := &marigold.Expression{
		Type:  marigold.IdentifierExpr,
		Value: "x",
	}

	ctx := &marigold.CodegenContext{
		Ast:          ast,
		CurrentScope: ast.Scope,
	}

	marigold.GenerateExpression(ctx, identExpr)

	t.Logf("Generated %d instructions", len(ctx.Instructions))
}

func TestCodegenFunctionCall(t *testing.T) {
	ast := &marigold.Ast{
		Functions: map[string]*marigold.Function{
			"add": {
				Name:       "add",
				Parameters: []string{"a", "b"},
				ParameterTypes: []marigold.Type{
					marigold.IntType,
					marigold.IntType,
				},
				ReturnType: marigold.IntType,
			},
		},
		Scope: &marigold.Scope{
			Variables: make(map[string]*marigold.Variable),
			Parent:    nil,
		},
	}

	arg1 := &marigold.Expression{
		Type:      marigold.LiteralExpr,
		Value:     5,
		ValueType: marigold.IntLiteral,
	}

	arg2 := &marigold.Expression{
		Type:      marigold.LiteralExpr,
		Value:     3,
		ValueType: marigold.IntLiteral,
	}

	callExpr := &marigold.Expression{
		Type:  marigold.CallExpr,
		Value: "add",
		Args:  []*marigold.Expression{arg1, arg2},
	}

	ctx := &marigold.CodegenContext{
		Ast:          ast,
		CurrentScope: ast.Scope,
	}

	marigold.GenerateExpression(ctx, callExpr)

	t.Logf("Generated %d instructions", len(ctx.Instructions))
}

func TestCodegenArrayLiteral(t *testing.T) {
	ast := &marigold.Ast{
		Functions: make(map[string]*marigold.Function),
		Scope: &marigold.Scope{
			Variables: make(map[string]*marigold.Variable),
			Parent:    nil,
		},
	}

	elem1 := &marigold.Expression{
		Type:      marigold.LiteralExpr,
		Value:     1,
		ValueType: marigold.IntLiteral,
	}

	elem2 := &marigold.Expression{
		Type:      marigold.LiteralExpr,
		Value:     2,
		ValueType: marigold.IntLiteral,
	}

	elem3 := &marigold.Expression{
		Type:      marigold.LiteralExpr,
		Value:     3,
		ValueType: marigold.IntLiteral,
	}

	arrayExpr := &marigold.Expression{
		Type: marigold.ArrayLiteral,
		Args: []*marigold.Expression{elem1, elem2, elem3},
	}

	ctx := &marigold.CodegenContext{
		Ast:    ast,
	}

	marigold.GenerateExpression(ctx, arrayExpr)

	t.Logf("Generated code for array [1, 2, 3]: %d instructions", len(ctx.Instructions))
}

func TestCodegenUnaryExpression(t *testing.T) {
	ast := &marigold.Ast{
		Functions: make(map[string]*marigold.Function),
		Scope: &marigold.Scope{
			Variables: make(map[string]*marigold.Variable),
			Parent:    nil,
		},
	}

	innerExpr := &marigold.Expression{
		Type:      marigold.LiteralExpr,
		Value:     42,
		ValueType: marigold.IntLiteral,
	}

	unaryExpr := &marigold.Expression{
		Type:     marigold.UnaryExpr,
		Operator: marigold.Minus,
		Rhs:      innerExpr,
	}

	ctx := &marigold.CodegenContext{
		Ast:    ast,
	}

	marigold.GenerateExpression(ctx, unaryExpr)

	t.Logf("Generated code for -42: %d instructions", len(ctx.Instructions))
}

func TestCodegenIndexExpression(t *testing.T) {
	ast := &marigold.Ast{
		Functions: make(map[string]*marigold.Function),
		Scope: &marigold.Scope{
			Variables: map[string]*marigold.Variable{
				"arr": {
					Name: "arr",
					Type: marigold.NewArrayType(-1, marigold.IntType),
				},
			},
			Parent: nil,
		},
	}

	arrayIdent := &marigold.Expression{
		Type:  marigold.IdentifierExpr,
		Value: "arr",
	}

	indexExpr := &marigold.Expression{
		Type:      marigold.LiteralExpr,
		Value:     0,
		ValueType: marigold.IntLiteral,
	}

	indexingExpr := &marigold.Expression{
		Type: marigold.IndexExpr,
		Lhs:  arrayIdent,
		Rhs:  indexExpr,
	}

	ctx := &marigold.CodegenContext{
		Ast:          ast,
		CurrentScope: ast.Scope,
	}

	marigold.GenerateExpression(ctx, indexingExpr)

	t.Logf("Generated code for arr[0]: %d instructions", len(ctx.Instructions))
}

func TestCodegenMapLiteral(t *testing.T) {
	ast := &marigold.Ast{
		Functions: make(map[string]*marigold.Function),
		Scope: &marigold.Scope{
			Variables: make(map[string]*marigold.Variable),
			Parent:    nil,
		},
	}

	key1 := &marigold.Expression{
		Type:      marigold.LiteralExpr,
		Value:     "name",
		ValueType: marigold.StringLiteral,
	}

	value1 := &marigold.Expression{
		Type:      marigold.LiteralExpr,
		Value:     "Alice",
		ValueType: marigold.StringLiteral,
	}

	mapExpr := &marigold.Expression{
		Type: marigold.MapLiteral,
		Args: []*marigold.Expression{key1, value1},
	}

	ctx := &marigold.CodegenContext{
		Ast:    ast,
	}

	marigold.GenerateExpression(ctx, mapExpr)

	t.Logf("Generated code for map literal: %d instructions", len(ctx.Instructions))
}