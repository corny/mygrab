package main

import (
	"github.com/hashicorp/golang-lru"
	"log"
	"sync"
	"time"
)

const (
	// Maximum age for a zgrab result
	zgrabTTL = time.Duration(3600) * time.Second

	// Cache size for zgrab results
	zgrabCacheSize = 1024 * 10
)

type ZgrabProcessor struct {
	// map for active (pending/running) jobs
	jobs map[string]bool

	// LRU cache to reduce database load
	cache *lru.Cache

	cacheHits      uint
	cacheMisses    uint
	cacheExpiries  uint
	concurrentHits uint

	// mutex for the map
	mutex sync.Mutex

	workers *WorkerPool
}

func NewZgrabProcessor(workersCount uint) *ZgrabProcessor {
	var err error
	proc := &ZgrabProcessor{}

	work := func(item interface{}) {
		address, ok := item.(*string)
		if !ok {
			log.Panic("invalid cast")
		}

		result := NewMxHost(*address)

		// Lock
		proc.mutex.Lock()

		// Add to cache
		proc.cache.Add(*address, &result)

		// Remove from active jobs map
		delete(proc.jobs, *address)

		// Unlock
		proc.mutex.Unlock()

		resultProcessor.Add(&result)
	}
	proc.workers = NewWorkerPool(workersCount, work)
	proc.jobs = make(map[string]bool)

	// Initialize cache
	if proc.cache, err = lru.New(zgrabCacheSize); err != nil {
		panic(err)
	}

	return proc
}

// Creates a new job
func (proc *ZgrabProcessor) NewJob(address string) {
	exist := false

	proc.mutex.Lock()

	// Does the address exist in the cache?
	if obj, ok := proc.cache.Get(address); ok {
		host, _ := obj.(*MxHost)

		if time.Since(*host.UpdatedAt) > zgrabTTL {
			// Cache is outdated
			proc.cache.Remove(address)
			proc.cacheExpiries += 1
			log.Println(address, "in cache and outdated")
		} else {
			// nothing to do
			exist = true
			proc.cacheHits += 1
			log.Println(address, "in cache and valid")
		}
	} else {
		log.Println(address, "not in cache")
		proc.cacheMisses += 1
	}

	if !exist {
		// Is there already an active job with the same address?
		if _, exist = proc.jobs[address]; exist {
			proc.concurrentHits += 1
			log.Println(address, "EXISTS in map")
		} else {
			log.Println(address, "not exists in map")
			// Add to active jobs map
			proc.jobs[address] = true
		}
	}

	proc.mutex.Unlock()

	if !exist {
		proc.workers.Add(&address)
	}
}

// Stops accepting new jobs and waits until all jobs are finished
func (proc *ZgrabProcessor) Close() {
	proc.workers.Close()
}
