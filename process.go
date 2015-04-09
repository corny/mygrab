package main

import (
	"github.com/miekg/dns"
	"github.com/zmap/zgrab/zlib"
	"io"
	"log"
	"net"
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

func Process(domain string) {
	// Do the MX lookups
	mxJob := dnsProcessor.NewJob(domain, TypeMX)

	// Make mx hostnames unique
	mxHosts := UniqueStrings(mxJob.Results())

	saveDomain(domain, mxJob.Result)

	// Do the A/AAAA lookups
	mxAddresses := dnsProcessor.NewJobs(mxHosts, addressTypes)

	saveMxRecords(mxHosts, mxAddresses.jobs)

	// Make addresses unique
	addresses := UniqueStrings(mxAddresses.Results())

	// Do the bannergrabs
	for _, addr := range addresses {
		log.Println("grabbing", addr)
		target := zlib.GrabTarget{Addr: net.ParseIP(addr)}
		result := NewHostResult(target)
		saveHostResult(result)
	}
}

func ProcessWithDecoder(in Decoder, config zlib.Config) {
	workers := config.Senders
	processQueue := make(chan zlib.GrabTarget, workers*4)
	outputQueue := make(chan HostResult, workers*4)

	w := zlib.NewGrabWorker(&config)

	// Create wait groups
	var workerDone sync.WaitGroup
	var outputDone sync.WaitGroup
	workerDone.Add(int(workers))
	outputDone.Add(1)

	// Start the output handler
	go func() {
		for result := range outputQueue {
			saveHostResult(result)
		}
		outputDone.Done()
	}()

	// Start all the workers
	for i := uint(0); i < workers; i++ {
		go func() {
			for obj := range processQueue {
				outputQueue <- NewHostResult(obj)
			}
			workerDone.Done()
		}()
	}

	// Read the input, send to workers
	for {
		obj, err := in.DecodeNext()
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}

		target, ok := obj.(zlib.GrabTarget)
		if !ok {
			panic("unable to cast")
		}
		processQueue <- target
	}
	close(processQueue)
	workerDone.Wait()
	close(outputQueue)
	outputDone.Wait()
	w.Done()
}
