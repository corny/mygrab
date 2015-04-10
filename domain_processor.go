package main

import (
	"github.com/miekg/dns"
	"github.com/zmap/zgrab/zlib"
	"log"
	"net"
	"reflect"
	"sync"
)

const (
	TypeMX   = dns.Type(dns.TypeMX)
	TypeA    = dns.Type(dns.TypeA)
	TypeAAAA = dns.Type(dns.TypeAAAA)
)

var (
	addressTypes = []dns.Type{TypeA, dns.Type(TypeAAAA)}
)

type DomainProcessor struct {
	// waiting domains
	workChannel chan string
	workDone    sync.WaitGroup

	dbChannel chan interface{}
	dbDone    sync.WaitGroup
}

func NewDomainProcessor(workersCount uint) *DomainProcessor {
	proc := &DomainProcessor{}

	proc.workChannel = make(chan string, 100)
	proc.workDone.Add(int(workersCount))

	proc.dbChannel = make(chan interface{}, 100)
	proc.dbDone.Add(1)

	// Start all workers
	for i := uint(0); i < workersCount; i++ {
		go proc.worker()
	}

	go proc.dbWorker()

	return proc
}

func (proc *DomainProcessor) worker() {
	for domain := range proc.workChannel {

		// Do the MX lookups
		mxJob := dnsProcessor.NewJob(domain, TypeMX)

		// Make mx hostnames unique
		mxHosts := UniqueStrings(mxJob.Results())

		proc.dbChannel <- mxJob

		// Do the A/AAAA lookups
		mxAddresses := dnsProcessor.NewJobs(mxHosts, addressTypes)

		proc.dbChannel <- mxAddresses

		// Make addresses unique
		addresses := UniqueStrings(mxAddresses.Results())

		// Do the bannergrabs
		for _, addr := range addresses {
			if false {
				zgrabProcessor.NewJob(&zlib.GrabTarget{Addr: net.ParseIP(addr)})
			}
		}
	}
	proc.workDone.Done()
}

// Creates a new job
func (proc *DomainProcessor) NewJob(domain string) {
	proc.workChannel <- domain
}

func (proc *DomainProcessor) dbWorker() {
	for result := range proc.dbChannel {
		switch res := result.(type) {
		case *DnsJobs:
			for _, job := range res.jobs {
				saveMxAddresses(job)
			}
		case *DnsJob:
			saveDomain(res)
		default:
			log.Fatal("unknown db result:", reflect.TypeOf(res))
		}
	}
	proc.dbDone.Done()
}

// Stops accepting new jobs and waits until all workers are finished
func (proc *DomainProcessor) Close() {
	close(proc.workChannel)
	proc.workDone.Wait()

	close(proc.dbChannel)
	proc.dbDone.Wait()
}
