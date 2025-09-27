package marigold

import "errors"

var (
	ErrInvalidSourceCode         = errors.New("invalid source code")
	ErrUnterminatedStringLiteral = errors.New("unterminated string literal")
)
