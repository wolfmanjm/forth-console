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
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/chzyer/readline"
	"go.bug.st/serial"
	"golang.design/x/clipboard"
)

var completer = readline.NewPrefixCompleter()

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

func main() {
	l, err := readline.NewEx(&readline.Config{
		Prompt:      "\033[31m»\033[0m ",
		HistoryFile: "./.history",
		// AutoComplete:    completer,
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
	l.CaptureExitSignal()

	// get flags
	dev := flag.String("d", "/dev/ttyUSB0", "Set serial port")
	baud := flag.Int("b", 3000000, "Set serial baudrate")
	incPath := flag.String("I", ".:..", "append directories to include path")
	_ = incPath

	flag.Parse()

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

	// go udpListen()

	/*
		// Handle SIGHUP
		go func() {
		    c := make(chan os.Signal, 1)
		    signal.Notify(c, syscall.SIGHUP)
		    for {
		        s := <-c
		        switch s {
		        case syscall.SIGHUP:
		        	fmt.Println("Got SIGHUP")
		        }
		    }
		}()
	*/

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

	case strings.HasPrefix(line, "d"):
		downLoadFile(l, s, strings.Trim(line[1:], " "))

	case strings.HasPrefix(line, "i"):
		lns, err := processRequireFiles(l, strings.Trim(line[1:], " "))
		if err != nil {
			fmt.Fprintln(l.Stderr(), "process file: ", err.Error())
		}
		for _, ln := range lns {
			fmt.Fprintln(l.Stderr(), ln)
		}

	case strings.HasPrefix(line, "p"):
		fmt.Fprintln(l.Stderr(), "Paste with handshaking")
		err := clipboard.Init()
		if err == nil {
			// Read from clipboard
			str := string(clipboard.Read(clipboard.FmtText))
			sendLinesWithOk(l, s, str)
		} else {
			fmt.Fprintln(l.Stderr(), "Clipboard error: ", err.Error())
		}

	case strings.HasPrefix(line, "br"):
		fmt.Fprintln(l.Stderr(), "Send Break")
		s.Write([]byte("\004"))

	default:
		fmt.Fprintln(l.Stderr(), "Unknown command: "+line)
		return
	}
}

var rc1 = regexp.MustCompile(`\s+(\\ .*)`)  // remove \ comment at end of line
var rc2 = regexp.MustCompile(`[ \t]\(.*\)`)  // remove ( ) comment in line
var processed = make([]string, 0, 4)

// this will process a .fs file with #require and include the required files once
// returns a slice of all the lines to send
func processRequireFiles(l *readline.Instance, fn string) ([]string, error) {
	fmt.Fprintln(l.Stderr(), "Process file: " + fn)

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
        if b, _ := regexp.MatchString(`^\s*\\\s.*`, line); b { continue }

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

            if rfn == fn {
       			return nil, fmt.Errorf("cannot require itself: %s - %s", rfn, line)
            }

            if slices.Index(processed, rfn) == -1 {
                fmt.Fprintf(l.Stderr(), "*** Including %s ***\n", rfn)
                s, err := processRequireFiles(l, rfn)
                if err != nil {
                	return nil, fmt.Errorf("processing required file: %s - %w", rfn, err)
                }
                // we need to prepend the required files
                lines = append(s, lines...)

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

// Fast downloads a file using the dl word
func downLoadFile(l *readline.Instance, s serial.Port, fn string) {
	fmt.Fprintln(l.Stderr(), "Fast Download file: <"+fn+">")
	f, err := os.Open(fn)
	if err != nil {
		fmt.Fprintf(l.Stderr(), "%v\n", err)
		return
	}
	defer f.Close()

	s.Write([]byte("dl\n"))
	time.Sleep(10 * time.Millisecond)

	// this would be better using the newer dl word
	// ok := sendWaitForResponse(s, "dl")
	// if !strings.HasPrefix(ok, "READY") {
	// 	fmt.Fprintln(l.Stderr(), "Did not get READY got: " + ok)
	// 	return
	// }
	// Create a new Scanner for the file

	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 1024)
	scanner.Buffer(buf, 1024)

    // Iterate over each line
    for scanner.Scan() {
        line := scanner.Text()
     	s.Write([]byte(line + "\n"))
    }

	// Check for errors during scanning
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(l.Stderr(), "Error reading file: %s\n", err.Error())
		return
	}

	// send ^D terminator then the load command
	s.Write([]byte("\004"))
	time.Sleep(10 * time.Millisecond)
	s.Write([]byte("load\n"))
}

// sends a command and waits for the response which it returns
func sendWaitForResponse(s serial.Port, str string) string {
	cond.L.Lock()
	defer cond.L.Unlock()
	s.Write([]byte(str + "\n"))
	cond.Wait()
	return lastLine
}

// send each line of the string and wait for ok. or other error
func sendLinesWithOk(l *readline.Instance, s serial.Port, str string) {
	li := strings.SplitSeq(str, "\n")
	for ln := range li {
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

// listen for UDP packet, can be used to trigger an event
func udpListen() {
	addr := &net.UDPAddr{
		IP:   net.IPv4(0, 0, 0, 0),
		Port: 12345,
		Zone: "",
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return
	}
	defer conn.Close()

	buf := make([]byte, 128)
	for {
		_, _, err = conn.ReadFrom(buf)
		if err != nil {
			fmt.Println("Got UDP error: ", err.Error())
		} else{
			fmt.Println("Got UDP packet")
		}
	}
}
