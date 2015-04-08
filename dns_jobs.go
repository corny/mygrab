package main

import (
	"github.com/miekg/dns"
	"log"
)

type JobGroup struct {
	jobs []*DnsJob
}

func NewDnsJobs(domains []string, types []dns.Type) (group JobGroup) {

	for _, domain := range domains {
		for _, typ := range types {
			job := NewDnsJob(domain, typ)
			log.Printf("created: %p", job)
			group.Append(job)
		}
	}

	return
}

func (group *JobGroup) Append(job *DnsJob) {
	group.jobs = append(group.jobs, job)
}

func (group *JobGroup) Wait() {
	for _, job := range group.jobs {
		log.Println("group wait", job.Query)
		job.wait.Wait()
		log.Println("group done")
	}
}

func (group *JobGroup) Results() []string {
	results := make([]string, 0)

	for _, job := range group.jobs {
		for _, item := range job.Results() {
			results = append(results, item)
		}
	}
	return results
}
