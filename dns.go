package main

import (
	"github.com/miekg/dns"
	"github.com/miekg/unbound"
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
	wait WaitGroup

	Query  *DnsQuery
	Result *DnsResult
}

// map from queries to jobs
var queryMap = make(map[DnsQuery]DnsJob)

// mutex to the map
var queryMutex = sync.Mutex{}

// waiting jobs
var queryChan = make(chan DnsJob)

// context for Unbound
var unboundCtx = unbound.New()

// Creates a new query
func NewDnsQuery(query *DnsQuery) DnsJob {
	queryMutex.Lock()

	// Is the same query already running?
	if job, ok := queryMap[query]; !ok {
		job := DnsJob{}
		job.Query = query
		job.wait.Add()

		queries[query] = job
		queryChan <- job
	}
	queryMutex.Unlock()

	return job
}

// Wait until the query is finished
func (job DnsJob) Wait() {
	job.Wait()
}

// Saves the result and wakes up the waiting routines
func (job DnsJob) Finished(result *DnsResult) {

	// clean up the map
	queryMutex.Lock()
	delete(queryMap, job.Query)
	queryMutex.Unlock()

	// set the result and wake up the waiting routines
	job.Result = result
	job.wait.Done()
}

// Appends a new entry to the result
func (result *DnsResult) Append(entry string) {
	result.Results = append(result.Results, entry)
}

// Does the lookup
func lookup(job *DnsJob) (result DnsResult) {
	// execute the query
	response, err := unboundCtx.Resolve(job.Query.Domain, job.Query.Type, dns.ClassINET)

	result.Secure = response.Secure
	result.WhyBogus = response.WhyBogus

	// error or NXDomain rcode?
	if err != nil || response.NxDomain {
		result.Error = err
		return
	}

	// Other erroneous rcode?
	if response.Rcode != dns.RcodeSuccess {
		result.Error = errors.New(dns.RcodeToString[response.Rcode])
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

	return
}
