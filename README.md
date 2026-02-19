This is a forth console inspired by e4thcomm.

It has line editing, command history, TAB completion for filenames used for download.

Command line options..

	  -I string
        append directories to include path (default ".:..")
	  -b int
	    Set serial baudrate (default 3000000)
	  -d string
	    Set serial port (default "/dev/ttyUSB0")

Once run you get a prompt, normal readline editing features are used so up arrow is get last in history, left/right to edit the line etc.

At the prompt you can prefix a command by \ the available commands are:

	  \q - Quit
	  \d fn - Fast Download file with requires/includes
	  \i fn - download file with requires/includes, using ping pong (slow)
	  \p - pastes clipboard with ping pong
	  \br - send ^D

Also ^D will quit and ^C will interrupt.

If you type \i or \d then a TAB it will show all the forth source files
available whicj you can select with TAB.

\d uses a fast download which basically streams to a large buffer then
 evaluates that buffer, really nice for large files you know compile ok. The
 required words for fast download can be found in the forth folder, and
 probably should be compiled into flash.

\i will download a line at a time checking for `ok.` if it doesn't get `ok.`
 it will stop, thisd is useful for sending smaller files that have just been
 developed and not tested yet.

When using the \i or \d commands it will process the file and and
`#require filename` or `#include filename` found in the source will insert that file
into the download stream (only once if it is found a second time it will be
ignored). This is non-standard forth so it will also do the same for
`\ #require` and `\ #include`


This program will also listen on UDP port 12345, this is used to tell it to
fast download a file externally from a editor (I use sublimetext). You would
add a comamnd to the editor that calls `udp-send filename`, udp-send can be
found in the cmd folder and can be built with go, various versions of the
executable are also found there.

An example of the build system I use for sublimetext editor...

	"build_systems": [
        {
            "name": "forth",
            "cmd": ["./cmd/udp-send", "$file"],
            "working_dir": "${file_path}",
            "file_regex": "^(..[^:]*):([0-9]+):?([0-9]+)?:? (.*)$",
        },
	],

Another handy feature is the `\p` command this will send whatever is in the
clipboard to the target, checking `ok.` as it goes. Really handy for sending
small clips of forth code.

Lastly various binaries are precompiled and stored in the bins folder.
