package marigold

import "fmt"

// Type represents any type in the Marigold language
type Type interface {
	String() string
	Equals(other Type) bool
	IsNumeric() bool
	IsAssignableTo(other Type) bool
}

// SimpleType represents basic types: int, float, string, bool
type SimpleType struct {
	Kind string // "int", "float", "string", "bool"
}

func (t *SimpleType) String() string {
	return t.Kind
}

func (t *SimpleType) Equals(other Type) bool {
	if other == nil {
		return false
	}
	if o, ok := other.(*SimpleType); ok {
		return t.Kind == o.Kind
	}
	return false
}

func (t *SimpleType) IsNumeric() bool {
	return t.Kind == "int" || t.Kind == "float"
}

func (t *SimpleType) IsAssignableTo(other Type) bool {
	if t.Equals(other) {
		return true
	}
	// Allow int <-> float conversions
	if o, ok := other.(*SimpleType); ok {
		if t.IsNumeric() && o.IsNumeric() {
			return true
		}
	}
	return false
}

// ArrayType represents array types: [5]int, []string
type ArrayType struct {
	Size        int  // -1 for dynamic/inferred size
	ElementType Type
}

func (t *ArrayType) String() string {
	sizeStr := ""
	if t.Size >= 0 {
		sizeStr = fmt.Sprintf("%d", t.Size)
	}
	return fmt.Sprintf("[%s]%s", sizeStr, t.ElementType.String())
}

func (t *ArrayType) Equals(other Type) bool {
	if other == nil {
		return false
	}
	if o, ok := other.(*ArrayType); ok {
		return t.Size == o.Size && t.ElementType.Equals(o.ElementType)
	}
	return false
}

func (t *ArrayType) IsNumeric() bool {
	return false
}

func (t *ArrayType) IsAssignableTo(other Type) bool {
	return t.Equals(other)
}

// MapType represents map types: map[string]int
type MapType struct {
	KeyType   Type
	ValueType Type
}

func (t *MapType) String() string {
	return fmt.Sprintf("map[%s]%s", t.KeyType.String(), t.ValueType.String())
}

func (t *MapType) Equals(other Type) bool {
	if other == nil {
		return false
	}
	if o, ok := other.(*MapType); ok {
		return t.KeyType.Equals(o.KeyType) && t.ValueType.Equals(o.ValueType)
	}
	return false
}

func (t *MapType) IsNumeric() bool {
	return false
}

func (t *MapType) IsAssignableTo(other Type) bool {
	return t.Equals(other)
}

// Helper functions for common types
var (
	IntType    = &SimpleType{Kind: "int"}
	FloatType  = &SimpleType{Kind: "float"}
	StringType = &SimpleType{Kind: "string"}
	BoolType   = &SimpleType{Kind: "bool"}
)

// NewArrayType creates an array type
func NewArrayType(size int, elementType Type) *ArrayType {
	return &ArrayType{
		Size:        size,
		ElementType: elementType,
	}
}

// NewMapType creates a map type
func NewMapType(keyType, valueType Type) *MapType {
	return &MapType{
		KeyType:   keyType,
		ValueType: valueType,
	}
}

// TypeFromTokenType converts old TokenType to new Type system
func TypeFromTokenType(tt TokenType) Type {
	switch tt {
	case Int:
		return IntType
	case Float:
		return FloatType
	case String:
		return StringType
	case Bool:
		return BoolType
	default:
		return nil
	}
}