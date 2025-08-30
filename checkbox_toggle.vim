nnoremap <buffer> <Space> :call ToggleCheckbox()<CR>

function! ToggleCheckbox()
  let line = getline('.')
  let col = col('.')
  
  " Check if cursor is on a checkbox pattern
  if match(line[col-3:col], '\[\s\]') >= 0 || match(line[col-3:col], '\[x\]') >= 0 ||
     \ match(line[col-2:col+1], '\[\s\]') >= 0 || match(line[col-2:col+1], '\[x\]') >= 0 ||
     \ match(line[col-1:col+2], '\[\s\]') >= 0 || match(line[col-1:col+2], '\[x\]') >= 0
    
    " Toggle the checkbox on current line
    if match(line, '\[ \]') >= 0
      s/\[ \]/[x]/
    elseif match(line, '\[x\]') >= 0
      s/\[x\]/[ ]/
    endif
  endif
endfunction