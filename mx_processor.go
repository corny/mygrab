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

		// Save DNS results
		resultProcessor.Add(mxAddresses)

		// Make addresses unique
		addresses := UniqueStrings(mxAddresses.Results())

		jobs := make([]*ZgrabJob, len(addresses))
		hosts := make([]*MxHost, len(addresses))

		for i, addr := range addresses {
			// Do the bannergrabs
			jobs[i] = zgrabProcessor.NewJob(net.ParseIP(addr))
		}

		for i, job := range jobs {
			job.Wait()
			hosts[i] = job.Result
		}

		txt := createTxtRecord(hostname, hosts)

		nsUpdater.Add(hostname, txt.String())
		resultProcessor.Add(&txt)

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
