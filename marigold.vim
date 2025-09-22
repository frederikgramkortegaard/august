" Vim syntax file
" Language: Marigold
" Maintainer: August Team
" Latest Revision: 2025

if exists("b:current_syntax")
  finish
endif

" Comments FIRST - highest priority to prevent other syntax inside
syn match marigoldComment "//.*$" contains=@Spell
syn region marigoldComment start="/\*" end="\*/" contains=@Spell

" String literals (also high priority)
syn region marigoldString start='"' end='"' skip='\\"' keepend
syn region marigoldString start="'" end="'" skip="\\'" keepend

" Keywords
syn keyword marigoldKeyword define return if else while break continue
syn keyword marigoldKeyword map len emit stop assert

" Types
syn keyword marigoldType string int float bool

" Built-in Functions
syn keyword marigoldBuiltin len emit stop assert string int float

" Boolean literals
syn keyword marigoldBoolean true false

" Blockchain context variables (starting with @)
syn match marigoldChainVar "@\w\+"

" Numeric literals
syn match marigoldNumber "\<\d\+\>"
syn match marigoldFloat "\<\d\+\.\d\+\>"

" Operators
syn match marigoldOperator "+"
syn match marigoldOperator "-"
syn match marigoldOperator "\*"
syn match marigoldOperator "/"
syn match marigoldOperator "%"
syn match marigoldOperator "\^"
syn match marigoldOperator "="
syn match marigoldOperator "=="
syn match marigoldOperator "!="
syn match marigoldOperator "<"
syn match marigoldOperator "<="
syn match marigoldOperator ">"
syn match marigoldOperator ">="
syn match marigoldOperator "&&"
syn match marigoldOperator "||"

" Delimiters
syn match marigoldDelimiter "("
syn match marigoldDelimiter ")"
syn match marigoldDelimiter "{"
syn match marigoldDelimiter "}"
syn match marigoldDelimiter "\["
syn match marigoldDelimiter "\]"
syn match marigoldDelimiter ","
syn match marigoldDelimiter ":"
syn match marigoldDelimiter ";"

" Function definitions
syn match marigoldFunction "\<\w\+\>\s*(" contains=marigoldFunctionName
syn match marigoldFunctionName "\<\w\+\>" contained

" Storage variables
syn keyword marigoldStorage persistent memory

" String slicing (new feature)
syn match marigoldSlice "\[\s*\d*\s*:\s*\d*\s*\]"

" Map indexing
syn match marigoldMapIndex "\[\s*@\?\w\+\s*\]"

" Type annotations in variable declarations
syn match marigoldTypeAnnotation ":\s*\(string\|int\|float\|bool\|map\)"

" Highlight groups
hi def link marigoldKeyword Keyword
hi def link marigoldType Type
hi def link marigoldBuiltin Function
hi def link marigoldBoolean Boolean
hi def link marigoldChainVar Special
hi def link marigoldString String
hi def link marigoldNumber Number
hi def link marigoldFloat Float
hi def link marigoldComment Comment
hi def link marigoldOperator Operator
hi def link marigoldDelimiter Delimiter
hi def link marigoldFunction Function
hi def link marigoldFunctionName Function
hi def link marigoldStorage StorageClass
hi def link marigoldSlice Special
hi def link marigoldMapIndex Identifier
hi def link marigoldTypeAnnotation Type

let b:current_syntax = "marigold"