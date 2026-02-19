\ download to memory then evaluate
0 variable dlsize
0 variable sline
0 variable lcnt
0 variable dlbuf
0 variable dlbufsize

: +c ( char -- )
    dlbuf @ dlsize @ + c!
    1 dlsize +!
;

\ fast download file into buf
\ splits each line into a counted string
: dl
    compiletoram? 0= if ." ERROR Cannot fast download to flash, use \i" exit then
    \ decide the size of the buffer, for now use 1/4 the unused size (allows room for dictionary expansion)
    \ and leave 1/4 of the memory for flashvar and that leaves 1/2 for dictionary expansion
    unused 4 / dlbufsize !
    flashvar-here dlbufsize @ 2* - dlbuf !  \ address of the download buffer
    \ ." dlbuf address is: " dlbuf @ hex. ." size is: " dlbufsize @ . cr

    1 dlsize !      \ overall buffer size
    0 lcnt !        \ count of chars in line
    dlbuf @ sline !     \ start of line address

    ." READY Stream the file and terminate with ^D (\004), then use load to evaluate" cr

    begin
        key
        case
            4 of
                0 dlbuf @ dlsize @ + 1- c!
                ." DONE Bytes downloaded= " dlsize @ . exit
                endof
            10 of
                \ put line length into first byte
                lcnt @ sline @ c!
                0 lcnt !
                dlbuf @ dlsize @ + sline !  \ start of next line
                1 dlsize +!             \ add one for the byte count
                endof
            9 of 32 +c 1 lcnt +! endof  \ convert tab to space
            dup 32 < ?of ( skip control chars ) endof
                dup +c              \ add character to buffer
                1 lcnt +!
        endcase

        lcnt @ 200 >= if ." ERROR Line Too Long" cr quit then
        dlsize @ dlbufsize @ >= if ." ERROR Buffer Overflow" cr quit then
    again
;

\ load/evaluate the buf line by line
0 variable ecnt
: load
    dlsize @ 0 <= if ." nothing to load" cr exit then
    0 ecnt !
    begin
        ecnt @ dlbuf @  +   \ start of line
        count               \ c-addr u
        dup 1+ ecnt +!      \ increment read pointer start of next line
        ?dup 0<> if
            evaluate
        then
        ecnt @ dlsize @ >=
    until
    ." downloaded code loaded" cr
;
