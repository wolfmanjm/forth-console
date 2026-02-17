package main

import (
	"fmt"
	"log"
	"net"
	"os"
)

func main() {
	conn, err := net.Dial("udp", "localhost:12345")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	message := []byte(os.Args[1])
	_, err = conn.Write(message)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Message sent!")
}
