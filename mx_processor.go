package main

import (
	"github.com/miekg/dns"
	"net"
)

var (
	addressTypes = []dns.Type{TypeA, dns.Type(TypeAAAA)}
)

type MxProcessor struct {
	workers *WorkerPool
}

func NewMxProcessor(workersCount uint) *MxProcessor {
	work := func(item interface{}) {
		hostname, _ := item.(string)

		// Do the A/AAAA lookups
		mxAddresses := dnsProcessor.NewJobs(hostname, addressTypes)
		mxAddresses.Wait()

		// Save results
		resultProcessor.Add(mxAddresses)

		// Make addresses unique
		addresses := UniqueStrings(mxAddresses.Results())

		// Do the bannergrabs
		for _, addr := range addresses {
			zgrabProcessor.NewJob(net.ParseIP(addr))
		}

	}

	return &MxProcessor{workers: NewWorkerPool(workersCount, work)}
}

// Creates a new job
func (proc *MxProcessor) NewJob(hostname string) {
	proc.workers.Add(hostname)
}

// Creates a new job
func (proc *MxProcessor) Close() {
	proc.workers.Close()
}
