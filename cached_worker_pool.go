package main

import (
	"sync"
	"time"
)

type CacheConfig struct {
	expireAfter   time.Duration
	refreshAfter  time.Duration
	checkInterval time.Duration
}

type CacheEntry struct {
	// waitGroup for the waiting routines
	wait sync.WaitGroup

	Key   string      `json:"-"`
	Value interface{} `json:"value"`

	// Is the entry currently enqueued or beeing processed?
	Pending bool `json:"pending"`

	// Cache attributes
	Hits      uint64    `json:"hits"`
	Created   time.Time `json:"created"`
	Refreshed time.Time `json:"refreshed"`
	Accessed  time.Time `json:"accessed"`
}

type CachedWorkerPool struct {
	// Cache for enqueued, running and finished entries
	cache        map[string]*CacheEntry
	cacheChannel chan bool
	cacheConfig  *CacheConfig

	// Statistics
	cacheHits      uint64
	cacheMisses    uint64
	cacheRefreshes uint64
	cacheExpiries  uint64

	// mutex for the cache
	mutex sync.Mutex

	// Worker Pool
	workers    *WorkerPool
	workerFunc WorkerFunc
}

func NewCacheConfig(expireAfter uint, refreshAfter uint, checkInterval uint) *CacheConfig {
	if expireAfter == 0 {
		panic("expireAfter must not be zero")
	}
	return &CacheConfig{
		expireAfter:   time.Duration(expireAfter) * time.Second,
		refreshAfter:  time.Duration(refreshAfter) * time.Second,
		checkInterval: time.Duration(checkInterval) * time.Second,
	}
}

func NewCachedWorkerPool(workersCount uint, workerFunc WorkerFunc, cacheConfig *CacheConfig) *CachedWorkerPool {
	proc := &CachedWorkerPool{
		cacheConfig: cacheConfig,
		workerFunc:  workerFunc,
		cache:       make(map[string]*CacheEntry),
	}
	proc.workers = NewWorkerPool(workersCount, proc.work)

	if cacheConfig != nil {
		// enable cache
		proc.cacheChannel = make(chan bool, 1)
		proc.cacheChannel <- true // start it

		// Start the cache worker
		go proc.cacheWorker()
	}

	return proc
}

// If a finished entry is already in the cache, it is returned
// If a entry is already pending, it is just returned.
// Otherwise a new entry is created and returned.
func (proc *CachedWorkerPool) NewJob(key string) (entry *CacheEntry) {
	exist := false
	now := time.Now()

	proc.mutex.Lock()

	// Does the address exist in the cache?
	if entry, exist = proc.cache[key]; exist {
		proc.cacheHits += 1
	} else {
		proc.cacheMisses += 1
		// Add to cache
		entry = &CacheEntry{
			Key:     key,
			Pending: true,
			Created: now,
		}
		entry.wait.Add(1)
		proc.cache[key] = entry
	}

	proc.mutex.Unlock()

	// Update access attributes
	// This is outside of the critical section,
	// but race conditions do not cause any trouble.
	entry.Accessed = now
	entry.Hits += 1

	if !exist {
		proc.workers.Add(entry)
	}

	return entry
}

// Stops accepting new entrys and waits until all entrys are finished
func (proc *CachedWorkerPool) Close() {
	if proc.cacheChannel != nil {
		close(proc.cacheChannel)
	}
	proc.workers.Close()
}

// Wait until the entry is finished
func (entry *CacheEntry) Wait() {
	entry.wait.Wait()
}

// The worker function for a CacheEntry
func (proc *CachedWorkerPool) work(item interface{}) {
	entry, _ := item.(*CacheEntry)

	// Call the worker function and save the return value
	proc.workerFunc(entry)

	// Lock
	proc.mutex.Lock()

	// Expire the entry immediately?
	if proc.cacheConfig == nil {
		delete(proc.cache, entry.Key)
	}

	// Unlock
	proc.mutex.Unlock()

	// Mark as finished and wake up waiting routines
	entry.Refreshed = time.Now()
	entry.Pending = false
	entry.wait.Done()
}

func (config *CacheConfig) shouldExpire(accessed time.Time) bool {
	return time.Since(accessed) > config.expireAfter
}

func (config *CacheConfig) shouldRefresh(refreshed time.Time) bool {
	return config.refreshAfter > 0 && time.Since(refreshed) > config.refreshAfter
}

// Periodically checks the cache and expires oder enqueues entries.
func (proc *CachedWorkerPool) cacheWorker() {
	for range proc.cacheChannel {
		enqueue := make([]*CacheEntry, 0)

		proc.mutex.Lock()
		for key, entry := range proc.cache {
			if !entry.Pending {
				if proc.cacheConfig.shouldExpire(entry.Accessed) {
					// expire the entry
					delete(proc.cache, key)
					proc.cacheExpiries++
				} else if proc.cacheConfig.shouldRefresh(entry.Refreshed) {
					// enqueue the entry
					entry.Pending = true
					entry.wait.Add(1)
					enqueue = append(enqueue, entry)
				}
			}
		}
		proc.mutex.Unlock()

		// Update refreshes counter
		proc.cacheRefreshes += uint64(len(enqueue))

		// Enqueue new entrys
		// this is a blocking operation and must not be in a locked section
		for _, entry := range enqueue {
			proc.workers.Add(entry)
		}

		// sleep and initiate the next run
		time.Sleep(proc.cacheConfig.checkInterval)
		proc.cacheChannel <- true
	}
}
