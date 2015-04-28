package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
)

const (
	nsupdateBatchSize     = 500
	nsupdateMaxItemLength = 255
)

var (
	dnsZone             = "tls-scan.informatik.uni-bremen.de"
	nsupdateServer      = "127.0.0.1"
	nsupdateTTL    uint = 900
	nsupdateKey    string
)

type NsUpdateJob struct {
	domain string
	txt    string
}

type NsUpdater struct {
	channel chan *NsUpdateJob
	wg      sync.WaitGroup
}

func NewNsUpdater() *NsUpdater {
	updater := &NsUpdater{}
	updater.channel = make(chan *NsUpdateJob, nsupdateBatchSize)
	updater.wg.Add(1)
	go updater.worker()
	return updater
}

func (updater *NsUpdater) Add(domain string, txt string) {

	updater.channel <- &NsUpdateJob{domain: domain, txt: txt}
}

func (updater *NsUpdater) Close() {
	close(updater.channel)
	updater.wg.Wait()
}

func (updater *NsUpdater) worker() {
	var stdin io.WriteCloser

	for {
		cmd := exec.Command("/usr/bin/nsupdate", "-k", nsupdateKey)
		stdin, _ = cmd.StdinPipe()
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout

		// first job (blocking read)
		log.Println("nsupdate: waiting for jobs")
		job := <-updater.channel
		if job == nil {
			log.Println("nsupdate: worker finished")
			updater.wg.Done()
			return
		}

		// Start process
		err := cmd.Start()
		if err != nil {
			log.Fatal(err)
		}

		// Introduce the update
		stdin.Write([]byte(fmt.Sprintf("server %s\nzone %s\n", nsupdateServer, dnsZone)))

		// send the first job
		stdin.Write(job.Bytes())

		// other jobs (non-blocking read)
		for i := 0; i < nsupdateBatchSize; i += 1 {
			log.Println("nsupdate: select more jobs")
			select {
			case job = <-updater.channel:
				stdin.Write(job.Bytes())
			default:
				job = nil
			}
			if job == nil {
				break
			}
		}

		stdin.Write([]byte("send\n"))
		stdin.Close()

		err = cmd.Wait()

		if err != nil {
			log.Fatalf("nsupdate: finished with error: %v", err)
		} else {
			log.Println("nsupdate: finished successfully")
		}
	}
}

// Creates commands to delete and add the TXT record
func (job *NsUpdateJob) Bytes() []byte {
	domain := job.domain + "." + dnsZone
	length := len(job.txt)
	chunks := length / nsupdateMaxItemLength
	buffer := bytes.NewBufferString(fmt.Sprintf("update delete %s TXT\nupdate add %s %d TXT", domain, domain, nsupdateTTL))

	// we need at least one chunk
	if chunks == 0 {
		chunks = 1
	}

	// Long data must be splittet into multiple chunks
	for i := 0; i < chunks; i++ {
		buffer.WriteString(" \"")
		if i == chunks-1 {
			// the last chunk, there is no maximum function for two integers in Go
			buffer.Write([]byte(job.txt[i*nsupdateMaxItemLength:]))
		} else {
			buffer.Write([]byte(job.txt[i*nsupdateMaxItemLength : (nsupdateMaxItemLength * (i + 1))]))
		}
		buffer.WriteString("\"")
	}

	buffer.WriteString("\n")
	return buffer.Bytes()
}
