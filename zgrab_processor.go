package main

import (
	"log"
	"net"
	"sync"
	"time"
)

type ZgrabJob struct {
	// waitGroup for the waiting routines
	wait sync.WaitGroup

	Address net.IP
	Result  *MxHostSummary
}

type ZgrabCacheEntry struct {
	refreshed time.Time
	accessed  time.Time
	job       *ZgrabJob
}

type ZgrabProcessor struct {
	// map for active (pending/running) jobs
	jobs map[string]*ZgrabJob

	// Cache
	cache         map[string]*ZgrabCacheEntry
	cacheChannel  chan interface{}
	expireAfter   time.Duration
	refreshAfter  time.Duration
	checkInterval time.Duration

	// Statistics
	cacheHits      uint64
	cacheMisses    uint64
	cacheRefreshes uint64
	cacheExpiries  uint64
	concurrentHits uint64

	// mutex for jobs and cache
	mutex sync.Mutex

	workers *WorkerPool
}

func NewZgrabProcessor(workersCount uint, expireAfter uint, refreshAfter uint, checkInterval uint) *ZgrabProcessor {
	proc := &ZgrabProcessor{}
	proc.workers = NewWorkerPool(workersCount, proc.work)
	proc.jobs = make(map[string]*ZgrabJob)

	if expireAfter > 0 {
		proc.expireAfter = time.Duration(expireAfter) * time.Second
		proc.refreshAfter = time.Duration(refreshAfter) * time.Second
		proc.checkInterval = time.Duration(checkInterval) * time.Second

		// enable cache
		proc.cacheChannel = make(chan interface{}, 1)
		proc.cacheChannel <- true // start it
		proc.cache = make(map[string]*ZgrabCacheEntry)
		log.Println("Cache entries will be refreshed after", refreshAfter, "seconds and removed after", expireAfter, "seconds")

		// Start the cache worker
		go proc.cacheWorker()
	} else {
		log.Println("Host cache disabled")
	}

	return proc
}

// Creates a new job
func (proc *ZgrabProcessor) NewJob(address net.IP) (job *ZgrabJob) {
	key := string(address)
	exist := false

	proc.mutex.Lock()

	// Does the address exist in the cache?
	if obj, ok := proc.cache[key]; ok {
		job = obj.job
		exist = true
		proc.cacheHits += 1
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
	if proc.cacheChannel != nil {
		close(proc.cacheChannel)
	}
	proc.workers.Close()
}

func (job *ZgrabJob) Wait() {
	job.wait.Wait()
}

func (proc *ZgrabProcessor) work(item interface{}) {
	job, ok := item.(*ZgrabJob)
	if !ok {
		log.Panic("unexpected item:", job)
	}

	// IP addresses (byte arrays) can not be used directly
	key := string(job.Address)

	// Do the banner grab
	job.Result = NewMxHostSummary(job.Address)

	// Lock
	proc.mutex.Lock()

	// Update cache
	if proc.cache != nil {
		now := time.Now()
		if value, ok := proc.cache[key]; ok {
			// Existing entry has been refreshed
			value.refreshed = now
			value.job = job
		} else {
			// Add to cache
			proc.cache[key] = &ZgrabCacheEntry{
				job:       job,
				refreshed: now,
				accessed:  now,
			}
		}
	}

	// Remove from active jobs map
	delete(proc.jobs, key)

	// Unlock
	proc.mutex.Unlock()

	// Wake up waiting routines
	job.wait.Done()

	// Enqueue the result to store it in the database
	if resultProcessor != nil {
		resultProcessor.Add(job.Result)
		if certs := job.Result.certificates; certs != nil {
			resultProcessor.Add(certs)
		}
	}
}

func (proc *ZgrabProcessor) cacheWorker() {
	for range proc.cacheChannel {
		enqueue := make([]*ZgrabJob, 0)

		proc.mutex.Lock()
		for key, entry := range proc.cache {
			if time.Since(entry.accessed) > proc.expireAfter {
				// expired
				delete(proc.cache, key)
				proc.cacheExpiries++
			} else if time.Since(entry.refreshed) > proc.refreshAfter {
				// enqueue if the job is not already pending
				if _, ok := proc.jobs[key]; !ok {
					proc.jobs[key] = entry.job
					proc.cacheRefreshes++
					entry.job.wait.Add(1)
					enqueue = append(enqueue, entry.job)
				}
			}
		}
		proc.mutex.Unlock()

		// Enqueue new jobs
		// this is a blocking operation
		for job := range enqueue {
			proc.workers.Add(job)
		}

		// sleep
		time.Sleep(proc.checkInterval)
		proc.cacheChannel <- true
	}
}
