This is a forth console inspired by e4thcomm.
It has line editing, command history, tab completion for filenames used for download.

Command line options..

  -I string
      append directories to include path (default ".:..")
  -b int
      Set serial baudrate (default 3000000)
  -d string
     Set serial port (default "/dev/ttyUSB0")

Once run you get a prompt, normal readline editing features are used so up arrow is get las tin history, left/right to edit the line etc
At the prompt you can prefix a command by \ the available commands are:

  \q - Quit
  \d fn - Fast Download file with requires/includes
  \i fn - download file with requires/includes, using ping pong (slow)
  \p - pastes clipboard with ping pong
  \br - send ^D
