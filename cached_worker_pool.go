package main

import (
	"sync"
	"time"
)

type CacheConfig struct {
	ExpireAfter   time.Duration
	RefreshAfter  time.Duration
	CheckInterval time.Duration
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
	CacheHits      uint64 `json:"hits"`
	CacheMisses    uint64 `json:"misses"`
	CacheRefreshes uint64 `json:"refreshes"`
	CacheExpiries  uint64 `json:"expiries"`

	CacheWorkerStarted time.Time `json:"worker_started"`
	CacheWorkerStopped time.Time `json:"worker_stopped"`

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
		ExpireAfter:   time.Duration(expireAfter) * time.Second,
		RefreshAfter:  time.Duration(refreshAfter) * time.Second,
		CheckInterval: time.Duration(checkInterval) * time.Second,
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
func (proc *CachedWorkerPool) NewJob(key string, accessed time.Time) (entry *CacheEntry) {
	exist := false

	proc.mutex.Lock()

	// Does the address exist in the cache?
	if entry, exist = proc.cache[key]; exist {
		proc.CacheHits += 1
	} else {
		proc.CacheMisses += 1
		// Add to cache
		entry = &CacheEntry{
			Key:     key,
			Pending: true,
			Created: accessed,
		}
		entry.wait.Add(1)
		proc.cache[key] = entry
	}

	proc.mutex.Unlock()

	// Update access attributes
	// This is outside of the critical section,
	// but race conditions do not cause any trouble.
	entry.Hits += 1
	if entry.Accessed.Before(accessed) {
		entry.Accessed = accessed
	}

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
	return time.Since(accessed) > config.ExpireAfter
}

func (config *CacheConfig) shouldRefresh(refreshed time.Time) bool {
	return config.RefreshAfter > 0 && time.Since(refreshed) > config.RefreshAfter
}

// Periodically checks the cache and expires oder enqueues entries.
func (proc *CachedWorkerPool) cacheWorker() {
	for range proc.cacheChannel {
		enqueue := make([]*CacheEntry, 0)

		proc.CacheWorkerStarted = time.Now()
		proc.mutex.Lock()
		for key, entry := range proc.cache {
			if !entry.Pending {
				if proc.cacheConfig.shouldExpire(entry.Accessed) {
					// expire the entry
					delete(proc.cache, key)
					proc.CacheExpiries++
				} else if proc.cacheConfig.shouldRefresh(entry.Refreshed) {
					// enqueue the entry
					entry.Pending = true
					entry.wait.Add(1)
					enqueue = append(enqueue, entry)
				}
			}
		}
		proc.mutex.Unlock()
		proc.CacheWorkerStopped = time.Now()

		// Update refreshes counter
		proc.CacheRefreshes += uint64(len(enqueue))

		// Enqueue new entrys
		// this is a blocking operation and must not be in a locked section
		for _, entry := range enqueue {
			proc.workers.Add(entry)
		}

		// sleep and initiate the next run
		time.Sleep(proc.cacheConfig.CheckInterval)
		proc.cacheChannel <- true
	}
}
