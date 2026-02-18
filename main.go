package main

// forth console somewhat like e4thcom but not quite as heavy
// Adding ability to stream code using the dl word

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/chzyer/readline"
	"go.bug.st/serial"
	"golang.design/x/clipboard"
)

func filterInput(r rune) (rune, bool) {
	switch r {
	// block CtrlZ feature
	case readline.CharCtrlZ:
		return r, false
	}
	return r, true
}

// stores the last line recieved
// useful for checking the ok.
var lastLine string
var cond *sync.Cond = sync.NewCond(&sync.Mutex{})
var incPath []string
var lastFile string

// Function constructor - constructs new function for listing given directory
func listFiles() func(string) []string {
	return func(line string) []string {
		names := make([]string, 0)
		for _, basedir := range incPath {
		    for _, ext := range []string{"fs", "f", "txt"} {
	        	pattern := filepath.Join(basedir, "*."+ext)
		        matches, _ := filepath.Glob(pattern)
	    	    names = append(names, matches...)
	    	}
	    }
		return names
	}
}

var completer = readline.NewPrefixCompleter(
	readline.PcItem("\\i",
		readline.PcItemDynamic(listFiles(),
			readline.PcItem("with",
				readline.PcItem("following"),
				readline.PcItem("items"),
			),
		),
	),
	readline.PcItem("\\d",
		readline.PcItemDynamic(listFiles(),
			readline.PcItem("with",
				readline.PcItem("following"),
				readline.PcItem("items"),
			),
		),
	),
)

func main() {
	// get flags
	dev := flag.String("d", "/dev/ttyUSB0", "Set serial port")
	baud := flag.Int("b", 3000000, "Set serial baudrate")
	p := flag.String("I", ".:..", "append directories to include path")

	flag.Parse()
	incPath = strings.Split(*p, ":")

	// open serial port
	mode := &serial.Mode{
		BaudRate: *baud,
		Parity:   serial.NoParity,
		DataBits: 8,
		StopBits: serial.OneStopBit,
	}
	s, err := serial.Open(*dev, mode)
	if err != nil {
		panic("unable to open serial port: " + *dev + ": " + err.Error())
	}
	defer s.Close()

	// open readline
	l, err := readline.NewEx(&readline.Config{
		Prompt:      "\033[31m»\033[0m ",
		HistoryFile: "./.history",
		AutoComplete:    completer,
		InterruptPrompt:     "^C",
		EOFPrompt:           "exit",
		UniqueEditLine:      true,
		HistorySearchFold:   true,
		FuncFilterInputRune: filterInput,
	})
	if err != nil {
		panic(err)
	}
	defer l.Close()
	//l.CaptureExitSignal()


	// serial reader
	ch := make(chan string, 100)
	defer close(ch)

	go readLoop(s, ch)

	// display any lines we get
	go func() {
		for s := range ch {
			fmt.Fprint(l.Stdout(), s)
			cond.L.Lock()
			lastLine = strings.TrimSuffix(s, "\n")
			cond.L.Unlock()
			cond.Signal()
		}
	}()

	go udpListen(l, s)

	for {
		line, err := l.Readline()
		if err == readline.ErrInterrupt {
			if len(line) == 0 {
				break
			} else {
				continue
			}
		} else if err == io.EOF {
			break
		}

		if strings.HasPrefix(line, "\\") {
			handleCommand(l, s, line[1:])
		} else {
			s.Write([]byte(line + "\n"))
		}
	}
}

