\ download to memory then evaluate
0 variable dlsize
0 variable sline
0 variable lcnt
50000 buffer: buf

: +c ( char -- )
    buf dlsize @ + c!
    1 dlsize +!
;

\ fast download file into buf
\ splits each line into a counted string
: dl
    ." READY Stream the file and terminate with ^D (\004), then use load to evaluate" cr
    1 dlsize !      \ overall buffer size
    0 lcnt !        \ count of chars in line
    buf sline !     \ start of line address
    begin
        key
        case
            4 of
                0 buf dlsize @ + 1- c!
                ." DONE Bytes downloaded= " dlsize @ . exit
                endof
            10 of
                \ put line length into first byte
                lcnt @ sline @ c!
                0 lcnt !
                buf dlsize @ + sline !  \ start of next line
                1 dlsize +!             \ add one for the byte count
                endof
            9 of 32 +c 1 lcnt +! endof  \ convert tab to space
            dup 32 < ?of ( skip control chars ) endof
                dup +c              \ add character to buffer
                1 lcnt +!
        endcase

        lcnt @ 200 >= if ." ERROR Line Too Long" cr quit then
        dlsize @ 50000 >= if ." ERROR Buffer Overflow" cr quit then
    again
;

\ load/evaluate the buf line by line
0 variable ecnt
: load
    0 ecnt !
    begin
        ecnt @ buf +        \ start of line
        count               \ c-addr u
        dup 1+ ecnt +!      \ increment read pointer start of next line
        ?dup 0<> if
            evaluate
        then
        ecnt @ dlsize @ >=
    until
    ." downloaded code loaded"
;
