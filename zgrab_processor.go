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

	// Is the job currently enqueued or beeing processed?
	pending bool

	Address net.IP
	Result  *MxHostSummary

	// Cache attributes
	refreshed time.Time
	accessed  time.Time
}

type ZgrabProcessor struct {
	// Cache for enqueued, running and finished jobs
	cache         map[string]*ZgrabJob
	cacheChannel  chan bool
	expireAfter   time.Duration
	refreshAfter  time.Duration
	checkInterval time.Duration

	// Statistics
	cacheHits      uint64
	cacheMisses    uint64
	cacheRefreshes uint64
	cacheExpiries  uint64

	// mutex for the cache
	mutex sync.Mutex

	workers *WorkerPool
}

func NewZgrabProcessor(workersCount uint, expireAfter uint, refreshAfter uint, checkInterval uint) *ZgrabProcessor {
	proc := &ZgrabProcessor{}
	proc.workers = NewWorkerPool(workersCount, proc.work)
	proc.cache = make(map[string]*ZgrabJob)

	if expireAfter > 0 {
		proc.expireAfter = time.Duration(expireAfter) * time.Second
		proc.refreshAfter = time.Duration(refreshAfter) * time.Second
		proc.checkInterval = time.Duration(checkInterval) * time.Second

		// enable cache
		proc.cacheChannel = make(chan bool, 1)
		proc.cacheChannel <- true // start it
		log.Println("Cache entries will be refreshed after", refreshAfter, "seconds and removed after", expireAfter, "seconds")

		// Start the cache worker
		go proc.cacheWorker()
	} else {
		log.Println("Host cache disabled")
	}

	return proc
}

// If a finished job is already in the cache, it is returned
// If a job is already pending, it is returned.
// Otherwise a new job is created and returned.
func (proc *ZgrabProcessor) NewJob(address net.IP) (job *ZgrabJob) {
	key := string(address)
	exist := false

	proc.mutex.Lock()

	// Does the address exist in the cache?
	if job, exist = proc.cache[key]; exist {
		proc.cacheHits += 1
	} else {
		proc.cacheMisses += 1
		// Add to active jobs map
		job = &ZgrabJob{Address: address, pending: true}
		job.wait.Add(1)
		proc.cache[key] = job
	}

	proc.mutex.Unlock()

	// Update access time
	job.accessed = time.Now()

	if !exist {
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

// Wait until the job is finished
func (job *ZgrabJob) Wait() {
	job.wait.Wait()
}

// The worker function for a job
func (proc *ZgrabProcessor) work(item interface{}) {
	job, ok := item.(*ZgrabJob)
	if !ok {
		log.Panic("unexpected item:", job)
	}

	// Do the banner grab
	job.Result = NewMxHostSummary(job.Address)

	// Lock
	proc.mutex.Lock()

	// Expire the entry immediately?
	if proc.expireAfter == 0 {
		delete(proc.cache, string(job.Address))
	}

	// Unlock
	proc.mutex.Unlock()

	// Mark as finished and wake up waiting routines
	job.refreshed = time.Now()
	job.pending = false
	job.wait.Done()

	// Enqueue the result to store it in the database
	if resultProcessor != nil {
		resultProcessor.Add(job.Result)
		if certs := job.Result.certificates; certs != nil {
			resultProcessor.Add(certs)
		}
	}
}

// Periodically checks the cache and expires oder enqueues entries.
func (proc *ZgrabProcessor) cacheWorker() {
	for range proc.cacheChannel {
		enqueue := make([]*ZgrabJob, 0)

		proc.mutex.Lock()
		for key, job := range proc.cache {
			if !job.pending {
				if time.Since(job.accessed) > proc.expireAfter {
					// expire the job
					delete(proc.cache, key)
					proc.cacheExpiries++
				} else if time.Since(job.refreshed) > proc.refreshAfter {
					// enqueue the job
					job.pending = true
					job.wait.Add(1)
					enqueue = append(enqueue, job)
				}
			}
		}
		proc.mutex.Unlock()

		// Update refreshes counter
		proc.cacheRefreshes += uint64(len(enqueue))

		// Enqueue new jobs
		// this is a blocking operation and must not be in a locked section
		for _, job := range enqueue {
			proc.workers.Add(job)
		}

		// sleep and initiate the next run
		time.Sleep(proc.checkInterval)
		proc.cacheChannel <- true
	}
}
