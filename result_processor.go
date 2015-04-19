package main

import (
	"log"
	"reflect"
)

// Stores results in the database
type ResultProcessor struct {
	workers *WorkerPool
}

func NewResultProcessor(workersCount uint) *ResultProcessor {
	work := func(item interface{}) {
		switch res := item.(type) {
		case *DnsJobs:
			for _, job := range res.jobs {
				saveMxAddresses(job)
			}
		case *DnsJob:
			saveDomain(res)
		case *MxHost:
			saveMxHost(res)
		case *TxtRecord:
			saveMxDomain(res)
		default:
			log.Fatal("unknown db result:", reflect.TypeOf(res))
		}
	}

	return &ResultProcessor{workers: NewWorkerPool(workersCount, work)}
}

// Creates a new job
func (proc *ResultProcessor) Add(result interface{}) {
	proc.workers.Add(result)
}

// Stops the worker pool
func (proc *ResultProcessor) Close() {
	proc.workers.Close()
}