func handleCommand(l *readline.Instance, s serial.Port, line string) {
	switch {
	case strings.HasPrefix(line, "q"):
		os.Exit(0)

	case strings.HasPrefix(line, "d") || strings.HasPrefix(line, "i"):
		fn := strings.Trim(line[1:], " ")
		lns, err := processRequireFiles("", l, fn)
		if err != nil {
			fmt.Fprintln(l.Stderr(), "process file: ", err.Error())
			return
		}

		if len(lns) > 0 {
			if strings.HasPrefix(line, "d") {
				downLoadLines(l, s, lns)
			} else {
				sendLinesWithOk(l, s, lns)
			}
			lastFile = fn
		}

	case strings.HasPrefix(line, "p"):
		fmt.Fprintln(l.Stderr(), "Paste with handshaking")
		err := clipboard.Init()
		if err == nil {
			// Read from clipboard
			str := string(clipboard.Read(clipboard.FmtText))
			li := strings.Split(str, "\n")
			sendLinesWithOk(l, s, li)
		} else {
			fmt.Fprintln(l.Stderr(), "Clipboard error: ", err.Error())
		}

	case strings.HasPrefix(line, "br"):
		fmt.Fprintln(l.Stderr(), "Send Break")
		s.Write([]byte("\004"))

	case strings.HasPrefix(line, "?") || strings.HasPrefix(line, "h"):
		fmt.Fprintln(l.Stderr(), `Available Commands:
		q - Quit
		d fn - Fast Download file with requires/includes
		i fn - download file with requires/includes, using ping pong
		p - pastes clipboard with ping pong
		br - send ^D
		`)

	default:
		fmt.Fprintln(l.Stderr(), "Unknown command: "+line)
		return
	}
}

// lookup file on the incPath
func lookupFile(filename string) (string) {
	for _, basepath := range incPath {
		matches, err := filepath.Glob(path.Join(basepath, filename))
		if len(matches) == 0 || err != nil{
			continue
		}
		return matches[0]
	}
	return ""
}

var rc1 = regexp.MustCompile(`\s+(\\ .*)`)  // remove \ comment at end of line
var rc2 = regexp.MustCompile(`[ \t]\(.*\)`)  // remove ( ) comment in line
var rcbl = regexp.MustCompile(`^\s*\\\s.*`) // matches a blank line with only comment

var processed []string

// this will process a .fs file with #require and include the required files once
// returns a slice of all the lines to send
func processRequireFiles(parent string, l *readline.Instance, fn string) ([]string, error) {
	if parent == "" { processed = make([]string, 0, 4) }
	fmt.Fprintln(l.Stderr(), "Process file: " + fn)

	lfn := lookupFile(fn)
	if lfn == "" {
		return nil, fmt.Errorf("processRequireFiles file not found: %s", fn)
	}

	fn = lfn
	f, err := os.Open(fn)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// add to list of processed files, stops recursion as well
    processed = append(processed, fn)

	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 1024)
	scanner.Buffer(buf, 1024)

	lines := make([]string, 0, 100)

	// Iterate over each line
	for scanner.Scan() {
		line := strings.Trim(scanner.Text(), " \t")
		if line == "" { continue }

		// skip lines that only have spaces and a comment
        if rcbl.MatchString(line) { continue }

        if strings.Contains(line, "compiletoflash") {
            fmt.Fprintln(l.Stderr(), "** Warning: compiletoflash stripped from file: ", fn)
            continue
		}

        if strings.Contains(line, "compiletoram") {
            fmt.Fprintln(l.Stderr(), "** Warning: compiletoram stripped from file: ", fn)
            continue
		}

        if strings.HasPrefix(line, "#require ") || strings.HasPrefix(line, "#include ") {
            _, rfn, ok := strings.Cut(line, " ")
            if !ok {
       			return nil, fmt.Errorf("malformed require in file: %s - %s", fn, line)
            }

            rfn = strings.Trim(rfn, " ")
            if rfn == fn {
       			return nil, fmt.Errorf("cannot require itself: %s - %s", rfn, line)
            }

            // check it has not already been processed
            if slices.Index(processed, rfn) == -1 {
                fmt.Fprintf(l.Stderr(), "*** Including %s ***\n", rfn)
                s, err := processRequireFiles(fn, l, rfn)
                if err != nil {
                	return nil, fmt.Errorf("processing required file: %s - %w", rfn, err)
                }

                lines = append(lines, s...)

			} else {
				fmt.Fprintln(l.Stderr(), "** INFO: already processed file: ", rfn)
			}

		} else {
			// strip out comments from line
	        line = rc1.ReplaceAllString(line, "")
	        line = rc2.ReplaceAllString(line, "")

	        lines = append(lines, line)
		}
	}

	// Check for errors during scanning
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading file: %s - %w", fn, err)
	}

	return lines, nil
}

