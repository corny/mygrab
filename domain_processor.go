package main

import (
	"log"
)

type DomainProcessor struct {
	workers *WorkerPool
}

func NewDomainProcessor(workersCount uint) *DomainProcessor {
	work := func(item interface{}) {
		domain, ok := item.(string)
		if !ok {
			log.Fatal("unexpected object:", item)
		}

		// Do the A/AAAA lookups
		mxAddresses := dnsProcessor.NewJob(domain, TypeMX)
		mxAddresses.Wait()

		resultProcessor.Add(mxAddresses)
	}
	proc := &DomainProcessor{workers: NewWorkerPool(workersCount, work)}
	return proc
}

// Creates a new job
func (proc *DomainProcessor) Add(domain string) {
	proc.workers.Add(domain)
}

// Stops the worker pool
func (proc *DomainProcessor) Close() {
	proc.workers.Close()
}
