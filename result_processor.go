package main

import (
	"github.com/zmap/zgrab/ztools/x509"
	"log"
	"reflect"
)

// Stores results in the database
type ResultProcessor struct {
	workers *WorkerPool
}

type MxRecord struct {
	*DnsJobs
	*TxtRecord
}

func NewResultProcessor(workersCount uint) *ResultProcessor {
	work := func(item interface{}) {
		switch res := item.(type) {
		case *DnsJob:
			saveDomain(res)
		case *MxHostSummary:
			saveMxHostSummary(res)
		case *MxRecord:
			saveMxRecord(res)
		case *x509.Certificate:
			saveCertificate(res)
		case []*x509.Certificate:
			for _, cert := range res {
				saveCertificate(cert)
			}
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
