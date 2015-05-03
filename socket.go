package main

import (
	"bufio"
	"log"
	"net"
	"os"
)

func controlSocket() {
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

		go func() {
			input := bufio.NewScanner(fd)
			output := bufio.NewWriter(fd)
			input.Scan()
			if err = processCommand(input.Text(), input, output); err != nil {
				output.WriteString(err.Error() + "\n")
			}
			output.Flush()
			fd.Close()
		}()
	}
}
