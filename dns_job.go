package main

import (
	"errors"
	"github.com/miekg/dns"
	"github.com/miekg/unbound"
	"log"
	"strconv"
	"sync"
)

type DnsQuery struct {
	Domain string
	Type   dns.Type
}

type DnsResult struct {
	// The result
	Results  []string
	Error    *error
	Secure   bool
	WhyBogus *string
}

type DnsJob struct {
	// waitGroup for the waiting routines
	wait sync.WaitGroup

	Query  *DnsQuery
	Result *DnsResult
}

// map from queries to jobs
var queryMap = make(map[DnsQuery]*DnsJob)

// mutex to the map
var queryMutex = sync.Mutex{}

// waiting jobs
var queryChan = make(chan *DnsJob, 10)

// context for Unbound
var unboundCtx = unbound.New()

// Creates a new query
func NewDnsJob(domain string, typ dns.Type) *DnsJob {
	var query = DnsQuery{Domain: domain, Type: typ}
	var job *DnsJob
	var exist bool

	queryMutex.Lock()

	// Is the same query already running?
	if job, exist = queryMap[query]; !exist {
		job = &DnsJob{}
		job.Query = &query
		job.wait.Add(1)
		log.Printf("job created %p", job)
		queryMap[query] = job
	}
	queryMutex.Unlock()

	if !exist {
		queryChan <- job
	}

	return job
}

// Wait until the query is finished
func (job *DnsJob) Wait() DnsJob {
	log.Printf("waiting for %p", job)
	job.wait.Wait()
	return *job
}

// Saves the result and wakes up the waiting routines
func (job *DnsJob) run() {
	result := job.Query.Lookup()
	job.Result = &result

	// clean up the map
	queryMutex.Lock()
	delete(queryMap, *job.Query)
	queryMutex.Unlock()

	// wake up the waiting routines
	log.Printf("job finished: %p", job)
	job.wait.Done()
}

// Wait until the query is finished
func (job *DnsJob) Results() []string {
	return job.Result.Results
}

// Appends a new entry to the result
func (result *DnsResult) Append(entry string) {
	result.Results = append(result.Results, entry)
}

// Does the lookup
func (query *DnsQuery) Lookup() (result DnsResult) {

	// execute the query
	response, err := unboundCtx.Resolve(query.Domain, uint16(query.Type), uint16(dns.ClassINET))

	result.Secure = response.Secure

	if response.WhyBogus != "" {
		result.WhyBogus = &response.WhyBogus
	}

	// error or NXDomain rcode?
	if err != nil || response.NxDomain {
		result.Error = &err
		return
	}

	// Other erroneous rcode?
	if response.Rcode != dns.RcodeSuccess {
		err = errors.New(dns.RcodeToString[response.Rcode])
		result.Error = &err
		return
	}

	for i, _ := range response.Data {
		switch record := response.Rr[i].(type) {
		case *dns.MX:
			result.Append(record.Mx)
		case *dns.A:
			result.Append(record.A.String())
		case *dns.AAAA:
			result.Append(record.AAAA.String())
		case *dns.TLSA:
			result.Append(strconv.Itoa(int(record.Usage)) +
				" " + strconv.Itoa(int(record.Selector)) +
				" " + strconv.Itoa(int(record.MatchingType)) +
				" " + record.Certificate)
		}
	}

	return result
}
