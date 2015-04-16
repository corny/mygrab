package main

import (
	"log"
)

// Does MX Lookups
type DomainProcessor struct {
	workers *WorkerPool
}

func NewDomainProcessor(workersCount uint) *DomainProcessor {
	work := func(item interface{}) {
		domain, ok := item.(string)
		if !ok {
			log.Fatal("unexpected object:", item)
		}

		log.Println(domain)

		// Do the A/AAAA lookups
		mxAddresses := dnsProcessor.NewJob(domain, TypeMX)
		mxAddresses.Wait()

		resultProcessor.Add(mxAddresses)
	}

	return &DomainProcessor{workers: NewWorkerPool(workersCount, work)}
}

// Creates a new job
func (proc *DomainProcessor) Add(domain string) {
	proc.workers.Add(domain)
}

// Stops the worker pool
func (proc *DomainProcessor) Close() {
	proc.workers.Close()
}
