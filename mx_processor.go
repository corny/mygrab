package main

import (
	"github.com/miekg/dns"
	"log"
	"net"
	"time"
)

var (
	addressTypes = []dns.Type{TypeA, dns.Type(TypeAAAA)}
)

type MxProcessor struct {
	cache *CachedWorkerPool
}

func NewMxProcessor(workersCount uint, cacheConfig *CacheConfig) *MxProcessor {
	proc := &MxProcessor{}
	proc.cache = NewCachedWorkerPool(workersCount, proc.work, cacheConfig)
	return proc
}

func (proc *MxProcessor) NewJob(hostname string) *CacheEntry {
	return proc.cache.NewJob(hostname, time.Now())
}

// If the hostname exists in the cache it returns its Value.
// Otherwise is creates a job and returns nil.
func (proc *MxProcessor) GetValue(hostname string) *string {
	job := proc.NewJob(hostname)
	value, _ := job.Value.(*string)
	return value
}

// Stops accepting new jobs and waits until all jobs are finished
func (proc *MxProcessor) Close() {
	proc.cache.Close()
}

func (proc *MxProcessor) work(obj interface{}) {
	entry, _ := obj.(*CacheEntry)
	hostname := entry.Key

	// Do the A/AAAA lookups
	mxAddresses := dnsProcessor.NewJobs(hostname, addressTypes)
	mxAddresses.Wait()

	// Make addresses unique
	addresses := UniqueStrings(mxAddresses.Results())

	jobs := make([]*CacheEntry, len(addresses))
	hosts := make([]*MxHostSummary, len(addresses))

	// Run the host checks
	for i, addr := range addresses {
		// Pass the access time from the host entry
		jobs[i] = hostProcessor.NewJobWithAccessTime(net.ParseIP(addr), entry.Accessed)
	}

	// Wait for the host checks to be finished
	for i, job := range jobs {
		job.Wait()
		hostSummary, _ := job.Value.(*MxHostSummary)
		hosts[i] = hostSummary
	}

	txtRecord := createTxtRecord(hostname, hosts)
	txtString := txtRecord.String()

	// Set value for the cache
	entry.Value = &txtString

	log.Println("TXT:", txtString)

	// Update Nameserver
	if nsUpdater != nil {
		nsUpdater.NewJob(hostname, txtString)
	}

	// Save to database
	if resultProcessor != nil {
		resultProcessor.Add(&MxRecord{mxAddresses, &txtRecord})
	}
}
