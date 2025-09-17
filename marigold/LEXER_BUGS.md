# Marigold Lexer Bug Report

## Critical Bugs Found

### 1. String Escape Sequences Not Processed ❌
**Issue**: Escape sequences are captured literally instead of being interpreted.

**Examples**:
- Input: `"test\\"` → Expected: `test\` → Got: `test\\`
- Input: `"say \"hello\""` → Expected: `say "hello"` → Got: `say \"hello\"`
- Input: `"a\\b\\c"` → Expected: `a\b\c` → Got: `a\\b\\c`

**Root Cause**: The lexer increments past escape sequences but doesn't process them.

### 2. Number Parsing Edge Cases ❌
**Issue**: Numbers ending with dots are incorrectly parsed as floats.

**Example**:
- Input: `123.` → Expected: 2 tokens (`123`, `.`) → Got: 1 token (`123.` as FloatLiteral)

**Root Cause**: Float parsing consumes the dot even when no digits follow.

### 3. Long Comments Return Nil ❌
**Issue**: Very long comments return `nil` instead of empty token slice.

**Example**:
- Input: `"// " + 1000 repeated words` → Expected: `[]` → Got: `nil`

**Root Cause**: Comment parsing may not return properly in edge cases.

### 4. Position Tracking Issues ❌
**Issue**: Position tracking counts unexpected tokens.

**Example**:
- Input: 3-line text → Expected: 3 tokens → Got: 6 tokens

**Root Cause**: Likely counting whitespace or other unexpected elements.

### 5. High ASCII Character Handling ❌
**Issue**: Some high ASCII characters don't panic as expected.

**Example**:
- Input: `\xFF` → Expected: panic → Got: no panic

**Root Cause**: Character validation may be incomplete.

## Working Features ✅

### EOF Handling
- Unterminated strings properly return errors
- EOF in comments handled gracefully
- EOF in operators handled properly

### Invalid Character Detection
- Unicode characters (α) ✅
- Null bytes ✅
- Control characters ✅
- Emojis (🚀) ✅
- Unrecognized punctuation (@#$%^&) ✅

### Large Input Robustness
- Very long identifiers (10,000 chars) ✅
- Very long strings ✅
- Large numbers ✅
- Deep nesting (1,000 levels) ✅

### Basic Functionality
- Keywords, identifiers, operators ✅
- Basic string literals ✅
- Integer and simple float literals ✅
- Comments (when not extremely long) ✅

## Severity Assessment

**High Priority** (Breaks basic functionality):
1. String escape sequences
2. Number parsing edge cases
3. Nil return from comments

**Medium Priority** (Edge cases):
4. Position tracking accuracy
5. High ASCII character handling

**Low Priority** (Works as designed):
- EOF handling
- Large input handling
- Basic token recognition

## Test Summary
- **Total Edge Case Tests**: ~30
- **Passing**: ~20 (67%)
- **Failing**: ~10 (33%)

The lexer is functional for basic use cases but has several edge case bugs that should be addressed for production use.