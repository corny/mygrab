package main

import (
	"sync"
	"time"
)

const (
	maxStatsCount = 60
)

// The worker function receives any object and returns nothing
type WorkerFunc func(interface{})

type WorkerPool struct {
	// channel for pending jobs
	channel chan interface{}

	// Number of processed jobs
	processed uint64

	// Recently processed jobs per second
	statsValues []int
	statsTicker *time.Ticker

	// WaitGroup to wait until all workers are done
	wg sync.WaitGroup

	// The function that does the work
	work WorkerFunc

	// Size of the worker pool
	maxWorkers     uint
	currentWorkers uint
}

func NewWorkerPool(maxWorkers uint, work WorkerFunc) *WorkerPool {
	proc := &WorkerPool{work: work}
	proc.channel = make(chan interface{}, 100)
	proc.maxWorkers = maxWorkers

	// Statistical stuff
	proc.statsValues = make([]int, maxStatsCount)
	proc.statsTicker = time.NewTicker(time.Second)
	go proc.statsWorker()

	return proc
}

func (proc *WorkerPool) worker() {
	for obj := range proc.channel {
		proc.work(obj)
		proc.processed += 1 // not atomic
	}
	proc.wg.Done()
}

// Saves the delta of processed jobs per seconds
func (proc *WorkerPool) statsWorker() {
	var previous uint64
	var current uint64
	pos := 0
	for range proc.statsTicker.C {
		current = proc.processed
		proc.statsValues[pos] = int(current - previous)
		previous = current
		pos = (pos + 1) % maxStatsCount
	}
}

// Adds a new object to the channel
func (proc *WorkerPool) Add(obj interface{}) {
	if proc.currentWorkers == 0 || (len(proc.channel) > 0 && proc.maxWorkers > proc.currentWorkers) {
		// Start another worker
		proc.currentWorkers++ // not atomic, not crucial
		proc.wg.Add(1)
		go proc.worker()
	}
	proc.channel <- obj
}

// Calculates the number of jobs processed in the last minute
func (proc *WorkerPool) JobsPerMinute() int {

	var total int
	for i := 0; i < maxStatsCount; i++ {
		total += proc.statsValues[i]
	}

	return total
}

// Stops accepting new jobs and waits until all workers are finished
func (proc *WorkerPool) Close() {
	close(proc.channel)
	proc.statsTicker.Stop()
	proc.wg.Wait()
}
