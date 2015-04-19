package main

import (
	"github.com/hashicorp/golang-lru"
	"log"
	"net"
	"sync"
	"time"
)

const (
	// Maximum age for a zgrab result
	zgrabTTL = time.Duration(3600) * time.Second

	// Cache size for zgrab results
	zgrabCacheSize = 1024 * 10
)

type ZgrabJob struct {
	// waitGroup for the waiting routines
	wait sync.WaitGroup

	Address net.IP
	Result  *MxHost
}

type ZgrabProcessor struct {
	// map for active (pending/running) jobs
	jobs map[string]*ZgrabJob

	// LRU cache to reduce database load
	cache *lru.Cache

	cacheHits      uint64
	cacheMisses    uint64
	cacheExpiries  uint64
	concurrentHits uint64

	// mutex for the map
	mutex sync.Mutex

	workers *WorkerPool
}

func NewZgrabProcessor(workersCount uint) *ZgrabProcessor {
	var err error
	proc := &ZgrabProcessor{}

	work := func(item interface{}) {
		job, ok := item.(*ZgrabJob)
		if !ok {
			log.Panic("unexpected object:", job)
		}

		// IP addresses (byte arrays) can not be used directly
		key := string(job.Address)

		// Do the banner grab
		result := NewMxHost(job.Address)
		job.Result = &result

		// Lock
		proc.mutex.Lock()

		// Add to cache
		proc.cache.Add(key, job)

		// Remove from active jobs map
		delete(proc.jobs, key)

		// Unlock
		proc.mutex.Unlock()

		// Wake up waiting routines
		job.wait.Done()

		// Enqueue the result saving to the database
		resultProcessor.Add(job.Result)
	}
	proc.workers = NewWorkerPool(workersCount, work)
	proc.jobs = make(map[string]*ZgrabJob)

	// Initialize cache
	if proc.cache, err = lru.New(zgrabCacheSize); err != nil {
		panic(err)
	}

	return proc
}

// Creates a new job
func (proc *ZgrabProcessor) NewJob(address net.IP) (job *ZgrabJob) {
	key := string(address)
	exist := false

	proc.mutex.Lock()

	// Does the address exist in the cache?
	if obj, ok := proc.cache.Get(key); ok {
		job, _ = obj.(*ZgrabJob)

		if time.Since(*job.Result.UpdatedAt) <= zgrabTTL {
			// nothing to do
			proc.cacheHits += 1
			exist = true
		} else {
			// Cache is outdated
			proc.cache.Remove(key)
			proc.cacheExpiries += 1
		}
	} else {
		proc.cacheMisses += 1
	}

	if !exist {
		// Is there already an active job with the same address?
		if job, exist = proc.jobs[key]; exist {
			proc.concurrentHits += 1
		} else {
			// Add to active jobs map
			job = &ZgrabJob{Address: address}
			proc.jobs[key] = job
		}
	}

	proc.mutex.Unlock()

	if !exist {
		job.wait.Add(1)
		proc.workers.Add(job)
	}

	return job
}

// Stops accepting new jobs and waits until all jobs are finished
func (proc *ZgrabProcessor) Close() {
	proc.workers.Close()
}

func (job *ZgrabJob) Wait() {
	job.wait.Wait()
}
