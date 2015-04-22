package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
)

const (
	nsupdateBatchSize = 500
	nsupdateDomain    = 50
	dnsServer         = "127.0.0.1"
	dnsZone           = "tls-scan.informatik.uni-bremen.de"
)

var (
	nsupdateKey string
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

	update := func(job *NsUpdateJob) {
		domain := job.domain + "." + dnsZone
		stdin.Write([]byte(fmt.Sprintf("update delete %s TXT\nupdate add %s %d TXT \"%s\"\n", domain, domain, 900, job.txt)))
	}

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
		stdin.Write([]byte(fmt.Sprintf("server %s\nzone %s\n", dnsServer, dnsZone)))

		// send the first job
		update(job)

		// other jobs (non-blocking read)
		for i := 0; i < nsupdateBatchSize; i += 1 {
			log.Println("nsupdate: select more jobs")
			select {
			case job = <-updater.channel:
				update(job)
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
