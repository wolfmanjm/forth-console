This is a forth console inspired by e4thcomm.

It has line editing, command history, tab completion for filenames used for download.

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

if you type \i or \d then a TAB it will show all the forth source files available.

\d uses a fast download which basically streams to a large buffer then
 evaluates that buffer, really nice for large files you know compile ok. The
 required words for fast download can be found in the forth folder, and
 probably should be compiled into flash.

\i will download a line at a time checking for `ok.` if it doesn't get `ok.`
 it will stop, thisd is useful for sending smaller files that have just been
 developed and not tested yet.