// Fast downloads lines using the dl word
func downLoadLines(l *readline.Instance, s serial.Port, lns []string) {
	fmt.Fprintln(l.Stderr(), "Fast Download file")

	ok := sendWaitForResponse(s, "dl")
	if !strings.HasPrefix(ok, "dl READY") {
		fmt.Fprintln(l.Stderr(), "Did not get READY got: " + ok)
		return
	}

    // Iterate over each line
    for _, line := range lns {
     	s.Write([]byte(line + "\n"))
    }

	// send ^D terminator then the load command
	ok = sendWaitForResponse(s, "\004")
	if !strings.HasPrefix(ok, "DONE") {
		fmt.Fprintln(l.Stderr(), "Did not get DONE got: " + ok)
		return
	}
	s.Write([]byte("load\n"))
}

// sends a command and waits for the response which it returns
func sendWaitForResponse(s serial.Port, str string) string {
	cond.L.Lock()
	defer cond.L.Unlock()
	if len(str) == 1 && str[0] == 4 {
		s.Write([]byte("\004"))
	} else {
		s.Write([]byte(str + "\n"))
	}
	cond.Wait()
	return lastLine
}

// send each line of the string and wait for ok. or other error
func sendLinesWithOk(l *readline.Instance, s serial.Port, li []string) {
	for _, ln := range li {
		cond.L.Lock()
		s.Write([]byte(ln + "\n"))
		// waits until we get a reply
		cond.Wait()
		if !strings.HasSuffix(lastLine, "ok.") {
			fmt.Fprintln(l.Stderr(), "Got error: ", lastLine)
			cond.L.Unlock()
			break
		}
		cond.L.Unlock()
	}
}

// this reads from the serial line and sends whole lines to the channel
// it buffers up partial lines
func readLoop(s serial.Port, ch chan string) {
	buf := make([]byte, 1024)
	var rdBuf bytes.Buffer

	for {
		// Reads up to length of buf (which is 1024 bytes)
		n, err := s.Read(buf)
		if err != nil {
			fmt.Println("readLoop exiting: " + err.Error())
			return
		}

		// if we have a partial line left over from last time then append new data to it
		if rdBuf.Len() > 0 {
			rdBuf.Write(buf[:n])
			// this reads the last fragment prepended to the new data
			n, _ = rdBuf.Read(buf)
			if n >= len(buf) {
				fmt.Println("line too long, discarded")
				rdBuf.Reset()
				continue
			}
		}

		// process each line
		for l := range bytes.Lines(buf[:n]) {
			if bytes.HasSuffix(l, []byte("\n")) {
				ch <- string(l)
			} else {
				rdBuf.Write(l)
			}
		}
	}
}

// listen for UDP packet, used to trigger download
func udpListen(l *readline.Instance, s serial.Port) {

	address, err := net.ResolveUDPAddr("udp", ":12345")
	if err != nil {
		fmt.Println(err)
	}

	conn, err := net.ListenUDP("udp", address)
	if err != nil {
		fmt.Println(err)
	}
	defer conn.Close()

	buffer := make([]byte, 1024)
	for {
		n, addr, err := conn.ReadFromUDP(buffer)
		_ = addr
		if err != nil {
			fmt.Fprintln(l.Stderr(), "Got UDP error: ", err.Error())
			continue
		}

		// if lastFile == "" { continue }
		if n > 0 {
			fn := string(buffer[0:n])
			fmt.Fprintln(l.Stderr(), "download file: " + fn)
			handleCommand(l, s, "d " + fn)
		}
	}
}
