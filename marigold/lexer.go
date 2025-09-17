package marigold

import (
	"fmt"
	"unicode"
)

func Lex(text string) ([]*Token, error) {

	if len(text) == 0 {
		return []*Token{}, ErrInvalidSourceCode
	}
	var filename string = "dummy"

	var cursor int = 0
	var start int = 0
	var row, col uint64 = 0, 0
	tokens := make([]*Token, 0)

	var increment = func() {
		switch text[cursor] {
		case '\n':
			row++
			col = 0
			cursor++
		case ' ', '\t', '\r':
			col++
			cursor++
		default:
			col++
			cursor++
		}

	}

	var makeToken = func(value string) {
		tokenType := TokenTypeMap[value]
		tokens = append(tokens, NewToken(value, tokenType, col, row, filename))
		increment()
	}

	for cursor < len(text) {
		currentChar := text[cursor]

		// Skip Whitespace
		switch currentChar {
		case ' ', '\t', '\r', '\n':
			increment()
			continue
		}

		// Operators & Syntax
		switch currentChar {
		case '(', ')', '[', ']', '{', '}', ',', '.', ':', ';', '+', '-', '*', '%', '^':
			makeToken(string(currentChar))
			continue

		// Tokens which can have a '=' extension e.g. > vs. >=
		case '=', '!', '<', '>':
			if cursor+1 < len(text) && text[cursor+1] == '=' {
				makeToken(string(text[cursor : cursor+2]))
				cursor++
			} else {
				makeToken(string(currentChar))
			}
			continue

		case '&':
			// Logical AND operator &&
			if cursor+1 < len(text) && text[cursor+1] == '&' {
				makeToken("&&")
				cursor++
			} else {
				panic(fmt.Sprintf("unexpected character '&' at %d:%d", row+1, col+1))
			}
			continue

		case '|':
			// Logical OR operator || or single pipe |
			if cursor+1 < len(text) && text[cursor+1] == '|' {
				makeToken("||")
				cursor++
			} else {
				makeToken(string(currentChar))
			}
			continue

		case '/':
			// Single line comments
			if cursor+1 < len(text) && text[cursor+1] == '/' {
				for cursor < len(text) && text[cursor] != '\n' {
					increment()
				}
			} else {
				makeToken(string(currentChar))
			}
			continue

		case '"':
			// String Literals
			startCol, startRow := col, row
			increment() // Skip opening quote

			var stringContent []byte
			for cursor < len(text) && text[cursor] != '"' {
				if text[cursor] == '\\' && cursor+1 < len(text) {
					// Handle escape sequences
					increment() // Skip backslash
					if cursor >= len(text) {
						return tokens, ErrUnterminatedStringLiteral
					}

					switch text[cursor] {
					case '"':
						stringContent = append(stringContent, '"')
					case '\\':
						stringContent = append(stringContent, '\\')
					case 'n':
						stringContent = append(stringContent, '\n')
					case 't':
						stringContent = append(stringContent, '\t')
					case 'r':
						stringContent = append(stringContent, '\r')
					default:
						// For unknown escapes, include both backslash and character
						stringContent = append(stringContent, '\\', text[cursor])
					}
					increment()
				} else {
					stringContent = append(stringContent, text[cursor])
					increment()
				}
			}

			if cursor >= len(text) {
				return tokens, ErrUnterminatedStringLiteral
			}

			// Create token with processed string content
			tokens = append(tokens, NewToken(string(stringContent), StringLiteral, startCol, startRow, filename))
			increment() // Skip closing quote
			continue
		}

		if unicode.IsDigit(rune(text[cursor])) {
			startCol, startRow := col, row
			start = cursor
			for cursor < len(text) && unicode.IsDigit(rune(text[cursor])) {
				increment()
			}

			// Check for floating point number
			if cursor < len(text) && text[cursor] == '.' && cursor+1 < len(text) && unicode.IsDigit(rune(text[cursor+1])) {
				// Only consume dot if there are digits after it
				increment() // consume the '.'
				for cursor < len(text) && unicode.IsDigit(rune(text[cursor])) {
					increment()
				}

				tokens = append(tokens, NewToken(string(text[start:cursor]), FloatLiteral, startCol, startRow, filename))
				continue
			} else {
				// Integer number (or number followed by dot with no digits)
				tokens = append(tokens, NewToken(string(text[start:cursor]), IntLiteral, startCol, startRow, filename))
				continue
			}
		}

		// Keyword & Identifier (ASCII only)
		if (text[cursor] >= 'a' && text[cursor] <= 'z') || (text[cursor] >= 'A' && text[cursor] <= 'Z') || text[cursor] == '_' || text[cursor] == '@' {
			startCol, startRow := col, row
			start = cursor
			for cursor < len(text) && ((text[cursor] >= 'a' && text[cursor] <= 'z') || (text[cursor] >= 'A' && text[cursor] <= 'Z') || (text[cursor] >= '0' && text[cursor] <= '9') || text[cursor] == '_' || text[cursor] == '@') {
				increment()
			}

			buffer := string(text[start:cursor])
			if toktype, ok := TokenTypeMap[buffer]; ok {
				tokens = append(tokens, NewToken(buffer, toktype, startCol, startRow, filename))
			} else {
				tokens = append(tokens, NewToken(buffer, Identifier, startCol, startRow, filename))
			}
			continue
		}

		panic(fmt.Sprintf("Unexpected character '%c' at %s:%d:%d", text[cursor], filename, row+1, col+1))

	}

	return tokens, nil
}
