package main

import (
	"github.com/miekg/dns"
	"github.com/zmap/zgrab/zlib"
	"io"
	"log"
	"sync"
)

const (
	TypeMX   = dns.Type(dns.TypeMX)
	TypeA    = dns.Type(dns.TypeA)
	TypeAAAA = dns.Type(dns.TypeAAAA)
)

var addressTypes []dns.Type

func init() {
	addressTypes = []dns.Type{TypeA, dns.Type(TypeAAAA)}
}

func Resolve(domain string) {
	// Do the MX lookups
	mxJob := NewDnsJob(domain, TypeMX)
	mxJob.Wait()

	// Make mx hostnames unique
	mxHosts := UniqueStrings(mxJob.Results())

	// Do the A/AAAA lookups
	mxAddresses := NewDnsJobs(mxHosts, addressTypes)
	mxAddresses.Wait()

	// Make addresses unique
	addresses := UniqueStrings(mxAddresses.Results())

	log.Println(addresses)
	// TODO dnsgrab
}

func Process(in Decoder, config zlib.Config) {
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
