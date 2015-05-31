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
	nsupdateBatchSize = 500
	dnsMaxItemLength  = 255
)

var (
	nsupdateServer = "127.0.0.1"
	nsupdateKey    string
)

type NsUpdateJob struct {
	domain string
	txt    string
}

type NsUpdater struct {
	channel chan *NsUpdateJob
	sync.WaitGroup
}

func NewNsUpdater() *NsUpdater {
	updater := &NsUpdater{}
	updater.channel = make(chan *NsUpdateJob, nsupdateBatchSize)
	updater.Add(1)
	go updater.worker()
	return updater
}

func (updater *NsUpdater) NewJob(domain string, txt string) {

	updater.channel <- &NsUpdateJob{domain: domain, txt: txt}
}

func (updater *NsUpdater) Close() {
	close(updater.channel)
	updater.Wait()
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
			updater.Done()
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
	buffer := bytes.NewBufferString(fmt.Sprintf("update delete %s TXT\nupdate add %s %d TXT", domain, domain, dnsTTL))
	chunks := SplitByLength(job.txt, dnsMaxItemLength)

	for _, chunk := range chunks {
		buffer.WriteString(" \"")
		buffer.WriteString(chunk)
		buffer.WriteString("\"")
	}

	buffer.WriteString("\n")
	return buffer.Bytes()
}
