package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"github.com/zmap/zgrab/ztools/x509"
	"net"
)

func processCommand(command string, input *bufio.Scanner, output *bufio.Writer) error {

	var str []byte
	var err error

	switch command {
	case "status":
		str, err = status()
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
			hostProcessor.NewJob(net.ParseIP(input.Text()))
		}
	case "import-certificates":
		// Read input to buffer
		buffer := new(bytes.Buffer)
		for input.Scan() {
			buffer.Write(input.Bytes())
			buffer.WriteString("\n")
		}

		var pemBlock *pem.Block
		bytes := buffer.Bytes()

		for {
			pemBlock, bytes = pem.Decode(bytes)
			if pemBlock == nil {
				break
			}
			if cert, err := x509.ParseCertificate(pemBlock.Bytes); err == nil {
				output.WriteString(hex.EncodeToString([]byte(cert.FingerprintSHA1)) + "\n")
				resultProcessor.Add(cert)
			}
		}
		output.Flush()
	case "resolve-mx":
		resolveDomainMxHosts()
	case "cache-mx":
		str, err = cacheStatus(mxProcessor.cache, nil)
	case "cache-hosts":
		converter := func(str string) string {
			return net.IP(str).String()
		}
		str, err = cacheStatus(hostProcessor.cache, converter)
	default:
		return errors.New("unknown command: " + command)
	}

	if err != nil {
		return err
	}
	if str != nil {
		_, err = output.Write(str)
		if err != nil {
			return err
		}
		output.Write([]byte("\n"))
	}

	return nil
}
