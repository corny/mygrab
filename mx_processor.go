package main

import (
	"github.com/miekg/dns"
	"github.com/zmap/zgrab/zlib"
	"net"
)

const (
	TypeMX   = dns.Type(dns.TypeMX)
	TypeA    = dns.Type(dns.TypeA)
	TypeAAAA = dns.Type(dns.TypeAAAA)
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
			zgrabProcessor.NewJob(&zlib.GrabTarget{Addr: net.ParseIP(addr)})
		}
	}

	return &MxProcessor{workers: NewWorkerPool(workersCount, work)}
}

// Creates a new job
func (proc *MxProcessor) NewJob(hostname string) {
	proc.workers.Add(hostname)
}
