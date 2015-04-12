package main

import (
	"bufio"
	"log"
	"net"
	"os"
)

func processSocket() {
	os.Remove(socketPath)

	l, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatal("listen error:", err)
	}

	for {
		fd, err := l.Accept()
		if err != nil {
			log.Fatal("accept error:", err)
		}

		go echoServer(fd)
	}
}

func echoServer(c net.Conn) {
	scanner := bufio.NewScanner(c)

	for scanner.Scan() {
		//domainProcessor.Add(scanner.Text())
		scanner.Text()

		_, err := c.Write(status())

		if err != nil {
			log.Println("write error:", err)
		}
	}

	c.Close()
}
