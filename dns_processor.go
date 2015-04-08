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

type DnsJobs struct {
	jobs []*DnsJob
}

type DnsProcessor struct {
	// map from queries to jobs
	jobs map[DnsQuery]*DnsJob

	// mutex to the map
	mutex sync.Mutex

	// waiting jobs
	channel chan *DnsJob

	// context for Unbound
	unboundCtx *unbound.Unbound
}

func NewDnsProcessor(workersCount uint) (proc DnsProcessor) {
	proc.unboundCtx = unbound.New()
	proc.channel = make(chan *DnsJob, 100)
	proc.jobs = make(map[DnsQuery]*DnsJob)

	// Start all workers
	for i := uint(0); i < workersCount; i++ {
		go proc.worker()
	}
	return
}

// A worker
func (proc *DnsProcessor) worker() {
	for job := range proc.channel {
		result := proc.Lookup(job.Query)
		job.Result = &result

		// clean up the map
		proc.mutex.Lock()
		delete(proc.jobs, *job.Query)
		proc.mutex.Unlock()

		// wake up the waiting routines
		log.Printf("job finished: %p", job)
		job.wait.Done()
	}
}

// Creates a new job
func (proc *DnsProcessor) NewJob(domain string, typ dns.Type) *DnsJob {

	var query = DnsQuery{Domain: domain, Type: typ}
	var job *DnsJob
	var exist bool

	proc.mutex.Lock()

	// Is the same query already running?
	if job, exist = proc.jobs[query]; !exist {
		job = &DnsJob{}
		job.Query = &query
		job.wait.Add(1)
		log.Printf("job created %p", job)
		proc.jobs[query] = job
	}
	proc.mutex.Unlock()

	if !exist {
		proc.channel <- job
	}

	return job
}

// Creates a group of jobs
func (proc *DnsProcessor) NewJobs(domains []string, types []dns.Type) (group DnsJobs) {
	for _, domain := range domains {
		for _, typ := range types {
			job := proc.NewJob(domain, typ)
			log.Printf("created: %p", job)
			group.append(job)
		}
	}
	return
}

// Closes the internal channel
// New Jobs will not be accepted any more
func (proc *DnsProcessor) Close() {
	close(proc.channel)
}

// Waits until the query is finished
func (job *DnsJob) Wait() {
	log.Printf("waiting for %p", job)
	job.wait.Wait()
}

// Waits until all queries in this group are finished
func (group *DnsJobs) Wait() {
	for _, job := range group.jobs {
		log.Println("group wait", job.Query)
		job.wait.Wait()
		log.Println("group done")
	}
}

// Appends a new entry to the result
func (result *DnsResult) append(entry string) {
	result.Results = append(result.Results, entry)
}

// Appends a new entry to the result
func (group *DnsJobs) append(job *DnsJob) {
	group.jobs = append(group.jobs, job)
}

// Returns the results
func (job *DnsJob) Results() []string {
	job.Wait()
	return job.Result.Results
}

// Returns the flattened results of all jobs
func (group *DnsJobs) Results() []string {
	results := make([]string, 0)

	for _, job := range group.jobs {
		for _, item := range job.Results() {
			results = append(results, item)
		}
	}
	return results
}

// Does the lookup
func (proc *DnsProcessor) Lookup(query *DnsQuery) (result DnsResult) {

	// execute the query
	response, err := proc.unboundCtx.Resolve(query.Domain, uint16(query.Type), uint16(dns.ClassINET))

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
			result.append(record.Mx)
		case *dns.A:
			result.append(record.A.String())
		case *dns.AAAA:
			result.append(record.AAAA.String())
		case *dns.TLSA:
			result.append(strconv.Itoa(int(record.Usage)) +
				" " + strconv.Itoa(int(record.Selector)) +
				" " + strconv.Itoa(int(record.MatchingType)) +
				" " + record.Certificate)
		}
	}

	return result
}
