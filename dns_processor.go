package main

import (
	"errors"
	"github.com/miekg/dns"
	"github.com/miekg/unbound"
	"log"
	"strconv"
	"strings"
	"sync"
)

type DnsQuery struct {
	Domain string
	Type   dns.Type
}

type DnsResult struct {
	// The result
	Results  []string
	Secure   bool
	Error    error
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
	// maps pending/running queries to jobs
	jobs map[DnsQuery]*DnsJob

	// mutex for the map
	mutex sync.Mutex

	// waiting jobs
	channel chan *DnsJob

	// wait group for the workers
	wait sync.WaitGroup

	// context for Unbound
	unboundCtx *unbound.Unbound
}

func NewDnsProcessor(workersCount uint) *DnsProcessor {
	proc := &DnsProcessor{}
	proc.unboundCtx = unbound.New()
	proc.channel = make(chan *DnsJob, 100)
	proc.jobs = make(map[DnsQuery]*DnsJob)
	proc.wait.Add(int(workersCount))

	// Start all workers
	for i := uint(0); i < workersCount; i++ {
		go proc.worker()
	}

	return proc
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
		log.Printf("DNS job finished: %p", job)
		job.wait.Done()
	}
	proc.wait.Done()
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
		log.Printf("DNS job created %p", job)
		proc.jobs[query] = job
	}
	proc.mutex.Unlock()

	if !exist {
		proc.channel <- job
	}

	return job
}

// Creates a group of jobs
func (proc *DnsProcessor) NewJobs(domains []string, types []dns.Type) *DnsJobs {
	group := &DnsJobs{}
	for _, domain := range domains {
		for _, typ := range types {
			job := proc.NewJob(domain, typ)
			log.Printf("DNS created: %p", job)
			group.append(job)
		}
	}
	return group
}

// Closes the internal channel and waits until all workers are done
func (proc *DnsProcessor) Close() {
	close(proc.channel)
	proc.wait.Wait()
}

// Waits until the query is finished
func (job *DnsJob) Wait() {
	log.Printf("DNS waiting for %p", job)
	job.wait.Wait()
}

// Waits until all queries in this group are finished
func (group *DnsJobs) Wait() {
	for _, job := range group.jobs {
		log.Println("DNS group wait", job.Query)
		job.wait.Wait()
		log.Println("DNS group done")
	}
}

// Appends a new entry to the result
func (result *DnsResult) append(entry string) {
	result.Results = append(result.Results, entry)
}

// The error string or nil
func (result *DnsResult) ErrorMessage() *string {
	if result.Error == nil {
		return nil
	}
	str := result.Error.Error()
	return &str
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
		result.Error = err
		return
	}

	// Other erroneous rcode?
	if response.Rcode != dns.RcodeSuccess {
		err = errors.New(dns.RcodeToString[response.Rcode])
		result.Error = err
		return
	}

	for i, _ := range response.Data {
		switch record := response.Rr[i].(type) {
		case *dns.MX:
			result.append(strings.TrimSuffix(record.Mx, "."))
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
