package main

import (
	"bufio"
	"errors"
	"net"
)

func processCommand(command string, input *bufio.Scanner, output *bufio.Writer) error {

	switch command {
	case "status":
		output.Write(status())
		_, err := output.Write([]byte("\n"))

		if err != nil {
			return err
		}

	case "import-domains":
		for input.Scan() {
			domainProcessor.Add(input.Text())
		}
	case "import-mx":
		for input.Scan() {
			mxProcessor.NewJob(input.Text())
		}
	case "import-addresses":
		for input.Scan() {
			zgrabProcessor.NewJob(net.ParseIP(input.Text()))
		}
	case "resolve-mx":
		resolveDomainMxHosts()
	default:
		return errors.New("unknown command: " + command)
	}

	return nil
}
