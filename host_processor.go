package main

import (
	"net"
)

type HostProcessor struct {
	cache *CachedWorkerPool
}

func NewHostProcessor(workersCount uint, cacheConfig *CacheConfig) *HostProcessor {
	workerFunc := func(obj interface{}) {
		entry, _ := obj.(*CacheEntry)

		// Run the host check
		hostSummary := NewMxHostSummary(net.IP(entry.Key))
		entry.Value = hostSummary

		// Enqueue the result to store it in the database
		if resultProcessor != nil {
			resultProcessor.Add(entry.Value)
			if certs := hostSummary.certificates; certs != nil {
				resultProcessor.Add(certs)
			}
		}
	}

	proc := &HostProcessor{
		cache: NewCachedWorkerPool(workersCount, workerFunc, cacheConfig),
	}

	return proc
}

func (proc *HostProcessor) NewJob(addr net.IP) *CacheEntry {
	return proc.cache.NewJob(string(addr))
}

// Stops accepting new jobs and waits until all jobs are finished
func (proc *HostProcessor) Close() {
	proc.cache.Close()
}
